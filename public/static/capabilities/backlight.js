/**
 * 受控端黑屏（隐私保护）
 *
 * 只关背光、不锁屏：设备保持唤醒，触摸/按键/串流照常，屏幕对外是黑的。
 * 与键盘/鼠标一样通过 HTTP API 调服务端执行（服务端再按部署形态
 * 决定走本机 su 还是 adb -s），不依赖 UHID 或 DataChannel。
 */
import { addControlButton } from '../screen.js';
import { GetConfigValue } from '../connect-new.js';

const btn = addControlButton('backlightToggleBtn',
    '黑屏（隐私保护：屏幕不显示，串流与触控照常）',
    'M12 7c2.76 0 5 2.24 5 5 0 .65-.13 1.26-.36 1.83l2.92 2.92c1.51-1.26 2.7-2.89 3.43-4.75-1.73-4.39-6-7.5-11-7.5-1.4 0-2.74.25-3.98.7l2.16 2.16C10.74 7.13 11.35 7 12 7zM2 4.27l2.28 2.28.46.46C3.08 8.3 1.78 10.02 1 12c1.73 4.39 6 7.5 11 7.5 1.55 0 3.03-.3 4.38-.84l.42.42L19.73 22 21 20.73 3.27 3 2 4.27z');

const TITLE_OFF = '黑屏（隐私保护：屏幕不显示，串流与触控照常）';
const TITLE_ON = '屏幕已黑（点击恢复显示）';

let screenOff = false;
let busy = false;

function flashError() {
    btn.classList.add('ctrl-error');
    setTimeout(() => btn.classList.remove('ctrl-error'), 1200);
}

// 可见状态提示：不弹窗，页面顶部淡入一行文字，3 秒后淡出。
// 之前失败是完全静默的 —— 用户点了没反应就无法判断是"请求没发出"
// 还是"请求发出但失败"，排查只能靠猜。
function showStatus(text, isError) {
    let el = document.getElementById('backlightStatus');
    if (!el) {
        el = document.createElement('div');
        el.id = 'backlightStatus';
        el.style.cssText = 'position:fixed;left:50%;top:16px;transform:translateX(-50%);' +
            'z-index:99999;padding:8px 16px;border-radius:8px;font-size:14px;' +
            'background:rgba(20,22,28,0.92);color:#e8eaed;pointer-events:none;' +
            'box-shadow:0 4px 16px rgba(0,0,0,0.4);opacity:0;transition:opacity 0.25s;';
        (document.body || document.documentElement).appendChild(el);
    }
    el.textContent = text;
    el.style.background = isError ? 'rgba(140,40,40,0.94)' : 'rgba(20,22,28,0.92)';
    el.style.opacity = '1';
    clearTimeout(el._hideTimer);
    el._hideTimer = setTimeout(() => { el.style.opacity = '0'; }, 3000);
}

async function setBacklight(off) {
    if (busy) return;
    busy = true;
    btn.classList.add('busy');
    showStatus(off ? '正在关闭屏幕…' : '正在恢复屏幕…', false);
    try {
        const res = await fetch('/api/screen/backlight', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ serial: GetConfigValue('device_id'), off: !!off })
        });
        const data = await res.json().catch(() => ({}));
        if (!res.ok || data.result !== 'success') {
            throw new Error(data.message || ('HTTP ' + res.status));
        }
        screenOff = !!off;
        btn.classList.toggle('active', screenOff);
        btn.setAttribute('title', screenOff ? TITLE_ON : TITLE_OFF);
        showStatus(screenOff ? '屏幕已关闭（不锁屏，串流与触控照常）' : '屏幕已恢复显示', false);
    } catch (e) {
        console.error('[backlight] 切换失败:', e);
        flashError();
        showStatus('黑屏操作失败: ' + (e && e.message ? e.message : e), true);
    } finally {
        busy = false;
        btn.classList.remove('busy');
    }
}

btn.setAttribute('title', TITLE_OFF);
btn.addEventListener('click', () => setBacklight(!screenOff));

// 离开页面时恢复背光，避免把手机留在黑屏状态
window.addEventListener('beforeunload', () => {
    if (!screenOff) return;
    const body = JSON.stringify({ serial: GetConfigValue('device_id'), off: false });
    try {
        if (navigator.sendBeacon) {
            navigator.sendBeacon('/api/screen/backlight', new Blob([body], { type: 'application/json' }));
        } else {
            fetch('/api/screen/backlight', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body, keepalive: true });
        }
    } catch (e) { /* ignore */ }
});

window.setDeviceBacklight = setBacklight;
