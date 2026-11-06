/**
 * ----------------------------------------
 * LOGIC & STATE
 * ----------------------------------------
 */
const STORAGE_KEY = 'webscreen_device_configs';
const IGNORED_STORAGE_KEY = 'webscreen_ignored_devices';
let knownDevices = [];
let activeConfigSerial = null;
let ignoredDevices = loadIgnoredDevices();
let showIgnored = false;

function loadIgnoredDevices() {
    try {
        const stored = localStorage.getItem(IGNORED_STORAGE_KEY);
        return stored ? JSON.parse(stored) : [];
    } catch(e) { return []; }
}

function saveIgnoredDevices() {
    localStorage.setItem(IGNORED_STORAGE_KEY, JSON.stringify(ignoredDevices));
}

function toggleShowIgnored() {
    showIgnored = !showIgnored;
    const btn = document.getElementById('toggleIgnoredBtn');
    const icon = document.getElementById('toggleIgnoredIcon');
    if (btn) {
        if (showIgnored) {
            btn.classList.add('bg-[#333]', 'text-white');
            if (icon) icon.textContent = 'visibility';
        } else {
            btn.classList.remove('bg-[#333]', 'text-white');
            if (icon) icon.textContent = 'visibility_off';
        }
    }
    renderDeviceList();
}

function ignoreDevice(serial) {
    if (!ignoredDevices.includes(serial)) {
        ignoredDevices.push(serial);
        saveIgnoredDevices();
        renderDeviceList();
        showToast(i18n.t('device_ignored') || '设备已忽略');
    }
}

function unignoreDevice(serial) {
    ignoredDevices = ignoredDevices.filter(s => s !== serial);
    saveIgnoredDevices();
    renderDeviceList();
    showToast(i18n.t('device_unignored') || '设备已取消忽略');
}

// Refactored structure to match new requirements (all in driver_config)
const defaultConfig = {
    driver_config: {}
};

// --- Config Management ---

function loadDeviceConfigs() {
    try {
        const stored = localStorage.getItem(STORAGE_KEY);
        return stored ? JSON.parse(stored) : {};
    } catch (e) {
        console.error('Failed to load configs', e);
        return {};
    }
}

function saveDeviceConfigs(configs) {
    try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(configs));
    } catch (e) {
        console.error('Failed to save configs', e);
    }
}

let deviceConfigs = loadDeviceConfigs();

function ensureDeviceConfig(device) {
    const serial = typeof device === 'string' ? device : device.device_id;
    console.log('Ensuring config for device', serial);
    if (!deviceConfigs[serial]) {
        const type = device.device_type;
        console.log(`Creating default config for new device: ${serial} of type ${type}`);
        let baseConfig = JSON.parse(JSON.stringify(defaultConfig));
        deviceConfigs[serial] = {
            device_type: type,
            device_id: serial,
            device_ip: device.ip || '0',
            device_port: device.port || '0',
            av_sync: baseConfig.av_sync,
            driver_config: baseConfig.driver_config
        };
        saveDeviceConfigs(deviceConfigs);
    }
    return deviceConfigs[serial];
}

function pruneDeviceConfigs(activeDevices) {
    let changed = false;
    Object.keys(deviceConfigs).forEach(serial => {
        if (!activeDevices.includes(serial)) {
            delete deviceConfigs[serial];
            changed = true;
        }
    });
    if (changed) saveDeviceConfigs(deviceConfigs);
}

// --- Formatting Helpers ---

function formatBitrate(value) {
    if (!value) return '';
    if (value >= 1000000000) return `${(value / 1000000000).toFixed(1)}G`;
    if (value >= 1000000) return `${(value / 1000000).toFixed(0)}M`;
    if (value >= 1000) return `${(value / 1000).toFixed(0)}K`;
    return String(value);
}

function parseBitrate(str) {
    if (!str) return 8000000;
    const match = str.match(/^(\d+(?:\.\d+)?)\s*([KMG])?$/i);
    if (!match) return 8000000;
    let value = parseFloat(match[1]);
    const unit = (match[2] || '').toUpperCase();
    if (unit === 'K') value *= 1000;
    else if (unit === 'M') value *= 1000000;
    else if (unit === 'G') value *= 1000000000;
    return Math.round(value);
}

// --- UI Rendering ---

