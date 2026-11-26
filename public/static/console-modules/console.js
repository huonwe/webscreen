// /static/console.js
import { getSetting } from '../global-settings.js';
import {
    t,
    escapeHtml,
    showToast,
    openConfirmDialog,
    openRenameDeviceDialog,
} from './ui-dialogs.js';
import {
    ensureUniversalSettingsInitialized,
    openGlobalSettingsModal,
} from './global-settings-ui.js';
import {
    openDeviceConfigModal,
    openRemoteConfigModal,
    getDeviceConfigEntry,
    setDeviceIgnored,
    getMergedDeviceStreamConfig,
    fetchDriverSchema,
} from './device-config-ui.js';
import {
    openAndroidPairModal,
    openConnectDeviceModal,
} from './android-device-ui.js';
import {
    openGameStreamPairModal,
} from './gamestream-pair-ui.js';

/**
 * Runtime state (kept in memory; persistent configs live in global_settings._device_config)
 */
const state = {
    devices: [],
    streamingStatusMap: new Map(), // uuid -> { streaming: boolean, config?: Record<string, string> }
    universalSchema: [],
    driverSchemas: new Map(), // `${type}:${device_id}` -> schemaArray
    showIgnored: false,
};

// --- Formatting Helpers ---

function formatBitrate(value) {
    const num = Number(value);
    if (!num || Number.isNaN(num)) return String(value);
    if (num >= 1_000_000_000) return `${(num / 1_000_000_000).toFixed(1)}G`;
    if (num >= 1_000_000) return `${Math.round(num / 1_000_000)}M`;
    if (num >= 1_000) return `${Math.round(num / 1_000)}K`;
    return String(num);
}

function getDeviceIcon(deviceType) {
    const type = String(deviceType || '').toLowerCase();
    if (type === 'gamestream') return 'sports_esports';
    if (type === 'linux' || type === 'xvfb' || type === 'windows' || type === 'desktop') return 'desktop_windows';
    return 'smartphone';
}

// --- Data Fetching ---

async function fetchStreamingStatus(device_uuid) {
    if (!device_uuid) return { streaming: false };
    try {
        const res = await fetch(`/api/device/streaming/status?device_uuid=${encodeURIComponent(device_uuid)}`);
        if (!res.ok) return { streaming: false };
        const data = await res.json();
        return {
            streaming: Boolean(data.streaming),
            config: data.config && typeof data.config === 'object' ? data.config : {},
        };
    } catch (err) {
        console.warn(`Failed to check streaming status for ${device_uuid}:`, err);
        return { streaming: false };
    }
}

async function refreshDevices({ silent = false } = {}) {
    const grid = document.getElementById('deviceGrid');
    if (grid) {
        grid.innerHTML = `
            <div class="col-span-full flex flex-col items-center justify-center py-20 text-gray-500">
                <div class="spinner mb-4"></div>
                <p>${escapeHtml(t('scanning_devices', '正在扫描设备...'))}</p>
            </div>
        `;
    }

    try {
        const [deviceRes, universalSchema] = await Promise.all([
            fetch('/api/device/list'),
            ensureUniversalSettingsInitialized(),
        ]);

        if (!deviceRes.ok) {
            throw new Error(`HTTP ${deviceRes.status}`);
        }

        const payload = await deviceRes.json();
        const devices = Array.isArray(payload.devices) ? payload.devices : [];

        state.devices = devices;
        state.universalSchema = universalSchema;
        state.streamingStatusMap.clear();

        // Concurrently fetch per-device driver schemas and active streaming statuses
        await Promise.all(
            devices.map(async (device) => {
                console.log("get devices:", device)
                const device_uuid = device.device_uuid;
                const schemaKey = `${device.device_type || 'unknown'}:${device.device_id || ''}`;

                const [driverSchema, streamStatus] = await Promise.all([
                    fetchDriverSchema(device.device_type, device.device_id),
                    fetchStreamingStatus(device_uuid),
                ]);

                state.driverSchemas.set(schemaKey, driverSchema);
                if (device_uuid) {
                    state.streamingStatusMap.set(device_uuid, streamStatus);
                }
            })
        );

        renderDeviceGrid();
        if (!silent) {
            showToast(t('refreshed_found', `已刷新，发现 ${devices.length} 台设备`, { n: devices.length }));
        }
    } catch (err) {
        console.error('Failed to fetch devices:', err);
        if (grid) {
            grid.innerHTML = `
                <div class="col-span-full flex flex-col items-center justify-center py-20 text-gray-400 bg-[#1e1f20]/50 rounded-3xl border border-dashed border-gray-700">
                    <span class="material-symbols-rounded text-5xl mb-3 text-red-400/80">cloud_off</span>
                    <p class="text-base mb-4">${escapeHtml(t('call_api_failed', '获取设备列表失败'))}</p>
                    <button type="button" id="retryFetchDevicesBtn"
                        class="px-5 py-2 rounded-full bg-[#2a2b2c] hover:bg-[#333] text-[var(--md-sys-color-primary)] text-sm font-medium transition-colors">
                        ${escapeHtml(t('refresh', '重试'))}
                    </button>
                </div>
            `;
            grid.querySelector('#retryFetchDevicesBtn')?.addEventListener('click', () => refreshDevices());
        }
        showToast(t('call_api_failed', '获取设备列表失败'), 'error');
    }
}

