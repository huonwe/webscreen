// /static/console-modules/global-settings-ui.js
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

const GLOBAL_STORAGE_KEY = 'global_settings';
let cachedUniversalSchema = null;

/**
 * Read all top-level entries from global_settings after ensuring initSettings() ran.
 */
function readAllGlobalSettingsSnapshot() {
    // Touching any key triggers initSettings() inside global-settings.js
    getSetting('_device_config');
    try {
        const raw = localStorage.getItem(GLOBAL_STORAGE_KEY);
        return raw ? JSON.parse(raw) : {};
    } catch (err) {
        console.error('Failed to parse global_settings snapshot:', err);
        return {};
    }
}

/**
 * Fetch universal config schema from backend and append any missing universal keys
 * into global_settings so defaults are always persisted.
 */
export async function ensureUniversalSettingsInitialized(forceRefresh = false) {
    if (!cachedUniversalSchema || forceRefresh) {
        try {
            const response = await fetch('/api/device/configDescription?device_type=universal');
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}`);
            }
            const rawSchema = await response.json();
            cachedUniversalSchema = normalizeSchema(rawSchema);
        } catch (err) {
            console.error('Failed to fetch universal config description:', err);
            cachedUniversalSchema = cachedUniversalSchema || [];
        }
    }

    for (const param of cachedUniversalSchema) {
        const key = param.name;
        if (!key || key.startsWith('_')) continue;

        if (getSetting(key) === undefined) {
            const fallbackDefault = param.default !== undefined
                ? param.default
                : (param.type === 'boolean' ? false : '');
            updateSetting(key, fallbackDefault);
        }
    }

    return cachedUniversalSchema;
}

/**
 * Format raw snake_case setting keys into readable labels when i18n key is missing.
 */
function humanizeSettingKey(key) {
    return key
        .replace(/_/g, ' ')
        .replace(/\b\w/g, (ch) => ch.toUpperCase());
}

/**
 * Render non-universal, non-underscore keys from global_settings dynamically.
 */
function renderGeneralSettingsFields(containerEl, entries) {
    if (!containerEl) return;
    containerEl.innerHTML = '';

    if (!entries.length) {
        containerEl.innerHTML = `
            <p class="text-xs text-gray-500 text-center py-3">
                ${escapeHtml(t('no_configurable_options', '暂无可配置项'))}
            </p>
        `;
        return;
    }

    const panel = document.createElement('div');
    panel.className = 'bg-[#2a2b2c] p-4 rounded-2xl space-y-4';

    for (const [key, value] of entries) {
        const fieldDiv = document.createElement('div');
        fieldDiv.className = 'flex flex-col gap-1.5';
        const inputId = `gs_general_${key}`;
        const labelText = t(key, humanizeSettingKey(key));

        if (typeof value === 'boolean') {
            const row = document.createElement('div');
            row.className = 'flex items-center justify-between py-1';

            const label = document.createElement('label');
            label.className = 'text-sm font-medium text-gray-200 cursor-pointer select-none ml-1';
            label.setAttribute('for', inputId);
            label.textContent = labelText;

            const checkbox = document.createElement('input');
            checkbox.type = 'checkbox';
            checkbox.id = inputId;
            checkbox.className = 'md-switch';
            checkbox.dataset.settingKey = key;
            checkbox.dataset.valueType = 'boolean';
            checkbox.checked = value;

            row.appendChild(label);
            row.appendChild(checkbox);
            fieldDiv.appendChild(row);
        } else {
            const label = document.createElement('label');
            label.className = 'block text-xs font-medium text-gray-300 ml-1';
            label.setAttribute('for', inputId);
            label.textContent = labelText;
            fieldDiv.appendChild(label);

            const strVal = value !== undefined && value !== null ? String(value) : '';
            const isMultiline = key.includes('prompt') || strVal.includes('\n') || strVal.length > 100;
            const isSecret = /secret|token|password|api_key/i.test(key);

            if (isMultiline) {
                const textarea = document.createElement('textarea');
                textarea.id = inputId;
                textarea.rows = 3;
                textarea.className = 'md-input w-full px-3 py-2 rounded-xl text-white text-sm font-mono leading-relaxed';
                textarea.dataset.settingKey = key;
                textarea.dataset.valueType = 'string';
                textarea.value = strVal;
                fieldDiv.appendChild(textarea);
            } else if (isSecret) {
                const wrapper = document.createElement('div');
                wrapper.className = 'relative flex items-center';

                const input = document.createElement('input');
                input.type = 'password';
                input.id = inputId;
                input.className = 'md-input w-full pl-3 pr-10 py-2 rounded-xl text-white text-sm';
                input.dataset.settingKey = key;
                input.dataset.valueType = 'string';
                input.value = strVal;

                const toggleBtn = document.createElement('button');
                toggleBtn.type = 'button';
                toggleBtn.className = 'absolute right-2.5 p-1 rounded-full text-gray-400 hover:text-white transition-colors';
                toggleBtn.innerHTML = '<span class="material-symbols-rounded text-base">visibility</span>';
                toggleBtn.addEventListener('click', () => {
                    const hidden = input.type === 'password';
                    input.type = hidden ? 'text' : 'password';
                    toggleBtn.innerHTML = `<span class="material-symbols-rounded text-base">${hidden ? 'visibility_off' : 'visibility'}</span>`;
                });

                wrapper.appendChild(input);
                wrapper.appendChild(toggleBtn);
                fieldDiv.appendChild(wrapper);
            } else {
                const input = document.createElement('input');
                const isNumber = typeof value === 'number';
                input.type = isNumber ? 'number' : 'text';
                input.id = inputId;
                input.className = 'md-input w-full px-3 py-2 rounded-xl text-white text-sm';
                input.dataset.settingKey = key;
                input.dataset.valueType = isNumber ? 'number' : 'string';
                input.value = strVal;
                fieldDiv.appendChild(input);
            }
        }

        panel.appendChild(fieldDiv);
    }

    containerEl.appendChild(panel);
}

/**
 * Dynamically build, open, and self-destroy the Global Settings modal.
 */
export async function openGlobalSettingsModal({ onSaved } = {}) {
    const modal = createModalShell({
        id: 'globalSettingsModal',
        title: t('global_settings', '全局设置'),
        subtitle: t('global_settings_subtitle', '配置默认通用串流参数与系统全局选项'),
        icon: 'tune',
        maxWidthClass: 'max-w-xl',
        bodyHtml: `
            <section class="space-y-2.5">
                <div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider text-gray-400 px-1">
                    <span class="material-symbols-rounded text-base text-[var(--md-sys-color-primary)]">settings_input_antenna</span>
                    <span>${escapeHtml(t('universal_stream_defaults', '通用串流默认参数'))}</span>
                </div>
                <div id="gsUniversalSection" class="min-h-[4rem] flex flex-col justify-center">
                    <div class="flex justify-center py-6"><div class="spinner"></div></div>
                </div>
            </section>

            <section class="space-y-2.5">
                <div class="flex items-center gap-2 text-xs font-medium uppercase tracking-wider text-gray-400 px-1">
                    <span class="material-symbols-rounded text-base text-[var(--md-sys-color-primary)]">translate</span>
                    <span>${escapeHtml(t('general_and_translation_settings', '全局与翻译选项'))}</span>
                </div>
                <div id="gsGeneralSection" class="min-h-[4rem] flex flex-col justify-center"></div>
            </section>
        `,
        footerHtml: `
            <button type="button" data-dialog-close
                class="px-5 py-2.5 rounded-full text-[var(--md-sys-color-primary)] hover:bg-white/5 font-medium text-sm transition-colors">
                ${escapeHtml(t('cancel', '取消'))}
            </button>
            <button type="button" id="gsSaveBtn"
                class="px-6 py-2.5 rounded-full bg-[var(--md-sys-color-primary)] text-[var(--md-sys-color-on-primary)] font-medium text-sm shadow-sm hover:opacity-90 transition-opacity">
                ${escapeHtml(t('save_settings', '保存设置'))}
            </button>
        `,
    });

    const universalSchema = await ensureUniversalSettingsInitialized();
    const snapshot = readAllGlobalSettingsSnapshot();

    // Filter out underscore-prefixed internal keys (e.g. _device_config)
    const visibleKeys = Object.keys(snapshot).filter((k) => !k.startsWith('_'));
    const universalKeySet = new Set(universalSchema.map((p) => p.name));

    const currentUniversalValues = {};
    universalSchema.forEach((param) => {
        currentUniversalValues[param.name] = getSetting(param.name);
    });

    const generalEntries = visibleKeys
        .filter((key) => !universalKeySet.has(key))
        .map((key) => [key, getSetting(key)]);

    const universalSectionEl = modal.query('#gsUniversalSection');
    const generalSectionEl = modal.query('#gsGeneralSection');

    renderSchemaFields(universalSectionEl, universalSchema, currentUniversalValues, {
        prefix: 'gs_uni_',
        allowEmptyInherit: false,
    });

    renderGeneralSettingsFields(generalSectionEl, generalEntries);

    modal.query('#gsSaveBtn')?.addEventListener('click', () => {
        // 1. Persist dynamic universal settings
        const updatedUniversal = collectSchemaValues(universalSectionEl, universalSchema, {
            prefix: 'gs_uni_',
            omitEmpty: false,
            booleanAsString: false,
        });
        Object.entries(updatedUniversal).forEach(([key, val]) => {
            updateSetting(key, val);
        });

        // 2. Persist general non-underscore settings
        generalSectionEl?.querySelectorAll('[data-setting-key]').forEach((el) => {
            const key = el.dataset.settingKey;
            const valueType = el.dataset.valueType;
            if (!key || key.startsWith('_')) return;

            if (valueType === 'boolean') {
                updateSetting(key, Boolean(el.checked));
            } else if (valueType === 'number') {
                const raw = el.value.trim();
                updateSetting(key, raw === '' ? 0 : Number(raw));
            } else {
                updateSetting(key, el.value);
            }
        });

        showToast(t('config_saved', '全局设置已保存'));
        modal.close();
        if (typeof onSaved === 'function') {
            onSaved();
        }
    });
}