function renderDeviceList() {
    const grid = document.getElementById('deviceGrid');
    grid.innerHTML = '';

    const visibleDevices = knownDevices.filter(device => {
        const serial = typeof device === 'string' ? device : device.device_id;
        return showIgnored || !ignoredDevices.includes(serial);
    });

    const toggleBtn = document.getElementById('toggleIgnoredBtn');
    if (toggleBtn) {
        if (ignoredDevices.length > 0) {
            toggleBtn.classList.remove('!hidden');
            toggleBtn.style.display = '';
        } else {
            toggleBtn.style.display = 'none';
            if (showIgnored) {
                showIgnored = false;
                toggleBtn.classList.remove('bg-[#333]', 'text-white');
                const icon = document.getElementById('toggleIgnoredIcon');
                if (icon) icon.textContent = 'visibility_off';
            }
        }
    }

    if (!visibleDevices.length) {
        grid.innerHTML = `
                    <div class="col-span-full flex flex-col items-center justify-center py-20 text-gray-500 bg-[#1e1f20]/50 rounded-3xl border border-dashed border-gray-700">
                        <span class="material-symbols-rounded text-5xl mb-4 opacity-50">phonelink_off</span>
                        <p class="text-lg">${i18n.t('no_devices') || '没有设备'}</p>
                        <button onclick="openModal('connectModal')" class="mt-4 text-[var(--md-sys-color-primary)] hover:underline">${i18n.t('connect_device')}</button>
                    </div>
                `;
        return;
    }

    function buildDeviceCard(device) {
        const serial = typeof device === 'string' ? device : device.device_id;
        const config = ensureDeviceConfig(device);
        const drv = config.driver_config || {};
        const isIgnored = ignoredDevices.includes(serial);

        // Construct config tags
        let tagsHtml = '';
        let dtype = config.device_type === 'xvfb' ? 'linux' : config.device_type;
        const cacheKey = `${dtype}_${serial}`;
        
        const generateBadges = (schema, currentValues) => {
            if (!schema) return '';
            let html = '';
            let schemaArray = Array.isArray(schema) ? schema : [];
            for (const param of schemaArray) {
                if (param.badge) {
                    const key = param.name;
                    let currentValue = currentValues[key];
                    if (currentValue === undefined) currentValue = param.default;
                    
                    if (param.type === 'boolean') {
                        const isTrue = currentValue === true || currentValue === 'true';
                        let localizedLabel = i18n.t(key);
                        if (!localizedLabel || localizedLabel === key) localizedLabel = key;
                        if (isTrue) {
                            html += `<span class="px-2 py-0.5 rounded-md bg-[#333] text-xs text-gray-300 font-mono">${localizedLabel}</span>`;
                        } else {
                            html += `<span class="px-2 py-0.5 rounded-md bg-[#333] text-xs text-gray-300 font-mono line-through opacity-70">${localizedLabel}</span>`;
                        }
                    } else if (currentValue !== undefined && currentValue !== '') {
                        let displayVal = currentValue;
                        if (key === 'video_bit_rate') displayVal = formatBitrate(displayVal);
                        if (key === 'max_fps' || key === 'frameRate') displayVal += 'FPS';
                        if (key === 'video_codec') displayVal = String(displayVal).toUpperCase();
                        
                        html += `<span class="px-2 py-0.5 rounded-md bg-[#333] text-xs text-gray-300 font-mono">${displayVal}</span>`;
                    }
                }
            }
            return html;
        };

        tagsHtml += generateBadges(schemaCache['universal'], config);
        tagsHtml += generateBadges(schemaCache[cacheKey], drv);

        // 设备管理信息（USB / 无线、状态机、自定义名称）：由统一 ADB Manager 提供
        const info = typeof adbInfo === 'function' ? adbInfo(serial) : null;
        const adbState = (info && info.status) || device.state || '';
        const displayName = (info && (info.name || info.auto_name)) || device.name || serial;
        const subLine = (info && info.auto_name && info.auto_name !== displayName)
            ? info.auto_name
            : (device.model || '');
        const connText = (info && info.transport) || device.connection || '';
        const style = (typeof ADB_STATUS_STYLE === 'object' ? ADB_STATUS_STYLE[adbState] : null) || null;
        const statusBadge = adbState
            ? `<span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px] font-medium ${style ? style.chip : 'bg-[#2a2b2c] text-gray-300'}"><span class="w-1.5 h-1.5 rounded-full ${style ? style.dot : 'bg-gray-400'}"></span>${adbStatusText(adbState)}</span>`
            : '';
        const connChip = connText
            ? `<span class="px-2 py-0.5 rounded-full bg-[#2a2b2c] text-[11px] text-gray-300 font-mono">${connText}</span>`
            : '';
        const isUnauthorized = adbState === 'UNAUTHORIZED';
        const canStream = !adbState || adbState === 'ONLINE';
        const isWireless = String(connText).toUpperCase() === 'WIFI';
        // 已断开/离线的设备可以从列表里移除（内部按 adb serial 记录，移除即清掉它的名字）
        const canRemove = !!info && (adbState === 'DISCONNECTED' || adbState === 'OFFLINE');
        const adbActionsHtml = info ? `
                                <button onclick="renameDevice('${serial}')" class="p-2 rounded-full hover:bg-white/10 text-gray-400 transition-colors" title="${i18n.t('rename_device') || '重命名'}">
                                    <span class="material-symbols-rounded">edit</span>
                                </button>
                                ${isWireless ? `<button onclick="disconnectDevice('${serial}')" class="p-2 rounded-full hover:bg-white/10 text-gray-400 transition-colors" title="${i18n.t('disconnect') || '断开'}">
                                    <span class="material-symbols-rounded">link_off</span>
                                </button>` : ''}
                                <button onclick="reconnectDevice('${serial}')" class="p-2 rounded-full hover:bg-white/10 text-gray-400 transition-colors" title="${i18n.t('reconnect') || '重连'}">
                                    <span class="material-symbols-rounded">refresh</span>
                                </button>
                                ${canRemove ? `<button onclick="removeDevice('${serial}')" class="p-2 rounded-full hover:bg-white/10 text-red-300/80 transition-colors" title="${i18n.t('remove_device') || '从列表移除'}">
                                    <span class="material-symbols-rounded">delete</span>
                                </button>` : ''}` : '';
        const unauthorizedHint = isUnauthorized ? `
                        <div class="mt-3 flex items-start gap-2 text-xs text-amber-300 bg-amber-500/10 border border-amber-500/20 rounded-xl px-3 py-2">
                            <span class="material-symbols-rounded text-base">phonelink_lock</span>
                            <span>${i18n.t('usb_unauthorized_hint') || '设备等待 USB 调试授权：请查看手机/平板屏幕，点击“允许 USB 调试”。'}</span>
                        </div>` : '';
        const footerAction = canStream
            ? `<button onclick="startStream('${serial}')" class="w-full py-3 rounded-full bg-[#2a2b2c] group-hover:bg-[var(--md-sys-color-primary)] group-hover:text-[var(--md-sys-color-on-primary)] text-[var(--md-sys-color-primary)] font-medium transition-all flex items-center justify-center gap-2">
                        <span class="material-symbols-rounded">play_arrow</span>
                        ${i18n.t('start_stream')}
                    </button>`
            : `<button disabled class="w-full py-3 rounded-full bg-[#232425] text-gray-500 font-medium flex items-center justify-center gap-2 cursor-not-allowed">
                        <span class="material-symbols-rounded">${isUnauthorized ? 'phonelink_lock' : 'phonelink_off'}</span>
                        ${isUnauthorized ? (i18n.t('waiting_authorization') || '等待授权') : (i18n.t('device_unavailable') || '设备不可用')}
                    </button>`;

        const card = document.createElement('div');
        card.className = `card ${isIgnored ? 'opacity-40 grayscale' : ''} rounded-[24px] p-5 flex flex-col justify-between h-full border border-transparent hover:border-[#444] group transition-all`;

        let ignoreBtnHtml = isIgnored ? 
            `<button onclick="unignoreDevice('${serial}')" class="p-2 rounded-full hover:bg-white/10 text-orange-400 transition-colors" title="Unignore">
                <span class="material-symbols-rounded">visibility</span>
            </button>` :
            `<button onclick="ignoreDevice('${serial}')" class="p-2 rounded-full hover:bg-white/10 text-gray-400 transition-colors" title="Ignore">
                <span class="material-symbols-rounded">visibility_off</span>
            </button>`;

        card.innerHTML = `
                    <div>
                        <div class="flex justify-between items-start mb-4">
                            <div class="flex items-center gap-3">
                                <div class="w-10 h-10 rounded-full bg-[var(--md-sys-color-secondary-container)] flex items-center justify-center text-[var(--md-sys-color-on-secondary-container)]">
                                    <span class="material-symbols-rounded">smartphone</span>
                                </div>
                                <div>
                                    <h3 class="font-medium text-lg leading-tight text-[#e3e3e3] truncate max-w-[140px] md:max-w-[180px]" title="${serial}">${displayName}</h3>
                                    <div class="flex flex-wrap items-center gap-1.5 mt-1">
                                        ${connChip}
                                        ${statusBadge}
                                    </div>
                                    ${subLine ? `<div class="text-[11px] text-gray-500 mt-1 truncate max-w-[150px] md:max-w-[190px]" title="${serial}">${subLine}</div>` : ''}
                                </div>
                            </div>
                            <div class="flex items-center">
                                ${adbActionsHtml}
                                ${ignoreBtnHtml}
                                <button onclick="showConfigModal('${serial}')" class="p-2 rounded-full hover:bg-white/10 text-gray-400 transition-colors" title="Settings">
                                    <span class="material-symbols-rounded">settings</span>
                                </button>
                            </div>
                        </div>

                        <div class="flex flex-wrap gap-2 mb-6">
                            ${tagsHtml || `<span class="text-xs text-gray-500 italic">${i18n.t('default_config') || 'Default Config'}</span>`}
                        </div>
                    </div>

                    ${unauthorizedHint}
                    ${footerAction}
                `;
        grid.appendChild(card);
        return card;
    }

    // 按接入方式分组渲染：USB 设备 / 无线设备 / 其他（本机桌面），
    // 与需求第 19、22 节一致；同一台手机同时有 USB 与 Wi-Fi 时，
    // 后端会归并成一条记录（USB 优先），这里只按 transport 分类。
    const groups = [
        { key: 'usb', title: i18n.t('usb_devices') || 'USB 设备', icon: 'usb', devices: [] },
        { key: 'wifi', title: i18n.t('wireless_devices') || '无线设备', icon: 'wifi', devices: [] },
        { key: 'other', title: i18n.t('other_devices') || '其他设备', icon: 'devices_other', devices: [] },
    ];
    visibleDevices.forEach(device => {
        const serial = typeof device === 'string' ? device : device.device_id;
        const info = typeof adbInfo === 'function' ? adbInfo(serial) : null;
        const type = (device.device_type || '').toLowerCase();
        if (type === 'android') {
            const transport = (info && info.transport) || device.connection || '';
            if (String(transport).toUpperCase() === 'USB') groups[0].devices.push(device);
            else groups[1].devices.push(device);
        } else {
            groups[2].devices.push(device);
        }
    });

    groups.forEach(group => {
        // 无线分组即使为空也显示，让用户知道这类设备的存在与接入方式
        const alwaysShow = group.key === 'wifi';
        if (!group.devices.length && !alwaysShow) return;
        const header = document.createElement('div');
        header.className = 'col-span-full flex items-center justify-between mt-2 mb-1 first:mt-0';
        header.innerHTML = `
            <div class="flex items-center gap-2 text-gray-400">
                <span class="material-symbols-rounded text-xl">${group.icon}</span>
                <span class="text-sm font-medium tracking-wide">${group.title}</span>
                <span class="text-xs px-2 py-0.5 rounded-full bg-[#2a2b2c]">${group.devices.length}</span>
            </div>`;
        grid.appendChild(header);
        if (!group.devices.length) {
            const empty = document.createElement('div');
            empty.className = 'col-span-full text-xs text-gray-500 mb-3 px-1';
            empty.textContent = i18n.t('no_wireless_devices') || '暂无无线设备：用上方「无线配对」配对，或用「连接设备」填写 ip:端口 接入。';
            grid.appendChild(empty);
            return;
        }
        group.devices.forEach(device => {
            const card = buildDeviceCard(device);
            if (card) grid.appendChild(card);
        });
    });
}