// --- Stream Launch / Join via URL Query Params ---

function launchStream(device) {
    const device_uuid = device.device_uuid;
    const schemaKey = `${device.device_type || 'unknown'}:${device.device_id || ''}`;
    const driverSchema = state.driverSchemas.get(schemaKey) || [];
    const { combined } = getMergedDeviceStreamConfig(device, state.universalSchema, driverSchema);

    const streamStatus = state.streamingStatusMap.get(device_uuid);
    const isJoining = Boolean(streamStatus?.streaming);
    const remoteConfig = streamStatus?.config || {};

    // Base device identity parameters + merged universal & driver configs
    const queryPayload = {
        device_uuid: device_uuid || '',
        device_type: device.device_type || '',
        device_id: device.device_id || '',
        device_ip: device.device_ip || '',
        device_port: device.device_port !== undefined && device.device_port !== null ? String(device.device_port) : '',
        ...combined,
    };

    // When joining an active stream, RemoteConfig takes precedence for video_codec and audio_codec
    if (isJoining) {
        queryPayload.join = 'true';
        if (remoteConfig.video_codec) {
            queryPayload.video_codec = remoteConfig.video_codec;
        }
        if (remoteConfig.audio_codec) {
            queryPayload.audio_codec = remoteConfig.audio_codec;
        }
    }

    const searchParams = new URLSearchParams();
    Object.entries(queryPayload).forEach(([key, value]) => {
        if (value !== undefined && value !== null && value !== '') {
            searchParams.set(key, String(value));
        }
    });

    showToast(
        isJoining
            ? t('joining_stream', '正在加入串流...')
            : t('starting_stream', '正在启动串流...')
    );

    const routeId = encodeURIComponent(device_uuid);
    const targetUrl = `/screen/${routeId}?${searchParams.toString()}`;

    setTimeout(() => {
        window.location.href = targetUrl;
    }, 250);
}

// --- Rendering ---

function buildConfigBadgesHtml(device) {
    const schemaKey = `${device.device_type || 'unknown'}:${device.device_id || ''}`;
    const driverSchema = state.driverSchemas.get(schemaKey) || [];
    const { universal, driver } = getMergedDeviceStreamConfig(device, state.universalSchema, driverSchema);

    const badges = [];

    const renderFromSchema = (schema, values) => {
        for (const param of schema) {
            const key = param.name;
            const val = values[key];
            if (val === undefined || val === null || val === '') continue;

            // Show badge-flagged params, or core codec/transport params in universal schema
            const isCoreUniversal = ['protocol_transport', 'transport_protocol', 'video_codec', 'audio_codec'].includes(key);
            if (!param.badge && !isCoreUniversal) continue;

            if (param.type === 'boolean') {
                const enabled = val === true || val === 'true';
                const label = t(key, key);
                badges.push(
                    enabled
                        ? `<span class="px-2 py-0.5 rounded-md bg-[#333] text-xs text-gray-300 font-mono">${escapeHtml(label)}</span>`
                        : `<span class="px-2 py-0.5 rounded-md bg-[#333] text-xs text-gray-400 font-mono line-through opacity-65">${escapeHtml(label)}</span>`
                );
            } else {
                let displayVal = String(val);
                if (key === 'video_bit_rate') displayVal = formatBitrate(displayVal);
                else if (key === 'max_fps' || key === 'frameRate') displayVal += 'FPS';
                else if (key.includes('codec') || key.includes('protocol')) displayVal = displayVal.toUpperCase();

                badges.push(
                    `<span class="px-2 py-0.5 rounded-md bg-[#333] text-xs text-gray-300 font-mono">${escapeHtml(displayVal)}</span>`
                );
            }
        }
    };

    renderFromSchema(state.universalSchema, universal);
    renderFromSchema(driverSchema, driver);

    return badges.length
        ? badges.join('')
        : `<span class="text-xs text-gray-500 italic">${escapeHtml(t('default_config', '默认配置'))}</span>`;
}


