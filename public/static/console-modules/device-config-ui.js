// /static/console-modules/device-config-ui.js
import { getSetting, updateSetting } from '../global-settings.js';
import {
    t,
    escapeHtml,
    showToast,
    createModalShell,
    normalizeSchema,
    renderSchemaFields,
    collectSchemaValues,
} from './ui-dialogs.js';
import { ensureUniversalSettingsInitialized } from './global-settings-ui.js';

const driverSchemaCache = new Map();

function readDeviceConfigMap() {
    const raw = getSetting('_device_config');
    if (raw && typeof raw === 'object' && !Array.isArray(raw)) {
        return raw;
    }
    const empty = {};
    updateSetting('_device_config', empty);
    return empty;
}

/**
 * Retrieve (and lazily initialize) a device's configuration record keyed by device_uuid.
 */
export function getDeviceConfigEntry(device_uuid) {
    if (!device_uuid) {
        return { ignored: false, universal: {}, driver_config: {} };
    }
    const allConfigs = readDeviceConfigMap();
    if (!allConfigs[device_uuid] || typeof allConfigs[device_uuid] !== 'object') {
        allConfigs[device_uuid] = {
            ignored: false,
            universal: {},
            driver_config: {},
        };
        updateSetting('_device_config', allConfigs);
    } else {
        allConfigs[device_uuid].ignored = Boolean(allConfigs[device_uuid].ignored);
        allConfigs[device_uuid].universal = allConfigs[device_uuid].universal && typeof allConfigs[device_uuid].universal === 'object'
            ? allConfigs[device_uuid].universal
            : {};
        allConfigs[device_uuid].driver_config = allConfigs[device_uuid].driver_config && typeof allConfigs[device_uuid].driver_config === 'object'
            ? allConfigs[device_uuid].driver_config
            : {};
    }
    return allConfigs[device_uuid];
}

export function saveDeviceConfigEntry(device_uuid, nextEntry) {
    if (!device_uuid) return;
    const allConfigs = readDeviceConfigMap();
    allConfigs[device_uuid] = {
        ignored: Boolean(nextEntry.ignored),
        universal: nextEntry.universal || {},
        driver_config: nextEntry.driver_config || {},
    };
    updateSetting('_device_config', allConfigs);
}

export function setDeviceIgnored(device_uuid, ignored) {
    if (!device_uuid) return;
    const entry = getDeviceConfigEntry(device_uuid);
    saveDeviceConfigEntry(device_uuid, {
        ...entry,
        ignored: Boolean(ignored),
    });
}

/**
 * Fetch and cache driver-specific configuration schema from backend.
 */
export async function fetchDriverSchema(deviceType, deviceId = '', forceRefresh = false) {
    const type = deviceType || 'unknown';
    const cacheKey = `${type}:${deviceId}`;
    if (driverSchemaCache.has(cacheKey) && !forceRefresh) {
        return driverSchemaCache.get(cacheKey);
    }

    try {
        const query = new URLSearchParams({ device_type: type });
        if (deviceId) query.set('device_id', deviceId);

        const res = await fetch(`/api/device/configDescription?${query.toString()}`);
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        const raw = await res.json();
        const normalized = normalizeSchema(raw);
        driverSchemaCache.set(cacheKey, normalized);
        return normalized;
    } catch (err) {
        console.warn(`Failed to fetch driver schema for ${cacheKey}:`, err);
        return driverSchemaCache.get(cacheKey) || [];
    }
}

/**
 * Compute effective universal + driver config for a device:
 * - Universal fields fall back to global_settings when left empty on the device.
 * - Driver fields use device overrides or schema defaults.
 */
export function getMergedDeviceStreamConfig(device, universalSchema = [], driverSchema = []) {
    const entry = getDeviceConfigEntry(device?.device_uuid);
    const effectiveUniversal = {};
    const effectiveDriver = {};

    // 1. Universal config (device override -> global_settings fallback)
    const universalKeys = new Set([
        ...normalizeSchema(universalSchema).map((p) => p.name),
        ...Object.keys(entry.universal || {}),
    ]);

    for (const key of universalKeys) {
        const deviceVal = entry.universal?.[key];
        if (deviceVal !== undefined && deviceVal !== null && deviceVal !== '') {
            effectiveUniversal[key] = deviceVal;
        } else {
            const globalVal = getSetting(key);
            if (globalVal !== undefined && globalVal !== null && globalVal !== '') {
                effectiveUniversal[key] = globalVal;
            }
        }
    }

    // 2. Driver config (device override -> schema default)
    for (const param of normalizeSchema(driverSchema)) {
        const key = param.name;
        const deviceVal = entry.driver_config?.[key];
        if (deviceVal !== undefined && deviceVal !== null && deviceVal !== '') {
            effectiveDriver[key] = deviceVal;
        } else if (param.default !== undefined && param.default !== null && param.default !== '') {
            effectiveDriver[key] = param.default;
        }
    }

    Object.entries(entry.driver_config || {}).forEach(([key, val]) => {
        if (!(key in effectiveDriver) && val !== undefined && val !== null && val !== '') {
            effectiveDriver[key] = val;
        }
    });

    return {
        universal: effectiveUniversal,
        driver: effectiveDriver,
        combined: {
            ...effectiveUniversal,
            ...effectiveDriver,
        },
    };
}

