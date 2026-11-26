// /static/console-modules/gamestream-pair-ui.js
import {
    t,
    escapeHtml,
    showToast,
    createModalShell,
} from './ui-dialogs.js';

/**
 * Dynamically mount, discover LAN GameStream/Sunshine hosts, pair via PIN challenge,
 * and remove the dialog DOM once finished or closed.
 */
export function openGameStreamPairModal({ onPaired } = {}) {
    let discoveredDevices = [];
    let selectedDevice = null;

    const modal = createModalShell({
        id: 'gamestreamPairModal',
        title: t('gamestream_pair_title', 'GameStream 设备配对'),
        icon: 'sports_esports',
        maxWidthClass: 'max-w-md',
        headerActionsHtml: `
            <button type="button" id="gsRefreshDiscoverBtn"
                class="p-1.5 rounded-full hover:bg-white/10 transition-colors"
                title="${escapeHtml(t('refresh', '刷新'))}">
                <span class="material-symbols-rounded text-gray-400">refresh</span>
            </button>
        `,
        bodyHtml: `
            <div id="gsDiscoveredList" class="space-y-2.5 max-h-60 overflow-y-auto"></div>

            <div id="gsPairChallengeForm" class="hidden space-y-4 pt-4 border-t border-[#333]">
                <div class="flex items-center justify-between text-xs text-gray-400 px-1">
                    <span id="gsSelectedDeviceLabel" class="font-medium text-[#e3e3e3]"></span>
                    <span id="gsSelectedDeviceAddr" class="font-mono"></span>
                </div>
                <div>
                    <label for="gsPairPinInput"
                        class="block text-xs font-medium text-[var(--md-sys-color-primary)] mb-1 ml-1">
                        ${escapeHtml(t('gamestream_pin', '配对 PIN 码 (4位)'))}
                    </label>
                    <input type="text" id="gsPairPinInput" maxlength="8"
                        class="md-input w-full px-4 py-3 rounded-xl text-white placeholder-gray-500"
                        placeholder="1234">
                </div>
                <div class="flex justify-end gap-3 pt-2">
                    <button type="button" data-dialog-close
                        class="px-5 py-2.5 rounded-full text-[var(--md-sys-color-primary)] hover:bg-white/5 font-medium text-sm transition-colors">
                        ${escapeHtml(t('cancel', '取消'))}
                    </button>
                    <button type="button" id="gsPairSubmitBtn"
                        class="px-6 py-2.5 rounded-full bg-[var(--md-sys-color-primary)] text-[var(--md-sys-color-on-primary)] font-medium text-sm shadow-sm hover:opacity-90 transition-opacity">
                        ${escapeHtml(t('gamestream_challenge', '发起配对 Challenge'))}
                    </button>
                </div>
            </div>
        `,
    });

    const listEl = modal.query('#gsDiscoveredList');
    const formEl = modal.query('#gsPairChallengeForm');
    const selectedLabelEl = modal.query('#gsSelectedDeviceLabel');
    const selectedAddrEl = modal.query('#gsSelectedDeviceAddr');
    const pinInput = modal.query('#gsPairPinInput');
    const submitBtn = modal.query('#gsPairSubmitBtn');

    const renderList = () => {
        if (!listEl) return;

        if (!discoveredDevices.length) {
            listEl.innerHTML = `
                <div class="text-center py-8 text-sm text-gray-400 bg-[#28292a]/50 rounded-2xl border border-dashed border-gray-700">
                    ${escapeHtml(t('gamestream_no_devices', '未发现网络中的 GameStream / Sunshine 服务端'))}
                </div>
            `;
            return;
        }

        listEl.innerHTML = '';
        discoveredDevices.forEach((dev, index) => {
            const isSelected = selectedDevice && selectedDevice.ip === dev.ip;
            const badgeClass = dev.paired ? 'gs-badge-paired' : 'gs-badge-unpaired';
            const badgeText = dev.paired
                ? t('gamestream_paired_badge', '已配对')
                : t('gamestream_unpaired_badge', '未配对');

            const item = document.createElement('button');
            item.type = 'button';
            item.className = `gs-device-item w-full text-left ${isSelected ? 'selected' : ''}`;
            item.innerHTML = `
                <div class="min-w-0 pr-3">
                    <div class="font-medium text-sm text-[#e3e3e3] truncate">${escapeHtml(dev.device_id || dev.ip)}</div>
                    <div class="text-xs text-gray-400 font-mono">${escapeHtml(`${dev.ip}:${dev.port || 47989}`)}</div>
                </div>
                <span class="px-2.5 py-0.5 rounded-full text-xs font-medium shrink-0 ${badgeClass}">${escapeHtml(badgeText)}</span>
            `;
            item.addEventListener('click', () => selectDevice(index));
            listEl.appendChild(item);
        });
    };

    const selectDevice = (index) => {
        const dev = discoveredDevices[index];
        if (!dev) return;
        selectedDevice = dev;
        renderList();

        if (selectedLabelEl) selectedLabelEl.textContent = dev.device_id || dev.ip;
        if (selectedAddrEl) selectedAddrEl.textContent = `${dev.ip}:${dev.port || 47989}`;
        formEl?.classList.remove('hidden');
        pinInput?.focus();
    };

    const discover = async () => {
        selectedDevice = null;
        formEl?.classList.add('hidden');
        if (pinInput) pinInput.value = '';

        if (listEl) {
            listEl.innerHTML = `
                <div class="flex flex-col items-center justify-center py-8 text-gray-400">
                    <div class="spinner mb-3"></div>
                    <p class="text-sm">${escapeHtml(t('gamestream_scanning', '正在发现局域网 GameStream 设备...'))}</p>
                </div>
            `;
        }

        try {
            const resp = await fetch('/api/gamestream/discover');
            const data = await resp.json().catch(() => ({}));
            if (!resp.ok) {
                throw new Error(data.error || t('call_api_failed', '发现设备失败'));
            }
            discoveredDevices = Array.isArray(data.devices) ? data.devices : [];
            renderList();
        } catch (err) {
            console.error('GameStream discover failed:', err);
            if (listEl) {
                listEl.innerHTML = `
                    <div class="text-center py-6 text-sm text-red-300">
                        ${escapeHtml(err.message || t('call_api_failed', '发现设备失败'))}
                    </div>
                `;
            }
        }
    };

    const submitPair = async () => {
        if (!selectedDevice) return;
        const pin = (pinInput?.value || '').trim();
        if (!pin) {
            showToast(t('fill_all_fields', '请输入配对 PIN 码'), 'error');
            pinInput?.focus();
            return;
        }

        if (submitBtn) submitBtn.disabled = true;
        try {
            const resp = await fetch('/api/gamestream/pair', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    device_id: selectedDevice.device_id,
                    ip: selectedDevice.ip,
                    port: selectedDevice.port || 47989,
                    pin,
                }),
            });
            const data = await resp.json().catch(() => ({}));
            if (!resp.ok) {
                throw new Error(data.error || t('pair_failed', 'GameStream 配对失败'));
            }

            showToast(t('pair_success', 'GameStream 配对成功'));
            modal.close();
            if (typeof onPaired === 'function') {
                onPaired();
            }
        } catch (err) {
            showToast(err.message || t('pair_failed', 'GameStream 配对失败'), 'error');
            if (submitBtn) submitBtn.disabled = false;
        }
    };

    modal.query('#gsRefreshDiscoverBtn')?.addEventListener('click', discover);
    submitBtn?.addEventListener('click', submitPair);
    pinInput?.addEventListener('keydown', (e) => {
        if (e.key === 'Enter') {
            e.preventDefault();
            submitPair();
        }
    });

    discover();
}