// --- Actions ---

async function fetchDevices() {
    const grid = document.getElementById('deviceGrid');
    // Show loading
    grid.innerHTML = `
                <div class="col-span-full flex flex-col items-center justify-center py-20 text-gray-500">
                    <div class="spinner mb-4"></div>
                    <p>${i18n.t('scanning_devices')}</p>
                </div>
            `;

    try {
        // Try real API first
        const response = await fetch('/api/device/list');
        if (!response.ok) throw new Error('API Error');
        const data = await response.json();
        const devices = Array.isArray(data.devices) ? data.devices : [];
        console.log('Fetched devices:', devices);
        knownDevices = devices;

        // Fetch schemas for all devices to render badges
        const schemaPromises = [];
        if (!schemaCache['universal']) {
            schemaPromises.push(
                fetch('/api/device/configDescription?device_type=universal')
                .then(r => r.json())
                .then(s => schemaCache['universal'] = s)
                .catch(e => console.error(e))
            );
        }
        for (const device of devices) {
            let activeConfigSerial = typeof device === 'string' ? device : device.device_id;
            let dtype = device.device_type;
            if (dtype === 'xvfb') dtype = 'linux';
            const cacheKey = `${dtype}_${activeConfigSerial}`;
            if (!schemaCache[cacheKey]) {
                schemaPromises.push(
                    fetch(`/api/device/configDescription?device_type=${dtype}&device_id=${encodeURIComponent(activeConfigSerial)}`)
                    .then(r => r.json())
                    .then(s => schemaCache[cacheKey] = s)
                    .catch(e => console.error(e))
                );
            }
        }
        await Promise.all(schemaPromises);

        const serials = devices.map(d => d.device_id);
        pruneDeviceConfigs(serials);
        devices.forEach(d => ensureDeviceConfig(d));

        renderDeviceList();
        showToast(i18n.t('refreshed_found', {n: devices.length}));

    } catch (error) {
        console.warn('Using mock data because fetch failed:', error);

        // Fallback to Mock Data for UI Preview
        setTimeout(() => {
            knownDevices = MOCK_DEVICES;
            knownDevices.forEach(d => ensureDeviceConfig(d));
            renderDeviceList();
            showToast(i18n.t('call_api_failed'), 'info');
        }, 800);
    }
}

