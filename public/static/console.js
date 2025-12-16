const deviceConfigs = {};
let knownDevices = [];
let activeConfigSerial = null;

const defaultScrcpyOptions = {
    MaxFPS: '60',
    VideoBitRate: '8M',
    Control: true,
    audio: true,
    videoCodec: 'h264',
    VideoCodecOptions: '',
    newDisplay: {
        width: '',
        height: '',
        fps: ''
    }
};

function getDefaultConfig(serial) {
    return {
        device_serial: serial,
        proxy_port: '6000',
        scrcpyOptions: JSON.parse(JSON.stringify(defaultScrcpyOptions))
    };
}

function ensureDeviceConfig(serial) {
    if (!deviceConfigs[serial]) {
        deviceConfigs[serial] = getDefaultConfig(serial);
    }
    return deviceConfigs[serial];
}

function pruneDeviceConfigs(activeDevices) {
    Object.keys(deviceConfigs).forEach(serial => {
        if (!activeDevices.includes(serial)) {
            delete deviceConfigs[serial];
        }
    });
}

function formatScrcpySummary(options) {
    if (!options) {
        return '—';
    }
    const parts = [];
    if (options.MaxFPS) {
        parts.push(`${options.MaxFPS} fps`);
    }
    if (options.VideoBitRate) {
        parts.push(options.VideoBitRate);
    }
    if (options.videoCodec) {
        parts.push(options.videoCodec.toUpperCase());
    }
    if (options.VideoCodecOptions) {
        parts.push(options.VideoCodecOptions);
    }
    parts.push(`control:${options.Control ? 'on' : 'off'}`);
    parts.push(`audio:${options.audio ? 'on' : 'off'}`);

    if (options.newDisplay) {
        const dims = [];
        if (options.newDisplay.width) {
            dims.push(options.newDisplay.width);
        }
        if (options.newDisplay.height) {
            dims.push(options.newDisplay.height);
        }
        let display = dims.length ? dims.join('x') : '';
        if (options.newDisplay.fps) {
            display = display ? `${display}@${options.newDisplay.fps}` : `${options.newDisplay.fps}fps`;
        }
        if (display) {
            parts.push(`display:${display}`);
        }
    }

    return parts.join(' • ');
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

    knownDevices.forEach(serial => {
        const config = ensureDeviceConfig(serial);
        const tr = document.createElement('tr');
        tr.dataset.serial = serial;
        tr.innerHTML = `
            <td>${serial}</td>
            <td><span class="status-connected">Connected</span></td>
            <td class="device-summary">${formatScrcpySummary(config.scrcpyOptions)}</td>
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
        const response = await fetch('/api/devices');
        const data = await response.json();
        const devices = Array.isArray(data.devices) ? data.devices : [];
        knownDevices = devices;
        pruneDeviceConfigs(devices);
        devices.forEach(ensureDeviceConfig);
        renderDeviceList();
    } catch (error) {
        console.error('Error fetching devices:', error);
        alert('Failed to fetch devices');
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
        const response = await fetch('/api/connect', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ ip, port })
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
        const response = await fetch('/api/pair', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ ip, port, code })
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
    const config = ensureDeviceConfig(serial);
    activeConfigSerial = serial;

    document.getElementById('configModalTitle').textContent = `Configure ${serial}`;
    document.getElementById('configProxyPort').value = config.proxy_port || '6000';
    document.getElementById('configMaxFPS').value = config.scrcpyOptions.MaxFPS || '';
    document.getElementById('configVideoBitrate').value = config.scrcpyOptions.VideoBitRate || '';
    document.getElementById('configVideoCodec').value = config.scrcpyOptions.videoCodec || 'h264';
    document.getElementById('configVideoCodecOptions').value = config.scrcpyOptions.VideoCodecOptions || '';
    document.getElementById('configControl').checked = Boolean(config.scrcpyOptions.Control);
    document.getElementById('configAudio').checked = Boolean(config.scrcpyOptions.audio);
    document.getElementById('configDisplayWidth').value = config.scrcpyOptions.newDisplay.width || '';
    document.getElementById('configDisplayHeight').value = config.scrcpyOptions.newDisplay.height || '';
    document.getElementById('configDisplayFPS').value = config.scrcpyOptions.newDisplay.fps || '';

    document.getElementById('configModal').style.display = 'flex';
}

function saveDeviceConfig() {
    if (!activeConfigSerial) {
        return;
    }

    const config = ensureDeviceConfig(activeConfigSerial);

    const proxyPortInput = document.getElementById('configProxyPort').value.trim();
    config.proxy_port = proxyPortInput || '6000';

    config.scrcpyOptions.MaxFPS = document.getElementById('configMaxFPS').value.trim() || '';
    config.scrcpyOptions.VideoBitRate = document.getElementById('configVideoBitrate').value.trim() || '';
    config.scrcpyOptions.videoCodec = document.getElementById('configVideoCodec').value;
    config.scrcpyOptions.VideoCodecOptions = document.getElementById('configVideoCodecOptions').value.trim();
    config.scrcpyOptions.Control = document.getElementById('configControl').checked;
    config.scrcpyOptions.audio = document.getElementById('configAudio').checked;
    config.scrcpyOptions.newDisplay.width = document.getElementById('configDisplayWidth').value.trim();
    config.scrcpyOptions.newDisplay.height = document.getElementById('configDisplayHeight').value.trim();
    config.scrcpyOptions.newDisplay.fps = document.getElementById('configDisplayFPS').value.trim();

    renderDeviceList();
    closeModal('configModal');
}

function buildStartPayload(serial) {
    const config = ensureDeviceConfig(serial);
    return {
        device_serial: config.device_serial || serial,
        proxy_port: config.proxy_port || '6000',
        scrcpyOptions: {
            MaxFPS: config.scrcpyOptions.MaxFPS,
            VideoBitRate: config.scrcpyOptions.VideoBitRate,
            Control: Boolean(config.scrcpyOptions.Control),
            audio: Boolean(config.scrcpyOptions.audio),
            videoCodec: config.scrcpyOptions.videoCodec,
            VideoCodecOptions: config.scrcpyOptions.VideoCodecOptions,
            newDisplay: {
                height: config.scrcpyOptions.newDisplay.height,
                width: config.scrcpyOptions.newDisplay.width,
                fps: config.scrcpyOptions.newDisplay.fps
            }
        }
    };
}

async function startStream(serial) {
    const payload = buildStartPayload(serial);
    console.log('Starting stream with payload:', payload);
    try {
        const response = await fetch('/api/start_stream', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        const data = await response.json();
        
        if (response.ok) {
            window.location.href = '/';
        } else {
            alert('Failed to start stream: ' + data.error);
        }
    } catch (error) {
        console.error('Error starting stream:', error);
        alert('Error starting stream');
    }
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