function buildDeviceCard(device) {
    const device_uuid = device.device_uuid;
    const entry = getDeviceConfigEntry(device_uuid);
    const isIgnored = Boolean(entry.ignored);
    const streamStatus = state.streamingStatusMap.get(device_uuid) || { streaming: false, config: {} };
    const isStreaming = streamStatus.streaming;

    const displayName = device.displayName || device.device_id || device_uuid;
    const endpoint = device.device_ip && device.device_ip !== '0'
        ? (device.device_port ? `${device.device_ip}:${device.device_port}` : device.device_ip)
        : '';
    const subParts = [device.device_id, endpoint].filter(Boolean);
    const subLine = subParts.join(' · ');

    const card = document.createElement('div');
    card.className = [
        'card rounded-[24px] p-5 flex flex-col justify-between h-full border transition-all group',
        isIgnored ? 'opacity-45 grayscale border-transparent' : 'border-transparent hover:border-[#444]',
    ].join(' ');

    const streamingBadgeHtml = isStreaming
        ? `<span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px] font-medium bg-emerald-500/15 text-emerald-300">
               <span class="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse"></span>
               ${escapeHtml(t('streaming_active', '串流中'))}
           </span>`
        : '';

    const statusBadgeHtml = device.device_status
        ? `<span class="px-2 py-0.5 rounded-full bg-[#2a2b2c] text-[11px] text-gray-300 font-mono">${escapeHtml(device.device_status)}</span>`
        : '';

    const footerHtml = isStreaming
        ? `
            <div class="flex items-center gap-2.5 mt-auto">
                <button type="button" data-action="stream"
                    class="flex-1 py-3 px-4 rounded-full bg-emerald-500/20 text-emerald-300 border border-emerald-500/40 hover:bg-emerald-500 hover:text-black font-medium text-sm transition-all flex items-center justify-center gap-1.5 whitespace-nowrap">
                    <span class="material-symbols-rounded text-lg">cast_connected</span>
                    <span>${escapeHtml(t('join_stream', '加入串流'))}</span>
                </button>
                <button type="button" data-action="remote-config"
                    class="py-3 px-4 rounded-full bg-[#2a2b2c] hover:bg-[#333] text-gray-200 border border-[#3a3b3c] font-medium text-sm transition-colors flex items-center justify-center gap-1.5 shrink-0 whitespace-nowrap"
                    title="${escapeHtml(t('remote_config', '远端配置'))}">
                    <span class="material-symbols-rounded text-lg text-gray-300">tune</span>
                    <span>${escapeHtml(t('remote_config', '远端配置'))}</span>
                </button>
            </div>
        `
        : `
            <button type="button" data-action="stream"
                class="w-full py-3 rounded-full bg-[#2a2b2c] group-hover:bg-[var(--md-sys-color-primary)] group-hover:text-[var(--md-sys-color-on-primary)] text-[var(--md-sys-color-primary)] font-medium transition-all flex items-center justify-center gap-2 mt-auto">
                <span class="material-symbols-rounded">play_arrow</span>
                <span>${escapeHtml(t('start_stream', '开始串流'))}</span>
            </button>
        `;

    card.innerHTML = `
        <div>
            <div class="flex justify-between items-start gap-2 mb-4">
                <div class="flex items-center gap-3 min-w-0">
                    <div class="w-10 h-10 rounded-full bg-[var(--md-sys-color-secondary-container)] flex items-center justify-center text-[var(--md-sys-color-on-secondary-container)] shrink-0">
                        <span class="material-symbols-rounded">${getDeviceIcon(device.device_type)}</span>
                    </div>
                    <div class="min-w-0">
                        <h3 class="font-medium text-lg leading-tight text-[#e3e3e3] truncate" title="${escapeHtml(displayName)}">
                            ${escapeHtml(displayName)}
                        </h3>
                        <div class="flex flex-wrap items-center gap-1.5 mt-1">
                            <span class="px-2 py-0.5 rounded-full bg-[#2a2b2c] text-[11px] text-gray-300 font-mono uppercase">
                                ${escapeHtml(device.device_type || 'unknown')}
                            </span>
                            ${streamingBadgeHtml}
                            ${statusBadgeHtml}
                        </div>
                        ${subLine ? `<p class="text-[11px] text-gray-500 font-mono mt-1 truncate" title="${escapeHtml(subLine)}">${escapeHtml(subLine)}</p>` : ''}
                    </div>
                </div>

                <div class="flex items-center shrink-0">
                    <button type="button" data-action="rename"
                        class="p-2 rounded-full hover:bg-white/10 text-gray-400 hover:text-white transition-colors"
                        title="${escapeHtml(t('rename_device', '修改显示名称'))}">
                        <span class="material-symbols-rounded text-[20px]">edit</span>
                    </button>
                    <button type="button" data-action="toggle-ignore"
                        class="p-2 rounded-full hover:bg-white/10 ${isIgnored ? 'text-orange-400' : 'text-gray-400 hover:text-white'} transition-colors"
                        title="${escapeHtml(isIgnored ? t('unignore_device', '取消忽略') : t('ignore_device', '忽略设备'))}">
                        <span class="material-symbols-rounded text-[20px]">${isIgnored ? 'visibility' : 'visibility_off'}</span>
                    </button>
                    <button type="button" data-action="configure"
                        class="p-2 rounded-full hover:bg-white/10 text-gray-400 hover:text-white transition-colors"
                        title="${escapeHtml(t('configure_device', '配置设备'))}">
                        <span class="material-symbols-rounded text-[20px]">settings</span>
                    </button>
                </div>
            </div>

            <div class="flex flex-wrap gap-2 mb-5">
                ${buildConfigBadgesHtml(device)}
            </div>
        </div>

        ${footerHtml}
    `;

    // Card Event Bindings
    card.querySelector('[data-action="stream"]')?.addEventListener('click', () => {
        launchStream(device);
    });
    card.querySelector('[data-action="remote-config"]')?.addEventListener('click', () => {
        console.log('Opening remote config modal for device:', device, streamStatus.config);
        openRemoteConfigModal(device, streamStatus.config, {
            universalSchema: state.universalSchema,
        });
    });

    card.querySelector('[data-action="rename"]')?.addEventListener('click', async () => {
        const updatedName = await openRenameDeviceDialog({
            uuid,
            currentName: device.displayName || device.device_id || '',
            deviceId: device.device_id || '',
        });
        if (updatedName !== null) {
            device.displayName = updatedName;
            renderDeviceGrid();
        }
    });

    card.querySelector('[data-action="toggle-ignore"]')?.addEventListener('click', () => {
        const nextIgnored = !Boolean(getDeviceConfigEntry(uuid).ignored);
        setDeviceIgnored(uuid, nextIgnored);
        showToast(
            nextIgnored
                ? t('device_ignored', '设备已忽略')
                : t('device_unignored', '设备已取消忽略')
        );
        renderDeviceGrid();
    });

    card.querySelector('[data-action="configure"]')?.addEventListener('click', () => {
        openDeviceConfigModal(device, {
            universalSchema: state.universalSchema,
            onSaved: () => renderDeviceGrid(),
        });
    });

    return card;
}