async function connectDevice() {
    const ip = document.getElementById('connectIP').value;
    const port = document.getElementById('connectPort').value;

    if (!ip) {
        showToast(i18n.t('enter_ip'), 'error');
        return;
    }

    try {
        const response = await fetch('/api/device/connect', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ device_type: 'android', ip, port })
        });

        if (response.ok) {
            showToast(i18n.t('connected_success'));
            closeModal('connectModal');
            fetchDevices();
        } else {
            const data = await response.json();
            throw new Error(data.error || i18n.t('connection_failed'));
        }
    } catch (error) {
        console.error(error);
        showToast(i18n.t('call_api_failed'), 'error');
    }
}

async function pairDevice() {
    const ip = document.getElementById('pairIP').value;
    const port = document.getElementById('pairPort').value;
    const code = document.getElementById('pairCode').value;

    if (!ip || !port || !code) {
        showToast(i18n.t('fill_all_fields'), 'error');
        return;
    }

    try {
        const response = await fetch('/api/device/pair', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ device_type: 'android', ip, port, code })
        });

        if (response.ok) {
            showToast(i18n.t('pair_success'));
            closeModal('pairModal');
            document.getElementById('connectIP').value = ip;
            openModal('connectModal');
        } else {
            const data = await response.json();
            throw new Error(data.error);
        }
    } catch (error) {
        showToast(i18n.t('pair_failed'), 'error');
    }
}

let currentConfigSchema = {};
let currentUniversalSchema = {};
let schemaCache = {}; // { 'universal': [...], 'type_id': [...] }

function startStream(serial) {
    const device = knownDevices.find(d => d.device_id === serial);
    if (!device) return;

    const config = ensureDeviceConfig(device);
    const drv = { ...(config.driver_config || {}) };

    let finalConfig = {
        device_type: config.device_type,
        device_id: config.device_id || serial,
        device_ip: config.device_ip || '0',
        device_port: config.device_port || '0',
        av_sync: config.av_sync || false,
        use_local_timestamp: config.use_local_timestamp || false,
        driver_config: drv
    };
    
    if (config.device_type == 'android') {
    finalConfig.driver_config.deviceID = config.device_ip || '';
    }
    console.log('Starting stream with config:', finalConfig);
    sessionStorage.setItem('webscreen_device_configs_now', JSON.stringify(finalConfig));
    showToast(i18n.t('starting_stream'));

    const id = `${finalConfig.device_type}_${finalConfig.device_id}_${finalConfig.device_ip}_${finalConfig.device_port}`;
    
    setTimeout(() => {
        window.location.href = `/screen/${id}`;
    }, 500);
}

// --- Modal Logic ---

function openModal(id) {
    const dialog = document.getElementById(id);
    if (dialog) {
        dialog.showModal();
        // 「连接设备」里带一个 USB 设备区块：打开时刷新一次并显示当前 USB 设备
        if (id === 'connectModal' && typeof ensureUSBBlockInConnectModal === 'function') {
            ensureUSBBlockInConnectModal();
            loadUSBDevicesInModal();
        }
    }
}

function closeModal(id) {
    const dialog = document.getElementById(id);
    if (dialog) {
        dialog.close();
    }
    if (id === 'configModal') {
        activeConfigSerial = null;
        currentConfigSchema = {};
        currentUniversalSchema = {};
    }
}

