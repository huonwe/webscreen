const STORAGE_KEY = 'webscreen_device_configs';

let knownDevices = [];
let activeConfigSerial = null;

const defaultStreamConfig = {
    video_codec: 'h264',
    audio_codec: 'opus',
    avsync: false,
    bitrate: 8000000,
    driver_config: {
        max_fps: '60',
        video_codec_options: '',
        // new_display: '1920x1080/60'  // format: "1920x1080/60" or empty
    }
};

function getDefaultConfig(device) {
    return {
        device_type: device.type || 'android',
        device_id: device.device_id,
        device_ip: device.ip || '0',
        device_port: device.port || '0',
        stream_config: JSON.parse(JSON.stringify(defaultStreamConfig))
    };
}

// Load configs from localStorage
function loadDeviceConfigs() {
    try {
        const stored = localStorage.getItem(STORAGE_KEY);
        return stored ? JSON.parse(stored) : {};
    } catch (e) {
        console.error('Failed to load device configs from localStorage:', e);
        return {};
    }
}

// Save configs to localStorage
function saveDeviceConfigs(configs) {
    try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(configs));
    } catch (e) {
        console.error('Failed to save device configs to localStorage:', e);
    }
}

// Get all device configs
let deviceConfigs = loadDeviceConfigs();

function ensureDeviceConfig(device) {
    const serial = typeof device === 'string' ? device : device.device_id;
    if (!deviceConfigs[serial]) {
        deviceConfigs[serial] = getDefaultConfig(typeof device === 'string' ? { device_id: device } : device);
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
    if (changed) {
        saveDeviceConfigs(deviceConfigs);
    }
}

function formatStreamSummary(streamConfig) {
    if (!streamConfig) {
        return '—';
    }
    const parts = [];
    const opts = streamConfig.driver_config || {};
    
    if (opts.max_fps) {
        parts.push(`${opts.max_fps} fps`);
    }
    if (streamConfig.bitrate) {
        parts.push(formatBitrate(streamConfig.bitrate));
    }
    if (streamConfig.video_codec) {
        parts.push(streamConfig.video_codec.toUpperCase());
    }
    if (opts.video_codec_options) {
        parts.push(opts.video_codec_options);
    }

    if (opts.new_display) {
        parts.push(`display:${opts.new_display}`);
    }

    return parts.join(' • ');
}

function formatBitrate(value) {
    if (!value) return '';
    if (value >= 1000000000) return `${(value / 1000000000).toFixed(1)}G`;
    if (value >= 1000000) return `${(value / 1000000).toFixed(0)}M`;
    if (value >= 1000) return `${(value / 1000).toFixed(0)}K`;
    return String(value);
}

function renderDeviceList() {
    const tbody = document.querySelector('#deviceTable tbody');
    if (!tbody) {
        return;
    }

    tbody.innerHTML = '';

    if (!knownDevices.length) {
        tbody.innerHTML = '<tr class="empty-row"><td colspan="4">No devices connected</td></tr>';
        return;
    }

    knownDevices.forEach(device => {
        // Handle both string (legacy/fallback) and object formats
        const serial = typeof device === 'string' ? device : device.device_id;
        const config = ensureDeviceConfig(device);
        const tr = document.createElement('tr');
        tr.dataset.serial = serial;

        let displayInfo = serial;
        if (device.ip && device.ip !== '0') {
            displayInfo += ` <span style="color: #888; font-size: 0.9em;">(${device.ip}:${device.port})</span>`;
        }

        tr.innerHTML = `
            <td>${displayInfo}</td>
            <td><span class="status-connected">${device.status || 'Connected'}</span></td>
            <td class="device-summary">${formatStreamSummary(config.stream_config)}</td>
            <td class="device-actions">
                <button class="btn btn-ghost btn-small" data-action="configure" data-serial="${serial}">Configure</button>
                <button class="btn btn-secondary btn-small" data-action="start" data-serial="${serial}">Start Stream</button>
            </td>
        `;
        tbody.appendChild(tr);
    });
}

async function fetchDevices() {
    try {
        const response = await fetch('/api/device/list');
        const data = await response.json();
        console.log('Fetched devices:', data);
        // API returns: { devices: [{type, device_id, ip, port, status}, ...] }
        const devices = Array.isArray(data.devices) ? data.devices : [];
        knownDevices = devices;

        const serials = devices.map(d => d.device_id);
        pruneDeviceConfigs(serials);
        devices.forEach(d => ensureDeviceConfig(d));

        renderDeviceList();
    } catch (error) {
        console.error('Error fetching devices:', error);
        // alert('Failed to fetch devices');
    }
}

async function connectDevice() {
    const ip = document.getElementById('connectIP').value;
    const port = document.getElementById('connectPort').value;

    if (!ip) {
        alert('Please enter IP address');
        return;
    }

    try {
        const response = await fetch('/api/device/connect', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ device_type: 'android', ip, port })
        });
        const data = await response.json();
        
        if (response.ok) {
            alert('Connected successfully!');
            closeModal('connectModal');
            fetchDevices();
        } else {
            alert('Connection failed: ' + data.error);
        }
    } catch (error) {
        console.error('Error connecting:', error);
        alert('Error connecting to device');
    }
}