function updateIgnoredToggleButton() {
    const toggleBtn = document.getElementById('toggleIgnoredBtn');
    const iconEl = document.getElementById('toggleIgnoredIcon');
    const badgeEl = document.getElementById('ignoredCountBadge');
    if (!toggleBtn) return;

    const ignoredCount = state.devices.filter((d) => Boolean(getDeviceConfigEntry(d.uuid).ignored)).length;

    if (ignoredCount > 0) {
        toggleBtn.classList.remove('hidden');
        toggleBtn.classList.add('flex');
        if (badgeEl) badgeEl.textContent = String(ignoredCount);
    } else {
        toggleBtn.classList.add('hidden');
        toggleBtn.classList.remove('flex');
        state.showIgnored = false;
    }

    if (state.showIgnored) {
        toggleBtn.classList.add('bg-[#333]', 'text-white');
        if (iconEl) iconEl.textContent = 'visibility';
    } else {
        toggleBtn.classList.remove('bg-[#333]', 'text-white');
        if (iconEl) iconEl.textContent = 'visibility_off';
    }
}

function renderDeviceGrid() {
    const grid = document.getElementById('deviceGrid');
    if (!grid) return;

    updateIgnoredToggleButton();
    grid.innerHTML = '';

    const visibleDevices = state.devices.filter((device) => {
        const entry = getDeviceConfigEntry(device.uuid);
        return state.showIgnored || !entry.ignored;
    });

    if (!visibleDevices.length) {
        grid.innerHTML = `
            <div class="col-span-full flex flex-col items-center justify-center py-20 text-gray-500 bg-[#1e1f20]/50 rounded-3xl border border-dashed border-gray-700">
                <span class="material-symbols-rounded text-5xl mb-4 opacity-50">phonelink_off</span>
                <p class="text-lg">${escapeHtml(t('no_devices', '没有可显示的设备'))}</p>
                <button type="button" id="emptyStateConnectBtn"
                    class="mt-4 text-[var(--md-sys-color-primary)] hover:underline font-medium text-sm">
                    ${escapeHtml(t('connect_device', '连接设备'))}
                </button>
            </div>
        `;
        grid.querySelector('#emptyStateConnectBtn')?.addEventListener('click', () => {
            openConnectDeviceModal({ onConnected: () => refreshDevices() });
        });
        return;
    }

    visibleDevices.forEach((device) => {
        grid.appendChild(buildDeviceCard(device));
    });
}

