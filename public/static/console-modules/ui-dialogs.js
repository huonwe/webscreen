// /static/console-modules/ui-dialogs.js

import { i18n } from '../i18n.js';

export function t(key, fallback = '', params = null) {
    const translated = i18n.t(key, params || {});
    if (translated && translated !== key) {
        return translated;
    }

    let text = fallback || key;
    if (params && typeof text === 'string') {
        Object.entries(params).forEach(([k, v]) => {
            text = text.replace(new RegExp(`\\{${k}\\}`, 'g'), String(v));
        });
    }
    return text;
}

/**
 * Escape HTML special characters for safe template interpolation.
 */
export function escapeHtml(value) {
    return String(value ?? '')
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
}

/**
 * Display a toast notification above any active <dialog> Top Layer.
 */
export function showToast(message, type = 'success') {
    let container = document.getElementById('toast-container');
    if (!container) {
        container = document.createElement('div');
        container.id = 'toast-container';
        document.body.appendChild(container);
    }

    const activeDialog = document.querySelector('dialog[open]');
    const host = activeDialog || document.body;
    if (container.parentElement !== host) {
        host.appendChild(container);
    }

    const toast = document.createElement('div');
    toast.className = `toast ${type === 'error' ? 'error' : ''}`;
    const iconName = type === 'error' ? 'error' : (type === 'info' ? 'info' : 'check_circle');
    toast.innerHTML = `
        <span>${escapeHtml(message)}</span>
        <span class="material-symbols-rounded text-sm">${iconName}</span>
    `;
    container.appendChild(toast);

    setTimeout(() => {
        toast.style.animation = 'toastOut 0.3s forwards';
        setTimeout(() => toast.remove(), 300);
    }, 3000);
}

/**
 * Dynamically mount a <dialog> modal and automatically remove it from DOM on close.
 */
export function createModalShell({
    id,
    title = '',
    subtitle = '',
    icon = '',
    maxWidthClass = 'max-w-md',
    headerActionsHtml = '',
    bodyHtml = '',
    footerHtml = '',
    onClose = null,
}) {
    if (id) {
        const existing = document.getElementById(id);
        if (existing) existing.remove();
    }

    const dialog = document.createElement('dialog');
    if (id) dialog.id = id;
    dialog.className = 'bg-transparent p-4 m-auto outline-none backdrop:bg-black/60';

    const iconHtml = icon
        ? `<span class="material-symbols-rounded text-2xl text-[var(--md-sys-color-primary)] shrink-0">${escapeHtml(icon)}</span>`
        : '';
    const subtitleHtml = subtitle
        ? `<p data-modal-subtitle class="text-xs text-gray-400 mt-0.5">${escapeHtml(subtitle)}</p>`
        : '';

    dialog.innerHTML = `
        <div class="dialog-panel w-full ${maxWidthClass} bg-[#1e1f20] rounded-[28px] p-6 shadow-2xl border border-[#333] max-h-[85vh] flex flex-col text-[#e3e3e3]">
            <div class="flex justify-between items-start gap-3 mb-5 shrink-0">
                <div class="flex items-center gap-2.5 min-w-0">
                    ${iconHtml}
                    <div class="min-w-0">
                        <h3 data-modal-title class="text-xl font-medium truncate">${escapeHtml(title)}</h3>
                        ${subtitleHtml}
                    </div>
                </div>
                <div class="flex items-center gap-1 shrink-0">
                    ${headerActionsHtml}
                    <button type="button" data-dialog-close
                        class="p-1.5 rounded-full hover:bg-white/10 transition-colors"
                        title="${escapeHtml(t('cancel', '关闭'))}">
                        <span class="material-symbols-rounded text-gray-400">close</span>
                    </button>
                </div>
            </div>

            <div data-modal-body class="overflow-y-auto pr-1 flex-1 space-y-5">
                ${bodyHtml}
            </div>

            ${footerHtml ? `
                <div data-modal-footer class="flex justify-end gap-3 mt-6 pt-4 border-t border-[#333] shrink-0">
                    ${footerHtml}
                </div>
            ` : ''}
        </div>
    `;

    let closed = false;
    const close = () => {
        if (closed) return;
        closed = true;

        const toastContainer = document.getElementById('toast-container');
        if (toastContainer && dialog.contains(toastContainer)) {
            document.body.appendChild(toastContainer);
        }

        if (typeof onClose === 'function') {
            try {
                onClose();
            } catch (err) {
                console.error('Modal onClose hook error:', err);
            }
        }

        if (dialog.open) {
            dialog.close();
        }
        dialog.remove();
    };

    dialog.querySelectorAll('[data-dialog-close]').forEach((btn) => {
        btn.addEventListener('click', close);
    });

    // Close when clicking outside dialog-panel (on backdrop)
    // dialog.addEventListener('click', (event) => {
    //     if (event.target === dialog) {
    //         close();
    //     }
    // });

    // Intercept Escape key so cleanup runs cleanly
    dialog.addEventListener('cancel', (event) => {
        event.preventDefault();
        close();
    });

    document.body.appendChild(dialog);
    dialog.showModal();

    return {
        dialog,
        close,
        bodyEl: dialog.querySelector('[data-modal-body]'),
        footerEl: dialog.querySelector('[data-modal-footer]'),
        query: (selector) => dialog.querySelector(selector),
        queryAll: (selector) => dialog.querySelectorAll(selector),
    };
}

