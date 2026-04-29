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
                        <button onclick="openModal('connectModal')" class="mt-4 text-[var(--md-sys-color-primary)] hover:underline">${i18n.t('connect_device') || '连接设备'}</button>
                    </div>
                `;
        return;
    }

    visibleDevices.forEach(device => {
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
                                    <h3 class="font-medium text-lg leading-tight text-[#e3e3e3] truncate max-w-[140px] md:max-w-[180px]" title="${serial}">${serial}</h3>
                                </div>
                            </div>
                            <div class="flex items-center">
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

                    <button onclick="startStream('${serial}')" class="w-full py-3 rounded-full bg-[#2a2b2c] group-hover:bg-[var(--md-sys-color-primary)] group-hover:text-[var(--md-sys-color-on-primary)] text-[var(--md-sys-color-primary)] font-medium transition-all flex items-center justify-center gap-2">
                        <span class="material-symbols-rounded">play_arrow</span>
                        ${i18n.t('start_stream')}
                    </button>
                `;
        grid.appendChild(card);
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
        // dialog.addEventListener('click', (e) => {
        //     const rect = dialog.getBoundingClientRect();
        //     if (e.clientX < rect.left || e.clientX > rect.right || e.clientY < rect.top || e.clientY > rect.bottom) {
        //         closeModal(id);
        //     }
        // });
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

// Initialize
document.addEventListener('DOMContentLoaded', fetchDevices);