// --- Toolbar & Global Event Bindings ---

function bindToolbarEvents() {
    // Language switcher
    document.querySelectorAll('.lang-btn').forEach((btn) => {
        btn.addEventListener('click', () => {
            const lang = btn.dataset.lang;
            if (lang && window.i18n && typeof window.i18n.setLang === 'function') {
                window.i18n.setLang(lang);
                renderDeviceGrid();
            }
        });
    });

    // Global settings modal trigger
    document.getElementById('globalSettingsBtn')?.addEventListener('click', () => {
        openGlobalSettingsModal({
            onSaved: async () => {
                state.universalSchema = await ensureUniversalSettingsInitialized();
                renderDeviceGrid();
            },
        });
    });

    // Clear localStorage trigger
    document.getElementById('clearStorageBtn')?.addEventListener('click', async () => {
        const confirmed = await openConfirmDialog({
            title: t('clear_local_storage', '清空本地存储'),
            message: t('confirm_clear_local_storage', '确定要清空所有本地保存的全局与设备配置吗？'),
            danger: true,
        });
        if (!confirmed) return;

        try {
            localStorage.clear();
            window.location.reload();
        } catch (err) {
            console.error('Failed to clear localStorage:', err);
            showToast(t('clear_local_storage_failed', '清空本地存储失败'), 'error');
        }
    });

    // Manual device refresh trigger (only refreshes when clicked or on initial load)
    document.getElementById('refreshDevicesBtn')?.addEventListener('click', () => {
        refreshDevices();
    });

    // Toggle ignored devices visibility
    document.getElementById('toggleIgnoredBtn')?.addEventListener('click', () => {
        state.showIgnored = !state.showIgnored;
        renderDeviceGrid();
    });

    // Android wireless pairing trigger
    document.getElementById('openAndroidPairBtn')?.addEventListener('click', () => {
        openAndroidPairModal({
            onPaired: () => refreshDevices(),
        });
    });

    // GameStream pairing trigger
    document.getElementById('openGameStreamPairBtn')?.addEventListener('click', () => {
        openGameStreamPairModal({
            onPaired: () => refreshDevices(),
        });
    });

    // Connect device trigger
    document.getElementById('openConnectDeviceBtn')?.addEventListener('click', () => {
        openConnectDeviceModal({
            onConnected: () => refreshDevices(),
        });
    });
}

document.addEventListener('DOMContentLoaded', () => {
    // Ensure global_settings is initialized on startup
    getSetting('_device_config');
    bindToolbarEvents();
    refreshDevices({ silent: true });
});