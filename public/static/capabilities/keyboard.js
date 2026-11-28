import { UHIDKeyboardEnabled } from './uhid_keyboard.js'
import { SendPacket } from '../connect-new.js'
const TYPE_KEY = 0x00;
const TYPE_KEY_ACTION_DOWN = 0;
const TYPE_KEY_ACTION_UP = 1;

// 物理键位 (e.code) -> 协议键值映射表
const KEY_CODE_MAP = {
    "Enter": 66,
    "Backspace": 67,
    "Delete": 112,
    "Escape": 111,
    "Home": 122,
    "ArrowUp": 19,
    "ArrowDown": 20,
    "ArrowLeft": 21,
    "ArrowRight": 22,
    "Space": 62,
    "Tab": 61,
    "ShiftLeft": 59,
    "ShiftRight": 60,
    "ControlLeft": 113,
    "ControlRight": 114,
    "AltLeft": 57,
    "AltRight": 58,
    "MetaLeft": 117,
    "MetaRight": 118,
    "CapsLock": 115,
    "PageUp": 92,
    "PageDown": 93,
    "End": 123,
    "Insert": 124,
    "Minus": 69,
    "Equal": 70,
    "BracketLeft": 71,
    "BracketRight": 72,
    "Backslash": 73,
    "Semicolon": 74,
    "Quote": 75,
    "Backquote": 68,
    "Comma": 55,
    "Period": 56,
    "Slash": 76,
};

for (let i = 0; i < 26; i++) {
    KEY_CODE_MAP["Key" + String.fromCharCode(65 + i)] = 29 + i;
}
for (let i = 0; i < 10; i++) {
    KEY_CODE_MAP["Digit" + i] = 7 + i;
}
// 启用 F1-F12 映射（与 Go 后端 131..142 对齐）
for (let i = 1; i <= 12; i++) {
    KEY_CODE_MAP["F" + i] = 130 + i;
}
// 需要阻止浏览器默认行为的键位集合
const INTERCEPT_CODES = new Set([
    "Escape",
    "ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight",
    "Space", "Tab", "Backspace",
    "F1", "F2", "F3", "F4", "F5", "F6",
    "F7", "F8", "F9", "F10", "F11", "F12",
]);

document.addEventListener('keydown', (e) => {
    if (UHIDKeyboardEnabled()) {
        return;
    }
    if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.isContentEditable) {
        return;
    }

    const keyCode = getKeyCode(e);
    if (keyCode === null) return;

    // 拦截 ESC、F1-F12、方向键及 Ctrl/Alt 组合键
    if (INTERCEPT_CODES.has(e.code) || e.ctrlKey || e.altKey) {
        e.preventDefault();
        e.stopPropagation();
    }

    if (e.repeat || pressedKeyCodes.has(keyCode)) return;

    pressedKeyCodes.add(keyCode);
    sendKeyboardEvent(TYPE_KEY_ACTION_DOWN, keyCode);
}, true); // 使用捕获阶段 (true)，优先于页面其他监听拿到按键

const pressedKeyCodes = new Set();

function getKeyCode(e) {
    if (e.code && KEY_CODE_MAP[e.code] !== undefined) {
        return KEY_CODE_MAP[e.code];
    }
    // 兜底：部分 Linux X11 / 特殊键盘布局下右 Shift 的 e.code 可能不是 "ShiftRight"
    if (e.key === "Shift") {
        return e.location === KeyboardEvent.DOM_KEY_LOCATION_RIGHT ? 60 : 59;
    }
    return null;
}

function releaseAllKeys() {
    if (pressedKeyCodes.size === 0) return;
    for (const keyCode of pressedKeyCodes) {
        sendKeyboardEvent(TYPE_KEY_ACTION_UP, keyCode);
    }
    pressedKeyCodes.clear();
}

document.addEventListener('keydown', (e) => {
    if (UHIDKeyboardEnabled()) {
        return;
    }
    if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.isContentEditable) {
        return;
    }

    const keyCode = getKeyCode(e);
    if (keyCode === null) return;

    if (
        ["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight", "Space", "Tab", "Backspace"].includes(e.code) ||
        e.ctrlKey || e.altKey
    ) {
        e.preventDefault();
    }

    // 过滤长按自动重复，仅在首次按下时发送 DOWN
    if (e.repeat || pressedKeyCodes.has(keyCode)) return;

    pressedKeyCodes.add(keyCode);
    sendKeyboardEvent(TYPE_KEY_ACTION_DOWN, keyCode);
});

document.addEventListener('keyup', (e) => {
    if (UHIDKeyboardEnabled()) {
        return;
    }

    const keyCode = getKeyCode(e);
    if (keyCode === null) return;

    e.preventDefault();
    pressedKeyCodes.delete(keyCode);
    sendKeyboardEvent(TYPE_KEY_ACTION_UP, keyCode);
});

window.addEventListener('blur', releaseAllKeys);
document.addEventListener('visibilitychange', () => {
    if (document.hidden) releaseAllKeys();
});
document.addEventListener('pointerlockchange', () => {
    if (!document.pointerLockElement) releaseAllKeys();
});

function sendKeyboardEvent(action, keyCode) {
    const packet = createKeyPacket(action, keyCode);
    SendPacket(packet);
}

function createKeyPacket(action, keyCode) {
    const buffer = new ArrayBuffer(4);
    const view = new DataView(buffer);
    view.setUint8(0, TYPE_KEY);
    view.setUint8(1, action);
    view.setUint16(2, keyCode, false);
    return buffer;
}