/**
 * Dynamic confirmation modal that resolves to true/false and self-destructs.
 */
export function openConfirmDialog({
    title,
    message,
    confirmText = t('confirm', '确定'),
    cancelText = t('cancel', '取消'),
    danger = false,
}) {
    return new Promise((resolve) => {
        let settled = false;
        const finish = (result) => {
            if (settled) return;
            settled = true;
            modal.close();
            resolve(result);
        };

        const confirmBtnClass = danger
            ? 'px-6 py-2 rounded-full bg-red-500/90 text-white font-medium text-sm shadow-sm hover:bg-red-500 transition-colors'
            : 'px-6 py-2 rounded-full bg-[var(--md-sys-color-primary)] text-[var(--md-sys-color-on-primary)] font-medium text-sm shadow-sm hover:opacity-90 transition-opacity';

        const modal = createModalShell({
            title,
            maxWidthClass: 'max-w-sm',
            bodyHtml: `<p class="text-sm text-gray-400 leading-relaxed">${escapeHtml(message)}</p>`,
            footerHtml: `
                <button type="button" data-confirm-cancel
                    class="px-5 py-2 rounded-full text-gray-300 hover:bg-white/5 font-medium text-sm transition-colors">
                    ${escapeHtml(cancelText)}
                </button>
                <button type="button" data-confirm-ok class="${confirmBtnClass}">
                    ${escapeHtml(confirmText)}
                </button>
            `,
            onClose: () => {
                if (!settled) {
                    settled = true;
                    resolve(false);
                }
            },
        });

        modal.query('[data-confirm-cancel]')?.addEventListener('click', () => finish(false));
        modal.query('[data-confirm-ok]')?.addEventListener('click', () => finish(true));
    });
}

/**
 * Dynamic dialog for updating a device's displayName via POST /api/device/updateDisplayName.
 * Resolves to the new displayName string if updated, or null if cancelled.
 */
export function openRenameDeviceDialog({ device_uuid, currentName = '', deviceId = '' }) {
    return new Promise((resolve) => {
        let settled = false;
        const finish = (result) => {
            if (settled) return;
            settled = true;
            modal.close();
            resolve(result);
        };

        const modal = createModalShell({
            title: t('rename_device', '修改设备显示名称'),
            subtitle: deviceId ? `ID: ${deviceId}` : device_uuid,
            icon: 'edit',
            maxWidthClass: 'max-w-sm',
            bodyHtml: `
                <div class="space-y-2">
                    <label for="dynamicRenameInput"
                        class="block text-xs font-medium text-[var(--md-sys-color-primary)] ml-1">
                        ${escapeHtml(t('display_name', '显示名称'))}
                    </label>
                    <input type="text" id="dynamicRenameInput"
                        class="md-input w-full px-4 py-3 rounded-xl text-white placeholder-gray-500"
                        placeholder="${escapeHtml(t('enter_display_name', '输入设备显示名称'))}">
                </div>
            `,
            footerHtml: `
                <button type="button" data-dialog-close
                    class="px-5 py-2.5 rounded-full text-[var(--md-sys-color-primary)] hover:bg-white/5 font-medium text-sm transition-colors">
                    ${escapeHtml(t('cancel', '取消'))}
                </button>
                <button type="button" id="dynamicRenameSubmitBtn"
                    class="px-6 py-2.5 rounded-full bg-[var(--md-sys-color-primary)] text-[var(--md-sys-color-on-primary)] font-medium text-sm shadow-sm hover:opacity-90 transition-opacity">
                    ${escapeHtml(t('confirm', '确定'))}
                </button>
            `,
            onClose: () => {
                if (!settled) {
                    settled = true;
                    resolve(null);
                }
            },
        });

        const input = modal.query('#dynamicRenameInput');
        const submitBtn = modal.query('#dynamicRenameSubmitBtn');
        if (input) {
            input.value = currentName || '';
            input.focus();
            input.select();
        }

        const submit = async () => {
            const displayName = (input?.value || '').trim();
            if (!displayName) {
                showToast(t('enter_display_name', '请输入设备显示名称'), 'error');
                input?.focus();
                return;
            }

            if (submitBtn) submitBtn.disabled = true;
            try {
                const response = await fetch('/api/device/updateDisplayName', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ device_uuid, display_name: displayName }),
                });
                const data = await response.json().catch(() => ({}));
                if (!response.ok) {
                    throw new Error(data.error || t('update_failed', '更新显示名称失败'));
                }
                showToast(t('device_renamed', '设备名称已更新'));
                finish(displayName);
            } catch (err) {
                showToast(err.message || t('update_failed', '更新显示名称失败'), 'error');
                if (submitBtn) submitBtn.disabled = false;
            }
        };

        submitBtn?.addEventListener('click', submit);
        input?.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') {
                e.preventDefault();
                submit();
            }
        });
    });
}