/**
 * Dynamically mount, populate, save, and destroy the per-device configuration modal.
 */
export async function openDeviceConfigModal(device, { universalSchema = null, onSaved } = {}) {
    const device_uuid = device.device_uuid;
    const displayTitle = device.displayName || device.device_id || device_uuid;

    const modal = createModalShell({
        id: 'deviceConfigModal',
        title: t('config_device_title', `配置设备 - ${displayTitle}`, { serial: displayTitle }),
        subtitle: t('configure_subtitle', '设置单设备串流参数（通用项留空则继承全局设置）'),
        icon: 'settings',
        maxWidthClass: 'max-w-lg',
        bodyHtml: `
            <section class="space-y-2.5">
                <div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider text-gray-400 px-1">
                    <span class="material-symbols-rounded text-base text-[var(--md-sys-color-primary)]">tune</span>
                    <span>${escapeHtml(t('universal_device_overrides', '通用参数（留空继承全局）'))}</span>
                </div>
                <div id="devUniversalContainer" class="min-h-[4rem] flex flex-col justify-center">
                    <div class="flex justify-center py-6"><div class="spinner"></div></div>
                </div>
            </section>

            <section class="space-y-2.5">
                <div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider text-gray-400 px-1">
                    <span class="material-symbols-rounded text-base text-[var(--md-sys-color-primary)]">developer_board</span>
                    <span>${escapeHtml(t('driver_specific_settings', '设备驱动参数'))}</span>
                </div>
                <div id="devDriverContainer" class="min-h-[6rem] flex flex-col justify-center">
                    <div class="flex justify-center py-6"><div class="spinner"></div></div>
                </div>
            </section>
        `,
        footerHtml: `
            <button type="button" data-dialog-close
                class="px-5 py-2.5 rounded-full text-[var(--md-sys-color-primary)] hover:bg-white/5 font-medium text-sm transition-colors">
                ${escapeHtml(t('cancel', '取消'))}
            </button>
            <button type="button" id="devConfigSaveBtn"
                class="px-6 py-2.5 rounded-full bg-[var(--md-sys-color-primary)] text-[var(--md-sys-color-on-primary)] font-medium text-sm shadow-sm hover:opacity-90 transition-opacity">
                ${escapeHtml(t('save_settings', '保存设置'))}
            </button>
        `,
    });

    const [resolvedUniversalSchema, driverSchema] = await Promise.all([
        universalSchema ? Promise.resolve(universalSchema) : ensureUniversalSettingsInitialized(),
        fetchDriverSchema(device.device_type, device.device_id),
    ]);

    const entry = getDeviceConfigEntry(device_uuid);
    const globalFallbackValues = {};
    resolvedUniversalSchema.forEach((param) => {
        globalFallbackValues[param.name] = getSetting(param.name);
    });

    const universalContainer = modal.query('#devUniversalContainer');
    const driverContainer = modal.query('#devDriverContainer');

    renderSchemaFields(universalContainer, resolvedUniversalSchema, entry.universal || {}, {
        prefix: 'dev_uni_',
        allowEmptyInherit: true,
        globalFallbackValues,
    });

    renderSchemaFields(driverContainer, driverSchema, entry.driver_config || {}, {
        prefix: 'dev_drv_',
        allowEmptyInherit: false,
    });

    modal.query('#devConfigSaveBtn')?.addEventListener('click', () => {
        const nextUniversal = collectSchemaValues(universalContainer, resolvedUniversalSchema, {
            prefix: 'dev_uni_',
            omitEmpty: true,
            booleanAsString: false,
        });

        const nextDriver = collectSchemaValues(driverContainer, driverSchema, {
            prefix: 'dev_drv_',
            omitEmpty: true,
            booleanAsString: true,
        });

        saveDeviceConfigEntry(device_uuid, {
            ignored: entry.ignored,
            universal: nextUniversal,
            driver_config: nextDriver,
        });

        showToast(t('config_saved', '设备配置已保存'));
        modal.close();
        if (typeof onSaved === 'function') {
            onSaved();
        }
    });
}