async function showConfigModal(serial) {
    activeConfigSerial = serial;
    const device = knownDevices.find(d => d.device_id === serial) || { device_id: serial };
    const config = ensureDeviceConfig(device);

    document.getElementById('configModalTitle').textContent = i18n.t('config_device_title', {serial: serial});
    
    const dynamicContainer = document.getElementById('dynamicSettings');
    const universalContainer = document.getElementById('universalSettings');
    dynamicContainer.innerHTML = `<div class="flex justify-center py-8"><div class="spinner"></div></div>`;
    if (universalContainer) universalContainer.innerHTML = `<div class="flex justify-center py-8"><div class="spinner"></div></div>`;
    openModal('configModal');

    // For compatibility with older versions, treat 'xvfb' as 'linux' for config schema purposes
    if (config.device_type === 'xvfb') {
        config.device_type = 'linux';
    }
    
    try {
        const [resDyn, resUni] = await Promise.all([
            fetch(`/api/device/configDescription?device_type=${config.device_type}&device_id=${encodeURIComponent(device.device_id)}`),
            fetch(`/api/device/configDescription?device_type=universal`)
        ]);
        
        if (!resDyn.ok) throw new Error('Failed to fetch dynamic config description');
        const schemaDyn = await resDyn.json();
        currentConfigSchema = schemaDyn;
        renderDynamicConfigForm(schemaDyn, config.driver_config || {}, 'dynamicSettings', 'dyn_');

        if (resUni.ok) {
            const schemaUni = await resUni.json();
            currentUniversalSchema = schemaUni;
            // The top-level configs are properties directly on the config item
            renderDynamicConfigForm(schemaUni, config, 'universalSettings', 'uni_');
        }
    } catch (e) {
        console.error(e);
        dynamicContainer.innerHTML = `<div class="text-red-400 text-sm text-center py-4">Failed to load configuration schema.</div>`;
        if (universalContainer) universalContainer.innerHTML = '';
    }
}

function renderDynamicConfigForm(schema, currentValues, containerId = 'dynamicSettings', prefix = 'dyn_') {
    const container = document.getElementById(containerId);
    if (!container) return;
    container.innerHTML = '';

    const panel = document.createElement('div');
    panel.className = 'bg-[#2a2b2c] p-4 rounded-2xl space-y-4';

    let schemaArray = Array.isArray(schema) ? schema : [];
    if (!Array.isArray(schema)) {
        // Fallback for older backend format if not updated
        schemaArray = Object.entries(schema).map(([k, v]) => ({ name: k, ...v }));
    }

    for (const param of schemaArray) {
        const key = param.name;
        const fieldDiv = document.createElement('div');
        fieldDiv.className = 'flex flex-col gap-1';

        const label = document.createElement('label');
        label.className = 'block text-xs font-medium text-gray-400 ml-1';
        let localizedLabel = i18n.t(key);
        if (!localizedLabel || localizedLabel === key) {
            localizedLabel = key;
        }
        label.textContent = localizedLabel + (param.required ? ' *' : '');
        label.title = param.description || '';

        let input;
        const currentValue = currentValues[key] !== undefined ? currentValues[key] : param.default;

        if (param.type === 'boolean') {
            const wrap = document.createElement('div');
            wrap.className = 'flex items-center justify-between py-1';
            
            const checkLabel = document.createElement('label');
            checkLabel.className = 'text-sm font-medium text-gray-300 cursor-pointer select-none ml-1';
            checkLabel.textContent = localizedLabel;
            checkLabel.title = param.description || '';

            input = document.createElement('input');
            input.type = 'checkbox';
            input.className = 'md-switch';
            input.id = `${prefix}${key}`;
            checkLabel.setAttribute('for', input.id);
            
            // Allow string 'true' or boolean true
            input.checked = currentValue === true || currentValue === 'true';

            wrap.appendChild(checkLabel);
            wrap.appendChild(input);
            fieldDiv.appendChild(wrap);
            
            if (param.description) {
                const desc = document.createElement('p');
                desc.className = 'text-[10px] text-gray-500 ml-1';
                desc.textContent = param.description;
                fieldDiv.appendChild(desc);
            }
        } else if (param.options && param.options.length > 0) {
            const wrap = document.createElement('div');
            input = document.createElement('select');
            input.className = 'md-input w-full px-3 py-2 rounded-lg text-white text-sm appearance-none bg-[url("data:image/svg+xml;base64,PHN2ZyBmaWxsPSIjZmZmIiBoZWlnaHQ9IjI0IiB2aWV3Qm94PSIwIDAgMjQgMjQiIHdpZHRoPSIyNCIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj48cGF0aCBkPSJNNyAxMGw1IDUgNS01eiIvPjwvc3ZnPg==")] bg-no-repeat bg-right';
            input.id = `${prefix}${key}`;

            if (!param.required) {
                const opt = document.createElement('option');
                opt.value = '';
                opt.textContent = '-- Select / Auto --';
                input.appendChild(opt);
            }

            param.options.forEach(o => {
                const opt = document.createElement('option');
                opt.value = o;
                opt.textContent = o;
                if (currentValue === o || currentValue === String(o)) opt.selected = true;
                input.appendChild(opt);
            });

            fieldDiv.appendChild(label);
            fieldDiv.appendChild(input);
            
            if (param.description) {
                const desc = document.createElement('p');
                desc.className = 'text-[10px] text-gray-500 ml-1';
                desc.textContent = param.description;
                fieldDiv.appendChild(desc);
            }
        } else {
            input = document.createElement('input');
            input.type = param.type === 'integer' ? 'number' : 'text';
            input.className = 'md-input w-full px-3 py-2 rounded-lg text-white text-sm';
            input.id = `${prefix}${key}`;
            input.value = currentValue !== undefined ? currentValue : '';
            if (param.default) {
                input.placeholder = `${param.default}`;
            }

            fieldDiv.appendChild(label);
            fieldDiv.appendChild(input);
            
            if (param.description) {
                const desc = document.createElement('p');
                desc.className = 'text-[10px] text-gray-500 ml-1 mt-1';
                desc.textContent = param.description;
                fieldDiv.appendChild(desc);
            }
        }

        panel.appendChild(fieldDiv);
    }
    container.appendChild(panel);
}

