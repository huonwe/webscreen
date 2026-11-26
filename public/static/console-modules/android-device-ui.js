// /static/console-modules/android-device-ui.js
import {
    t,
    escapeHtml,
    showToast,
    createModalShell,
} from './ui-dialogs.js';

/**
 * Dynamically mount and self-destroy the Connect Device modal.
 */
export function openConnectDeviceModal({ initialIp = '', onConnected } = {}) {
    const modal = createModalShell({
        id: 'connectModal',
        title: t('connect_new_device', '连接新设备'),
        icon: 'add_link',
        maxWidthClass: 'max-w-sm',
        bodyHtml: `
            <div class="space-y-4">
                <div>
                    <label for="dynamicConnectIP"
                        class="block text-xs font-medium text-[var(--md-sys-color-primary)] mb-1 ml-1">
                        ${escapeHtml(t('ip_address', 'IP 地址'))}
                    </label>
                    <input type="text" id="dynamicConnectIP"
                        class="md-input w-full px-4 py-3 rounded-xl text-white placeholder-gray-500"
                        placeholder="192.168.0.x">
                </div>
                <div>
                    <label for="dynamicConnectPort"
                        class="block text-xs font-medium text-gray-400 mb-1 ml-1">
                        ${escapeHtml(t('port_default', '端口 (默认 5555)'))}
                    </label>
                    <input type="text" id="dynamicConnectPort"
                        class="md-input w-full px-4 py-3 rounded-xl text-white placeholder-gray-500"
                        placeholder="5555" value="5555">
                </div>
            </div>
        `,
        footerHtml: `
            <button type="button" data-dialog-close
                class="px-5 py-2.5 rounded-full text-[var(--md-sys-color-primary)] hover:bg-white/5 font-medium text-sm transition-colors">
                ${escapeHtml(t('cancel', '取消'))}
            </button>
            <button type="button" id="dynamicConnectSubmitBtn"
                class="px-6 py-2.5 rounded-full bg-[var(--md-sys-color-primary)] text-[var(--md-sys-color-on-primary)] font-medium text-sm shadow-sm hover:opacity-90 transition-opacity">
                ${escapeHtml(t('connect', '连接'))}
            </button>
        `,
    });

    const ipInput = modal.query('#dynamicConnectIP');
    const portInput = modal.query('#dynamicConnectPort');
    const submitBtn = modal.query('#dynamicConnectSubmitBtn');

    if (ipInput) {
        ipInput.value = initialIp || '';
        ipInput.focus();
    }

    const submit = async () => {
        const device_ip = (ipInput?.value || '').trim();
        const device_port = (portInput?.value || '').trim() || '5555';

        if (!device_ip) {
            showToast(t('enter_ip', '请输入 IP 地址'), 'error');
            ipInput?.focus();
            return;
        }

        if (submitBtn) submitBtn.disabled = true;
        try {
            const response = await fetch('/api/device/connect', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ device_type: 'android', device_ip, device_port }),
            });
            const data = await response.json().catch(() => ({}));
            if (!response.ok) {
                throw new Error(data.error || t('connection_failed', '连接设备失败'));
            }

            showToast(t('connected_success', '设备连接成功'));
            modal.close();
            if (typeof onConnected === 'function') {
                onConnected();
            }
        } catch (err) {
            showToast(err.message || t('connection_failed', '连接设备失败'), 'error');
            if (submitBtn) submitBtn.disabled = false;
        }
    };

    submitBtn?.addEventListener('click', submit);
    [ipInput, portInput].forEach((el) => {
        el?.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') {
                e.preventDefault();
                submit();
            }
        });
    });
}

/**
 * Dynamically mount and self-destroy the Android Wireless Pairing modal.
 * On successful pairing, transitions smoothly into the Connect Device modal.
 */
export function openAndroidPairModal({ onPaired } = {}) {
    const modal = createModalShell({
        id: 'pairModal',
        title: t('wireless_pair_title', '无线配对'),
        icon: 'qr_code_scanner',
        maxWidthClass: 'max-w-sm',
        bodyHtml: `
            <div class="space-y-4">
                <div>
                    <label for="dynamicPairIP"
                        class="block text-xs font-medium text-[var(--md-sys-color-primary)] mb-1 ml-1">
                        ${escapeHtml(t('ip_address', 'IP 地址'))}
                    </label>
                    <input type="text" id="dynamicPairIP"
                        class="md-input w-full px-4 py-3 rounded-xl text-white placeholder-gray-500"
                        placeholder="192.168.0.x">
                </div>
                <div>
                    <label for="dynamicPairPort"
                        class="block text-xs font-medium text-gray-400 mb-1 ml-1">
                        ${escapeHtml(t('port_wireless', '端口 (无线调试端口)'))}
                    </label>
                    <input type="text" id="dynamicPairPort"
                        class="md-input w-full px-4 py-3 rounded-xl text-white placeholder-gray-500"
                        placeholder="e.g. 38123">
                </div>
                <div>
                    <label for="dynamicPairCode"
                        class="block text-xs font-medium text-gray-400 mb-1 ml-1">
                        ${escapeHtml(t('pair_code', '配对码 (6位)'))}
                    </label>
                    <input type="text" inputmode="numeric" id="dynamicPairCode"
                        class="md-input w-full px-4 py-3 rounded-xl text-white placeholder-gray-500"
                        placeholder="123456">
                </div>
            </div>
        `,
        footerHtml: `
            <button type="button" data-dialog-close
                class="px-5 py-2.5 rounded-full text-[var(--md-sys-color-primary)] hover:bg-white/5 font-medium text-sm transition-colors">
                ${escapeHtml(t('cancel', '取消'))}
            </button>
            <button type="button" id="dynamicPairSubmitBtn"
                class="px-6 py-2.5 rounded-full bg-[var(--md-sys-color-primary)] text-[var(--md-sys-color-on-primary)] font-medium text-sm shadow-sm hover:opacity-90 transition-opacity">
                ${escapeHtml(t('pair', '配对'))}
            </button>
        `,
    });

    const ipInput = modal.query('#dynamicPairIP');
    const portInput = modal.query('#dynamicPairPort');
    const codeInput = modal.query('#dynamicPairCode');
    const submitBtn = modal.query('#dynamicPairSubmitBtn');

    ipInput?.focus();

    const submit = async () => {
        const device_ip = (ipInput?.value || '').trim();
        const device_port = (portInput?.value || '').trim();
        const code = (codeInput?.value || '').trim();

        if (!device_ip || !device_port || !code) {
            showToast(t('fill_all_fields', '请填写所有必填项'), 'error');
            return;
        }

        if (submitBtn) submitBtn.disabled = true;
        try {
            const response = await fetch('/api/device/pair', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ device_type: 'android', device_ip, device_port, code }),
            });
            const data = await response.json().catch(() => ({}));
            if (!response.ok) {
                throw new Error(data.error || t('pair_failed', '无线配对失败'));
            }

            showToast(t('pair_success', '无线配对成功'));
            modal.close();
            openConnectDeviceModal({
                initialIp: device_ip,
                onConnected: onPaired,
            });
        } catch (err) {
            showToast(err.message || t('pair_failed', '无线配对失败'), 'error');
            if (submitBtn) submitBtn.disabled = false;
        }
    };

    submitBtn?.addEventListener('click', submit);
    [ipInput, portInput, codeInput].forEach((el) => {
        el?.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') {
                e.preventDefault();
                submit();
            }
        });
    });
}