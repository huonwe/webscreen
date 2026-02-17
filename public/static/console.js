/**
 * ----------------------------------------
 * LOGIC & STATE
 * ----------------------------------------
 */
const STORAGE_KEY = 'webscreen_device_configs';
let knownDevices = [];
let activeConfigSerial = null;

// Refactored structure to match new requirements (all in driver_config)
const defaultScrcpyConfig = {
    device_type: 'android',
    av_sync: false,
    driver_config: {
        max_fps: '',
        max_size: '',
        video_codec: 'h264',
        audio: 'true',
        audio_codec: 'opus',
        video_bit_rate: 8000000,
        video_codec_options: '',
        new_display: '',
        deviceID: '', // For scrcpy driver to identify which device to connect to
    }
};

const defaultXvfbConfig = {
    device_type: "xvfb",
    device_id: "localhost",
    device_ip: "127.0.0.1",
    device_port: "0",
    av_sync: false,
    driver_config: {
        ip : "",
        user: "",
        resolution: "1920x1080",
        frameRate: "60",
        bitRate: "8000000",
        video_codec: "h264",
    }
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
        // console.log('Creating default config for', serial, 'of type', type);
        let baseConfig;
        if (type === 'xvfb') {
             baseConfig = JSON.parse(JSON.stringify(defaultXvfbConfig));
        } else {
             baseConfig = JSON.parse(JSON.stringify(defaultScrcpyConfig));
        }

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

    if (!knownDevices.length) {
        grid.innerHTML = `
                    <div class="col-span-full flex flex-col items-center justify-center py-20 text-gray-500 bg-[#1e1f20]/50 rounded-3xl border border-dashed border-gray-700">
                        <span class="material-symbols-rounded text-5xl mb-4 opacity-50">phonelink_off</span>
                        <p class="text-lg">${i18n.t('no_devices')}</p>
                        <button onclick="openModal('connectModal')" class="mt-4 text-[var(--md-sys-color-primary)] hover:underline">${i18n.t('connect_device')}</button>
                    </div>
                `;
        return;
    }

    knownDevices.forEach(device => {
        const serial = typeof device === 'string' ? device : device.device_id;
        const config = ensureDeviceConfig(device);
        // Access nested driver_config now
        const drv = config.driver_config || {};

        // Construct config tags
        let tagsHtml = '';
        if (config.device_type === 'xvfb') {
             if (drv.resolution) tagsHtml += `<span class="px-2 py-0.5 rounded-md bg-[#333] text-xs text-gray-300 font-mono">${drv.resolution}</span>`;
             if (drv.frameRate) tagsHtml += `<span class="px-2 py-0.5 rounded-md bg-[#333] text-xs text-gray-300 font-mono">${drv.frameRate}FPS</span>`;
        } else {
            if (drv.max_fps) tagsHtml += `<span class="px-2 py-0.5 rounded-md bg-[#333] text-xs text-gray-300 font-mono">${drv.max_fps}FPS</span>`;
            if (drv.video_bit_rate) tagsHtml += `<span class="px-2 py-0.5 rounded-md bg-[#333] text-xs text-gray-300 font-mono">${formatBitrate(drv.video_bit_rate)}</span>`;
            if (drv.video_codec) tagsHtml += `<span class="px-2 py-0.5 rounded-md bg-[#333] text-xs text-gray-300 font-mono uppercase">${drv.video_codec}</span>`;
            if (drv.audio === 'true') {
                tagsHtml += `<span class="px-2 py-0.5 rounded-md bg-[#333] text-xs text-gray-300 font-mono">Audio</span>`;
            } else {
                tagsHtml += `<span class="px-2 py-0.5 rounded-md bg-[#333] text-xs text-gray-300 font-mono line-through">Audio</span>`;
            }
        }

        const card = document.createElement('div');
        card.className = 'card rounded-[24px] p-5 flex flex-col justify-between h-full border border-transparent hover:border-[#444] group';

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
                            <button onclick="showConfigModal('${serial}')" class="p-2 rounded-full hover:bg-white/10 text-gray-400 transition-colors" title="Settings">
                                <span class="material-symbols-rounded">settings</span>
                            </button>
                        </div>

                        <div class="flex flex-wrap gap-2 mb-6">
                            ${tagsHtml || `<span class="text-xs text-gray-500 italic">${i18n.t('default_config')}</span>`}
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

function startStream(serial) {
    const device = knownDevices.find(d => d.device_id === serial);
    if (!device) return;

    const config = ensureDeviceConfig(device);
    const drv = config.driver_config || {};

    let finalConfig;

    if (config.device_type === 'xvfb') {
        finalConfig = {
            device_type: "xvfb",
            device_id: config.device_id || serial,
            device_ip: config.device_ip || "127.0.0.1",
            device_port: config.device_port || "0",
            av_sync: document.getElementById('configAVSync').checked,
            driver_config: {
                ip : drv.ip || "",
                user: drv.user || "",
                resolution: drv.resolution || "1920x1080",
                frameRate: String(drv.frameRate || "60"),
                bitRate: String(drv.bitRate || "20000000"),
                video_codec: drv.video_codec || "h264",
            }
        };
    } else {
        // Format as requested by user
        finalConfig = {
            device_type: config.device_type,
            device_id: config.device_id || serial,
            device_ip: config.device_ip || '0',
            device_port: config.device_port || '0',
            av_sync: document.getElementById('configAVSync').checked,
            driver_config: {
                max_fps: String(drv.max_fps || ''),
                video_codec: drv.video_codec || "h264",
                audio_codec: drv.audio_codec || "opus",
                audio: drv.audio || "true",
                video_bit_rate: String(drv.video_bit_rate || 8000000),
                video_codec_options: drv.video_codec_options || '',
                new_display: drv.new_display || '',
                max_size: drv.max_size || '',
                deviceID: config.device_ip || '', // Pass device IP as deviceID for scrcpy to identify which device to connect to
            }
        };
    }

    console.log('Starting stream with config:', finalConfig);
    sessionStorage.setItem('webscreen_device_configs', JSON.stringify(finalConfig));
    showToast(i18n.t('starting_stream'));

    id = `${finalConfig.device_type}_${finalConfig.device_id}_${finalConfig.device_ip}_${finalConfig.device_port}`;
    // Delay slightly for UX
    setTimeout(() => {
        window.location.href = `/screen/${id}`;
    }, 500);
}

// --- Modal Logic ---

function openModal(id) {
    const dialog = document.getElementById(id);
    if (dialog) {
        dialog.showModal();
        // Add closing listener on backdrop click
        dialog.addEventListener('click', (e) => {
            if (e.target === dialog) closeModal(id);
        });
    }
}

function closeModal(id) {
    const dialog = document.getElementById(id);
    if (dialog) {
        // Animation out could be added here
        dialog.close();
    }
    if (id === 'configModal') activeConfigSerial = null;
}

function showConfigModal(serial) {
    activeConfigSerial = serial;
    const device = knownDevices.find(d => d.device_id === serial) || { device_id: serial };
    const config = ensureDeviceConfig(device);

    // New structure: everything is in driver_config
    const drv = config.driver_config || {};

    const androidSettings = document.getElementById('androidSettings');
    const xvfbSettings = document.getElementById('xvfbSettings');

    document.getElementById('configAVSync').checked = config.av_sync || false;
    console.log('Configuring modal for', serial, 'of type', config.device_type);
    if (config.device_type === 'xvfb') {
        console.log('Showing XVFB settings for', serial);
        androidSettings.classList.add('hidden');
        xvfbSettings.classList.remove('hidden');

        document.getElementById('xvfbIp').value = drv.ip || '';
        document.getElementById('xvfbUser').value = drv.user || '';
        document.getElementById('xvfbResolution').value = drv.resolution || '1920x1080';
        document.getElementById('xvfbFrameRate').value = drv.frameRate || '60';
        document.getElementById('xvfbBitRate').value = drv.bitRate || '20000000';
        document.getElementById('xvfbVideoCodec').value = drv.video_codec || 'h264';

    } else {
        androidSettings.classList.remove('hidden');
        xvfbSettings.classList.add('hidden');

        document.getElementById('configMaxFPS').value = drv.max_fps || '';
        document.getElementById('configVideoBitrate').value = formatBitrate(drv.video_bit_rate).replace(/[KMG]$/, '') || '';
        document.getElementById('configMaxSize').value = drv.max_size || '';
        document.getElementById('configVideoCodec').value = drv.video_codec || 'h264';
        // document.getElementById('configVideoCodecOptions').value = drv.video_codec_options || '';
        document.getElementById('configAudio').checked = drv.audio === 'true';
        document.getElementById('configNewDisplay').value = drv.new_display || '';
    }

    document.getElementById('configModalTitle').textContent = i18n.t('config_device_title', {serial: serial});
    openModal('configModal');
}

function saveDeviceConfig() {
    if (!activeConfigSerial) return;

    const device = knownDevices.find(d => d.device_id === activeConfigSerial);
    const config = ensureDeviceConfig(device);

    config.av_sync = document.getElementById('configAVSync').checked;

    // Initialize if missing
    if (!config.driver_config) config.driver_config = {};
    const drv = config.driver_config;

    if (config.device_type === 'xvfb') {
        drv.ip = document.getElementById('xvfbIp').value.trim();
        drv.user = document.getElementById('xvfbUser').value.trim();
        drv.resolution = document.getElementById('xvfbResolution').value.trim();
        drv.frameRate = document.getElementById('xvfbFrameRate').value.trim();
        drv.bitRate = document.getElementById('xvfbBitRate').value.trim();
        drv.video_codec = document.getElementById('xvfbVideoCodec').value;
    } else { // Android Scrcpy
        drv.max_fps = document.getElementById('configMaxFPS').value.trim() || '';
        // Parse bitrate input (e.g. "8") to number (8000000) using 'M' as default if not specified
        const bitrateInput = document.getElementById('configVideoBitrate').value.trim();
        // If user just types "8", treat as 8M. If "20000000", parseBitrate handles it?
        // Existing parseBitrate handles "8M" or "8000000". 
        // If user enters "8", we append "M" to maintain previous UX, or rewrite parseBitrate.
        // Let's assume input "8" means 8Mbps for simplicity in this UI context.
        drv.video_bit_rate = parseBitrate(bitrateInput + (bitrateInput.match(/[KMG]/i) ? '' : 'M'));
        drv.max_size = document.getElementById('configMaxSize').value.trim() || '';
        drv.video_codec = document.getElementById('configVideoCodec').value;
        // drv.video_codec_options = document.getElementById('configVideoCodecOptions').value.trim();
        drv.new_display = document.getElementById('configNewDisplay').value.trim();
        drv.audio = document.getElementById('configAudio').checked ? 'true' : 'false';
        drv.audio_codec = 'opus'; // Hardcoded default for now
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