function saveDeviceConfig() {
    if (!activeConfigSerial) return;

    const device = knownDevices.find(d => d.device_id === activeConfigSerial);
    const config = ensureDeviceConfig(device);

    if (!config.driver_config) config.driver_config = {};
    const drv = config.driver_config;

    let dynArray = Array.isArray(currentConfigSchema) ? currentConfigSchema : [];
    if (!Array.isArray(currentConfigSchema)) {
        dynArray = Object.entries(currentConfigSchema).map(([k, v]) => ({ name: k, ...v }));
    }

    for (const param of dynArray) {
        const key = param.name;
        const input = document.getElementById(`dyn_${key}`);
        if (!input) continue;

        if (param.type === 'boolean') {
            drv[key] = input.checked ? 'true' : 'false'; 
        } else {
            const val = input.value.trim();
            if (val) {
                drv[key] = val;
            } else {
                delete drv[key];
            }
        }
    }

    let uniArray = Array.isArray(currentUniversalSchema) ? currentUniversalSchema : [];
    if (!Array.isArray(currentUniversalSchema)) {
        uniArray = Object.entries(currentUniversalSchema).map(([k, v]) => ({ name: k, ...v }));
    }

    for (const param of uniArray) {
        const key = param.name;
        const input = document.getElementById(`uni_${key}`);
        if (!input) continue;

        if (param.type === 'boolean') {
            config[key] = input.checked;
        } else {
            const val = input.value.trim();
            if (val) {
                config[key] = val;
            } else {
                delete config[key];
            }
        }
    }

    saveDeviceConfigs(deviceConfigs);
    renderDeviceList();
    closeModal('configModal');
    showToast(i18n.t('config_saved'));
}

// --- Toast Logic ---

function showToast(message, type = 'success') {
    const container = document.getElementById('toast-container');
    const toast = document.createElement('div');
    toast.className = `toast ${type === 'error' ? 'error' : ''}`;
    toast.innerHTML = `
                <span>${message}</span>
                ${type === 'error' ? '<span class="material-symbols-rounded text-sm">error</span>' : '<span class="material-symbols-rounded text-sm">check_circle</span>'}
            `;
    container.appendChild(toast);

    // Remove after 3 seconds
    setTimeout(() => {
        toast.style.animation = 'toastOut 0.3s forwards';
        setTimeout(() => toast.remove(), 300);
    }, 3000);
}

// ---------------------------------------------------------------------------
// 设备管理（USB / 无线）：状态徽章、重命名、断开/重连、添加无线设备、事件日志
// 数据来自统一 ADB Manager：/api/adb/devices 与 /api/adb/events
// ---------------------------------------------------------------------------

const ADB_STATUS_STYLE = {
    ONLINE:       { chip: 'bg-emerald-500/15 text-emerald-300', dot: 'bg-emerald-400' },
    UNAUTHORIZED: { chip: 'bg-amber-500/15 text-amber-300',     dot: 'bg-amber-400' },
    CONNECTING:   { chip: 'bg-sky-500/15 text-sky-300',         dot: 'bg-sky-400' },
    OFFLINE:      { chip: 'bg-gray-500/15 text-gray-300',       dot: 'bg-gray-400' },
    DISCONNECTED: { chip: 'bg-gray-500/15 text-gray-400',       dot: 'bg-gray-500' },
    ERROR:        { chip: 'bg-red-500/15 text-red-300',         dot: 'bg-red-400' },
};

const ADB_STATUS_TEXT = {
    ONLINE:       { zh: '在线', en: 'Online' },
    UNAUTHORIZED: { zh: '等待授权', en: 'Unauthorized' },
    CONNECTING:   { zh: '连接中', en: 'Connecting' },
    OFFLINE:      { zh: '离线', en: 'Offline' },
    DISCONNECTED: { zh: '已断开', en: 'Disconnected' },
    ERROR:        { zh: '错误', en: 'Error' },
};

function adbStatusText(state) {
    const label = ADB_STATUS_TEXT[state];
    if (!label) return state;
    const lang = (window.i18n && i18n.lang) ? i18n.lang : 'zh';
    return lang === 'en' ? label.en : label.zh;
}

let adbDeviceMap = {};

function adbInfo(serial) {
    return adbDeviceMap[serial] || null;
}

async function loadADBDevices() {
    try {
        const res = await fetch('/api/adb/devices');
        if (!res.ok) return false;
        const data = await res.json();
        adbDeviceMap = {};
        (data.devices || []).forEach(d => { adbDeviceMap[d.serial] = d; });
        return true;
    } catch (e) {
        return false;
    }
}

async function loadADBEvents() {
    try {
        const res = await fetch('/api/adb/events?limit=30');
        if (!res.ok) return;
        const data = await res.json();
        renderADBEvents(data.events || []);
    } catch (e) { /* 忽略：老版本没有这个接口 */ }
}

function renderADBEvents(events) {
    const box = document.getElementById('adbEventList');
    if (!box) return;
    if (!events.length) {
        box.innerHTML = `<div class="text-xs text-gray-500 px-1 py-2">${i18n.t('no_events') || '暂无设备事件'}</div>`;
        return;
    }
    const color = { info: 'text-gray-300', warn: 'text-amber-300', error: 'text-red-300' };
    box.innerHTML = events.map(ev => `
        <div class="flex items-start gap-3 px-1 py-1.5 border-b border-[#262728] last:border-0">
            <span class="text-[11px] text-gray-500 font-mono whitespace-nowrap mt-0.5">${ev.time}</span>
            <span class="text-xs ${color[ev.level] || 'text-gray-300'}">${ev.message}</span>
        </div>`).join('');
}