/**
 * Dynamically mount and self-destroy a read-only modal displaying active RemoteConfig.
 */
export async function openRemoteConfigModal(device, remoteConfig = {}, { universalSchema = null } = {}) {
    const displayTitle = device.displayName || device.device_id || device.device_uuid;

    const modal = createModalShell({
        id: 'remoteConfigModal',
        title: `${t('remote_config', '远端配置')} - ${displayTitle}`,
        subtitle: t('remote_config_readonly_subtitle', '当前活动串流的远端配置参数（只读）'),
        icon: 'cast_connected',
        maxWidthClass: 'max-w-lg',
        bodyHtml: `
            <section class="space-y-2.5">
                <div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider text-gray-400 px-1">
                    <span class="material-symbols-rounded text-base text-[var(--md-sys-color-primary)]">tune</span>
                    <span>${escapeHtml(t('universal_stream_defaults', '通用串流参数'))}</span>
                </div>
                <div id="remoteUniversalContainer" class="min-h-[4rem] flex flex-col justify-center">
                    <div class="flex justify-center py-6"><div class="spinner"></div></div>
                </div>
            </section>

            <section class="space-y-2.5">
                <div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider text-gray-400 px-1">
                    <span class="material-symbols-rounded text-base text-[var(--md-sys-color-primary)]">developer_board</span>
                    <span>${escapeHtml(t('driver_specific_settings', '设备驱动参数'))}</span>
                </div>
                <div id="remoteDriverContainer" class="min-h-[4rem] flex flex-col justify-center">
                    <div class="flex justify-center py-6"><div class="spinner"></div></div>
                </div>
            </section>

            <section id="remoteExtraWrapper" class="hidden space-y-2.5">
                <div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider text-gray-400 px-1">
                    <span class="material-symbols-rounded text-base text-[var(--md-sys-color-primary)]">info</span>
                    <span>${escapeHtml(t('runtime_parameters', '运行时参数'))}</span>
                </div>
                <div id="remoteExtraContainer"></div>
            </section>
        `,
        footerHtml: `
            <button type="button" data-dialog-close
                class="px-6 py-2.5 rounded-full bg-[#2a2b2c] hover:bg-[#333] text-[#e3e3e3] font-medium text-sm transition-colors">
                ${escapeHtml(t('close', '关闭'))}
            </button>
        `,
    });

    const [resolvedUniversalSchema, driverSchema] = await Promise.all([
        universalSchema ? Promise.resolve(universalSchema) : ensureUniversalSettingsInitialized(),
        fetchDriverSchema(device.device_type, device.device_id),
    ]);

    const configMap = remoteConfig && typeof remoteConfig === 'object' ? remoteConfig : {};
    // delete configMap['protocol_transport'];
    // console.log('configMap before deletion:', configMap);
    // console.log('universalSchema:', resolvedUniversalSchema)
    const newUniversalSchema = resolvedUniversalSchema.filter(item => item.name == 'video_codec' || item.name == 'audio_codec');
    
    renderSchemaFields(modal.query('#remoteUniversalContainer'), newUniversalSchema, configMap, {
        prefix: 'rem_uni_',
        readOnly: true,
    });

    renderSchemaFields(modal.query('#remoteDriverContainer'), driverSchema, configMap, {
        prefix: 'rem_drv_',
        readOnly: true,
    });

    // Render any additional keys returned by RemoteConfig that aren't defined in universal/driver schemas
    const knownKeys = new Set([
        ...normalizeSchema(resolvedUniversalSchema).map((p) => p.name),
        ...normalizeSchema(driverSchema).map((p) => p.name),
    ]);
    const extraEntries = Object.entries(configMap).filter(([k]) => !knownKeys.has(k));

    if (extraEntries.length > 0) {
        modal.query('#remoteExtraWrapper')?.classList.remove('hidden');
        const extraSchema = extraEntries.map(([key, val]) => ({
            name: key,
            type: val === 'true' || val === 'false' || typeof val === 'boolean' ? 'boolean' : 'string',
        }));
        renderSchemaFields(modal.query('#remoteExtraContainer'), extraSchema, configMap, {
            prefix: 'rem_ext_',
            readOnly: true,
        });
    }
}