async function pairDevice() {
    const ip = document.getElementById('pairIP').value;
    const port = document.getElementById('pairPort').value;
    const code = document.getElementById('pairCode').value;

    if (!ip || !port || !code) {
        alert('Please fill all fields');
        return;
    }

    try {
        const response = await fetch('/api/device/pair', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ device_type: 'android', ip, port, code })
        });
        const data = await response.json();
        
        if (response.ok) {
            alert('Paired successfully! Now you can connect.');
            closeModal('pairModal');
            // Pre-fill connect modal
            document.getElementById('connectIP').value = ip;
            showConnectModal();
        } else {
            alert('Pairing failed: ' + data.error);
        }
    } catch (error) {
        console.error('Error pairing:', error);
        alert('Error pairing device');
    }
}

function showConnectModal() {
    document.getElementById('connectModal').style.display = 'flex';
}

function showPairModal() {
    document.getElementById('pairModal').style.display = 'flex';
}

function closeModal(id) {
    document.getElementById(id).style.display = 'none';
    if (id === 'configModal') {
        activeConfigSerial = null;
    }
}

// Close modal when clicking outside
window.onclick = function(event) {
    if (event.target.classList.contains('modal')) {
        closeModal(event.target.id);
    }
}

function showConfigModal(serial) {
    // Find the device object from knownDevices
    const device = knownDevices.find(d => d.device_id === serial) || { device_id: serial };
    const config = ensureDeviceConfig(device);
    activeConfigSerial = serial;

    const opts = config.stream_config.driver_config || {};

    document.getElementById('configModalTitle').textContent = `Configure ${serial}`;
    document.getElementById('configMaxFPS').value = opts.max_fps || '';
    document.getElementById('configVideoBitrate').value = formatBitrate(config.stream_config.bitrate) || '';
    document.getElementById('configVideoCodec').value = config.stream_config.video_codec || 'h264';
    document.getElementById('configVideoCodecOptions').value = opts.video_codec_options || '';
    document.getElementById('configNewDisplay').value = opts.new_display || '';

    document.getElementById('configModal').style.display = 'flex';
}

function saveDeviceConfig() {
    if (!activeConfigSerial) {
        return;
    }

    // Find the device object from knownDevices
    const device = knownDevices.find(d => d.device_id === activeConfigSerial) || { device_id: activeConfigSerial };
    const config = ensureDeviceConfig(device);

    // Ensure driver_config exists
    if (!config.stream_config.driver_config) {
        config.stream_config.driver_config = {};
    }
    const opts = config.stream_config.driver_config;

    opts.max_fps = document.getElementById('configMaxFPS').value.trim() || '';
    config.stream_config.bitrate = parseBitrate(document.getElementById('configVideoBitrate').value.trim());
    config.stream_config.video_codec = document.getElementById('configVideoCodec').value;
    opts.video_codec_options = document.getElementById('configVideoCodecOptions').value.trim();
    opts.new_display = document.getElementById('configNewDisplay').value.trim();

    // Save to localStorage
    saveDeviceConfigs(deviceConfigs);

    renderDeviceList();
    closeModal('configModal');
}

// Parse bitrate string like "8M" to number
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

function startStream(serial) {
    // Find the device object from knownDevices
    const device = knownDevices.find(d => d.device_id === serial);
    if (!device) {
        alert('Device not found: ' + serial);
        return;
    }

    const config = ensureDeviceConfig(device);
    
    // Build the CONFIG object that connect.js expects
    const streamConfig = {
        device_type: config.device_type || 'android',
        device_id: config.device_id || serial,
        device_ip: config.device_ip || '0',
        device_port: config.device_port || '0',
        stream_config: {
            video_codec: config.stream_config.video_codec || 'h264',
            audio_codec: config.stream_config.audio_codec || 'opus',
            bitrate: config.stream_config.bitrate || 8000000,
            driver_config: config.stream_config.driver_config || {}
        }
    };

    // Store CONFIG in sessionStorage so connect.js can access it
    sessionStorage.setItem('webscreen_stream_config', JSON.stringify(streamConfig));

    
    window.location.href = `/screen/display`;
}

// Initial load
fetchDevices();

const deviceTableBody = document.querySelector('#deviceTable tbody');
if (deviceTableBody) {
    deviceTableBody.addEventListener('click', event => {
        const button = event.target.closest('button[data-action]');
        if (!button) {
            return;
        }
        const { action, serial } = button.dataset;
        if (!serial) {
            return;
        }
        if (action === 'configure') {
            showConfigModal(serial);
        } else if (action === 'start') {
            startStream(serial);
        }
    });
}