// 简易输入弹窗（fnOS 面板是 iframe，window.prompt 可能被沙箱禁用）
function dmPrompt(title, initial, placeholder) {
    return new Promise(resolve => {
        const overlay = document.createElement('div');
        overlay.style.cssText = 'position:fixed;inset:0;background:rgba(0,0,0,.55);display:flex;align-items:center;justify-content:center;z-index:9999';
        overlay.innerHTML = `
            <div style="width:min(420px,92vw);background:#1e1f20;border:1px solid #333;border-radius:24px;padding:22px">
                <div style="color:#e3e3e3;font-weight:500;margin-bottom:14px">${title}</div>
                <input id="dmPromptInput" value="${initial || ''}" placeholder="${placeholder || ''}"
                       style="width:100%;background:#2a2b2c;border:1px solid #3a3b3c;border-radius:12px;padding:11px 13px;color:#e3e3e3;outline:none;font-family:inherit">
                <div style="display:flex;justify-content:flex-end;gap:10px;margin-top:18px">
                    <button id="dmPromptCancel" style="padding:9px 18px;border-radius:999px;background:transparent;border:1px solid #3a3b3c;color:#bbb">${i18n.t('cancel') || '取消'}</button>
                    <button id="dmPromptOk" style="padding:9px 18px;border-radius:999px;background:var(--md-sys-color-primary);color:var(--md-sys-color-on-primary);border:0;font-weight:500">${i18n.t('confirm') || '确定'}</button>
                </div>
            </div>`;
        document.body.appendChild(overlay);
        const input = overlay.querySelector('#dmPromptInput');
        input.focus();
        if (initial) input.select();
        const done = value => { overlay.remove(); resolve(value); };
        overlay.querySelector('#dmPromptOk').onclick = () => done(input.value.trim());
        overlay.querySelector('#dmPromptCancel').onclick = () => done(null);
        overlay.addEventListener('keydown', e => {
            if (e.key === 'Enter') done(input.value.trim());
            if (e.key === 'Escape') done(null);
        });
    });
}

// 简易确认弹窗（与 dmPrompt 同风格，fnOS 面板是 iframe，window.confirm 可能被沙箱禁用）
function dmConfirm(title, text) {
    return new Promise(resolve => {
        const overlay = document.createElement('div');
        overlay.style.cssText = 'position:fixed;inset:0;background:rgba(0,0,0,.55);display:flex;align-items:center;justify-content:center;z-index:9999';
        overlay.innerHTML = `
            <div style="width:min(420px,92vw);background:#1e1f20;border:1px solid #333;border-radius:24px;padding:22px">
                <div style="color:#e3e3e3;font-weight:500;margin-bottom:10px">${title}</div>
                <div style="color:#9aa0a6;font-size:13px;line-height:1.6">${text}</div>
                <div style="display:flex;justify-content:flex-end;gap:10px;margin-top:18px">
                    <button id="dmConfirmCancel" style="padding:9px 18px;border-radius:999px;background:transparent;border:1px solid #3a3b3c;color:#bbb">${i18n.t('cancel') || '取消'}</button>
                    <button id="dmConfirmOk" style="padding:9px 18px;border-radius:999px;background:var(--md-sys-color-primary);color:var(--md-sys-color-on-primary);border:0;font-weight:500">${i18n.t('confirm') || '确定'}</button>
                </div>
            </div>`;
        document.body.appendChild(overlay);
        const done = v => { overlay.remove(); resolve(v); };
        overlay.querySelector('#dmConfirmOk').onclick = () => done(true);
        overlay.querySelector('#dmConfirmCancel').onclick = () => done(false);
        overlay.addEventListener('keydown', e => {
            if (e.key === 'Escape') done(false);
            if (e.key === 'Enter') done(true);
        });
    });
}

async function adbPost(path, body) {
    const res = await fetch(path, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
    });
    let data = {};
    try { data = await res.json(); } catch (e) { /* 空响应 */ }
    if (!res.ok) throw new Error(data.error || ('HTTP ' + res.status));
    return data;
}

async function renameDevice(serial) {
    const info = adbInfo(serial) || {};
    const name = await dmPrompt(i18n.t('rename_device') || '设备名称', info.name || info.auto_name || '', info.auto_name || '');
    if (name === null) return;
    try {
        await adbPost('/api/adb/device/name', { serial, name });
        showToast(i18n.t('device_renamed') || '设备名称已更新');
    } catch (e) {
        showToast(e.message, 'error');
    }
    await refreshDevices();
}

async function disconnectDevice(serial) {
    try {
        await adbPost('/api/adb/device/disconnect', { serial });
        showToast(i18n.t('device_disconnected') || '已断开');
    } catch (e) {
        showToast(e.message, 'error');
    }
    await refreshDevices();
}

async function reconnectDevice(serial) {
    try {
        await adbPost('/api/adb/device/reconnect', { serial });
        showToast(i18n.t('reconnecting') || '正在重连…');
    } catch (e) {
        showToast(e.message, 'error');
    }
    await refreshDevices();
}

async function addWirelessDevice() {
    const addr = await dmPrompt(i18n.t('add_wireless_device') || '添加无线设备', '', '192.168.31.120:5555');
    if (!addr) return;
    showToast(i18n.t('connecting') || '正在连接…');
    try {
        await adbPost('/api/adb/device/wifi', { address: addr });
        showToast(i18n.t('wireless_added') || '无线设备已连接');
    } catch (e) {
        showToast(e.message, 'error');
    }
    await refreshDevices();
}