/**
 * Normalize backend schema response into a uniform parameter descriptor array.
 */
export function normalizeSchema(schema) {
    if (Array.isArray(schema)) return schema;
    if (schema && typeof schema === 'object') {
        return Object.entries(schema).map(([name, meta]) => ({ name, ...(meta || {}) }));
    }
    return [];
}

/**
 * Render dynamic form controls from a configDescription schema into containerEl.
 *
 * @param {HTMLElement} containerEl
 * @param {Array<object>} schema
 * @param {object} currentValues
 * @param {object} options
 * @param {string} options.prefix - Input element ID prefix
 * @param {boolean} [options.allowEmptyInherit=false] - When true, empty values represent "inherit global"
 * @param {object} [options.globalFallbackValues={}] - Used to display inherited global values in placeholders
 * @param {boolean} [options.readOnly=false] - Render all controls in disabled read-only mode
 */
export function renderSchemaFields(
    containerEl,
    schema,
    currentValues = {},
    { prefix = 'cfg_', allowEmptyInherit = false, globalFallbackValues = {}, readOnly = false } = {}
) {
    if (!containerEl) return;
    containerEl.innerHTML = '';

    const items = normalizeSchema(schema);
    if (!items.length) {
        containerEl.innerHTML = `
            <p class="text-xs text-gray-500 text-center py-3">
                ${escapeHtml(t('no_configurable_options', '暂无可配置项'))}
            </p>
        `;
        return;
    }

    const panel = document.createElement('div');
    panel.className = 'bg-[#2a2b2c] p-4 rounded-2xl space-y-4';

    for (const param of items) {
        const key = param.name;
        const fieldDiv = document.createElement('div');
        fieldDiv.className = 'flex flex-col gap-1.5';

        const localizedLabel = t(key, key);
        const globalVal = globalFallbackValues[key];
        const rawVal = currentValues[key];
        const effectiveVal = (allowEmptyInherit && !readOnly)
            ? (rawVal !== undefined && rawVal !== null ? rawVal : '')
            : (rawVal !== undefined && rawVal !== null ? rawVal : (param.default ?? ''));

        const buildLabel = () => {
            const label = document.createElement('label');
            label.className = 'block text-xs font-medium text-gray-300 ml-1';
            label.setAttribute('for', `${prefix}${key}`);
            label.textContent = localizedLabel + (param.required && !allowEmptyInherit && !readOnly ? ' *' : '');
            if (param.description) label.title = param.description;
            return label;
        };

        const appendDescription = () => {
            if (!param.description) return;
            const desc = document.createElement('p');
            desc.className = 'text-[11px] text-gray-500 ml-1 leading-snug';
            desc.textContent = param.description;
            fieldDiv.appendChild(desc);
        };

        if (param.type === 'boolean') {
            if (allowEmptyInherit && !readOnly) {
                fieldDiv.appendChild(buildLabel());
                const select = document.createElement('select');
                select.id = `${prefix}${key}`;
                select.className = 'md-input w-full px-3 py-2 rounded-xl text-white text-sm';

                const globalHint = globalVal !== undefined && globalVal !== ''
                    ? ` (${String(globalVal)})`
                    : '';
                const options = [
                    { value: '', label: `${t('inherit_global', '继承全局设置')}${globalHint}` },
                    { value: 'true', label: t('enabled', '启用 (true)') },
                    { value: 'false', label: t('disabled', '禁用 (false)') },
                ];
                options.forEach((optItem) => {
                    const opt = document.createElement('option');
                    opt.value = optItem.value;
                    opt.textContent = optItem.label;
                    if (String(effectiveVal) === optItem.value) opt.selected = true;
                    select.appendChild(opt);
                });
                fieldDiv.appendChild(select);
                appendDescription();
            } else {
                const row = document.createElement('div');
                row.className = 'flex items-center justify-between py-1';

                const switchLabel = document.createElement('label');
                switchLabel.className = `text-sm font-medium text-gray-200 select-none ml-1 ${readOnly ? 'cursor-default' : 'cursor-pointer'}`;
                switchLabel.textContent = localizedLabel;
                switchLabel.setAttribute('for', `${prefix}${key}`);

                const checkbox = document.createElement('input');
                checkbox.type = 'checkbox';
                checkbox.className = `md-switch ${readOnly ? 'opacity-75 cursor-not-allowed' : ''}`;
                checkbox.id = `${prefix}${key}`;
                checkbox.checked = effectiveVal === true || effectiveVal === 'true';
                checkbox.disabled = readOnly;

                row.appendChild(switchLabel);
                row.appendChild(checkbox);
                fieldDiv.appendChild(row);
                appendDescription();
            }
        } else if (Array.isArray(param.options) && param.options.length > 0) {
            fieldDiv.appendChild(buildLabel());
            const select = document.createElement('select');
            select.id = `${prefix}${key}`;
            select.disabled = readOnly;
            select.className = `md-input w-full px-3 py-2 rounded-xl text-white text-sm ${readOnly ? 'opacity-80 cursor-not-allowed' : ''}`;

            if ((allowEmptyInherit && !readOnly) || !param.required || effectiveVal === '') {
                const emptyOpt = document.createElement('option');
                emptyOpt.value = '';
                if (readOnly) {
                    emptyOpt.textContent = '--';
                } else if (allowEmptyInherit) {
                    const globalHint = globalVal !== undefined && globalVal !== ''
                        ? ` (${globalVal})`
                        : '';
                    emptyOpt.textContent = `${t('inherit_global', '继承全局设置')}${globalHint}`;
                } else {
                    emptyOpt.textContent = t('select_auto_default', '-- 默认 / 自动 --');
                }
                select.appendChild(emptyOpt);
            }

            const optionValues = param.options.map(String);
            if (effectiveVal !== '' && !optionValues.includes(String(effectiveVal))) {
                optionValues.unshift(String(effectiveVal));
            }

            optionValues.forEach((optionVal) => {
                const opt = document.createElement('option');
                opt.value = optionVal;
                opt.textContent = optionVal;
                if (String(effectiveVal) === optionVal) opt.selected = true;
                select.appendChild(opt);
            });

            fieldDiv.appendChild(select);
            appendDescription();
        } else {
            fieldDiv.appendChild(buildLabel());
            const input = document.createElement('input');
            input.type = !readOnly && (param.type === 'integer' || param.type === 'number') ? 'number' : 'text';
            input.id = `${prefix}${key}`;
            input.disabled = readOnly;
            input.className = `md-input w-full px-3 py-2 rounded-xl text-white text-sm placeholder-gray-500 ${readOnly ? 'opacity-80 cursor-not-allowed' : ''}`;
            input.value = effectiveVal !== undefined && effectiveVal !== null && effectiveVal !== ''
                ? String(effectiveVal)
                : (readOnly ? '--' : '');

            if (!readOnly) {
                if (allowEmptyInherit) {
                    input.placeholder = globalVal !== undefined && globalVal !== ''
                        ? `${t('inherit_global', '继承全局')}: ${globalVal}`
                        : t('inherit_global', '留空继承全局设置');
                } else if (param.default !== undefined && param.default !== '') {
                    input.placeholder = String(param.default);
                }
            }

            fieldDiv.appendChild(input);
            appendDescription();
        }

        panel.appendChild(fieldDiv);
    }

    containerEl.appendChild(panel);
}
/**
 * Collect values back from inputs rendered by renderSchemaFields.
 * When omitEmpty is true, empty string inputs are omitted so caller can delete overrides.
 */
export function collectSchemaValues(
    rootEl,
    schema,
    { prefix = 'cfg_', omitEmpty = false, booleanAsString = false } = {}
) {
    const result = {};
    const items = normalizeSchema(schema);

    for (const param of items) {
        const key = param.name;
        const input = rootEl.querySelector(`#${CSS.escape(`${prefix}${key}`)}`);
        if (!input) continue;

        if (param.type === 'boolean') {
            if (input.type === 'checkbox') {
                result[key] = booleanAsString ? (input.checked ? 'true' : 'false') : input.checked;
            } else {
                // Tri-state select for allowEmptyInherit
                const val = input.value;
                if (val === '' && omitEmpty) continue;
                if (val !== '') {
                    result[key] = booleanAsString ? val : (val === 'true');
                }
            }
        } else {
            const val = input.value.trim();
            if (val === '' && omitEmpty) {
                continue;
            }
            if (val !== '' && (param.type === 'integer' || param.type === 'number')) {
                const num = Number(val);
                result[key] = Number.isNaN(num) ? val : num;
            } else {
                result[key] = val;
            }
        }
    }

    return result;
}