// 「连接设备」弹窗里的 USB 设备区块：列出已接入设备，并可一键刷新 +
// 重新请求调试授权（未授权设备会重新在手机上弹出授权对话框）。
function ensureUSBBlockInConnectModal() {
    const dialog = document.getElementById('connectModal');
    if (!dialog || dialog.querySelector('#connectUSBSection')) return;
    const panel = dialog.querySelector('.dialog-panel');
    if (!panel) return;
    const box = document.createElement('div');
    box.id = 'connectUSBSection';
    box.className = 'mt-6 pt-5 border-t border-[#333]';
    box.innerHTML = `
        <div class="flex items-center justify-between mb-2">
            <div class="flex items-center gap-2 text-gray-300">
                <span class="material-symbols-rounded text-lg">usb</span>
                <span class="text-sm font-medium">${i18n.t('usb_devices')}</span>
            </div>
            <button id="usbRescanBtn" onclick="rescanUSBDevices()" class="flex items-center gap-1 px-3 py-1.5 rounded-full bg-[#2a2b2c] hover:bg-[#333] text-xs text-[var(--md-sys-color-primary)] transition-colors">
                <span class="material-symbols-rounded text-base">refresh</span>${i18n.t('refresh_usb_auth') || '刷新并请求授权'}
            </button>
        </div>
        <div id="usbDeviceList" class="text-xs text-gray-400 space-y-1 max-h-40 overflow-y-auto"></div>`;
    const fields = panel.querySelector('.space-y-4');
    if (fields) fields.insertAdjacentElement('afterend', box);
    else panel.appendChild(box);
}

function renderUSBDeviceList(devices) {
    const box = document.getElementById('usbDeviceList');
    if (!box) return;
    const usb = (devices || []).filter(d => String(d.transport || '').toUpperCase() === 'USB');
    if (!usb.length) {
        box.innerHTML = `<div class="py-1">${i18n.t('no_usb_devices') || '未检测到 USB 设备：请用数据线连接手机，并在手机上开启 USB 调试。'}</div>`;
        return;
    }
    box.innerHTML = usb.map(d => {
        const style = ADB_STATUS_STYLE[d.status] || { chip: 'bg-[#2a2b2c] text-gray-300', dot: 'bg-gray-400' };
        return `<div class="flex items-center justify-between gap-2 py-1">
            <span class="truncate text-gray-300">${d.name || d.auto_name || d.serial}</span>
            <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px] ${style.chip}"><span class="w-1.5 h-1.5 rounded-full ${style.dot}"></span>${adbStatusText(d.status)}</span>
        </div>`;
    }).join('');
}

async function loadUSBDevicesInModal() {
    await loadADBDevices();
    renderUSBDeviceList(Object.values(adbDeviceMap));
}

async function rescanUSBDevices() {
    const btn = document.getElementById('usbRescanBtn');
    if (btn) btn.disabled = true;
    showToast(i18n.t('rescanning_usb') || '正在刷新 USB 设备并请求授权…');
    try {
        const res = await fetch('/api/adb/usb/rescan', { method: 'POST' });
        const data = await res.json().catch(() => ({}));
        if (Array.isArray(data.devices)) {
            adbDeviceMap = {};
            data.devices.forEach(d => { adbDeviceMap[d.serial] = d; });
            renderUSBDeviceList(data.devices);
            renderDeviceList();
        }
        if (!res.ok) throw new Error(data.error || ('HTTP ' + res.status));
        showToast(i18n.t('rescan_done') || '已重新请求授权，请在手机上确认');
    } catch (e) {
        showToast(e.message, 'error');
    } finally {
        if (btn) btn.disabled = false;
        loadADBEvents();
    }
}

async function removeDevice(serial) {
    const info = adbInfo(serial) || {};
    const name = info.name || info.auto_name || serial;
    const ok = await dmConfirm(
        i18n.t('remove_device') || '从列表移除',
        (i18n.t('confirm_remove_device') || '从设备列表移除「{name}」？设备重新接入时会再次出现（自定义名称会被清除）。').replace('{name}', name)
    );
    if (!ok) return;
    try {
        await adbPost('/api/adb/device/remove', { serial });
        showToast(i18n.t('device_removed') || '已从列表移除');
    } catch (e) {
        showToast(e.message, 'error');
    }
    await refreshDevices();
}

// 设备列表 + 事件日志一起刷新（页面不可见时不打扰）
async function refreshDevices() {
    await loadADBDevices();
    const grid = document.getElementById('deviceGrid');
    if (grid) renderDeviceList();
    loadADBEvents();
}

function ensureADBPanels() {
    const grid = document.getElementById('deviceGrid');
    if (!grid || document.getElementById('adbEventPanel')) return;
    const panel = document.createElement('section');
    panel.id = 'adbEventPanel';
    panel.className = 'mt-8 rounded-[24px] bg-[#1e1f20] border border-[#262728] p-5';
    panel.innerHTML = `
        <div class="flex items-center gap-2 text-gray-300 mb-3">
            <span class="material-symbols-rounded text-xl">receipt_long</span>
            <span class="text-sm font-medium">${i18n.t('device_events') || '设备事件'}</span>
        </div>
        <div id="adbEventList" class="max-h-56 overflow-y-auto"></div>`;
    grid.insertAdjacentElement('afterend', panel);
}

async function initDeviceManager() {
    const available = await loadADBDevices();
    if (!available) return; // 本机 root 模式或旧版本：没有设备管理接口
    ensureADBPanels();
    renderDeviceList();
    loadADBEvents();
    setInterval(() => {
        if (document.visibilityState === 'visible') refreshDevices();
    }, 3000);
}

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    fetchDevices();
    initDeviceManager();
});
