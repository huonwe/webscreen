const TYPE_KEY   = 0x00; // key event
// Key Packet Structure:
// 偏移,长度,类型,字段名,说明
// 0,1,uint8,Type,固定 0x02 (KeyEvent)
// 1,1,uint8,Action,"0: Down, 1: Up"
// 2,2,uint16,KeyCode,Android KeyCode (如 Power=26)
const TYPE_KEY_ACTION_DOWN = 0;
const TYPE_KEY_ACTION_UP = 1;

const ANDROID_KEYCODES = {
    "Enter": 66,
    "Backspace": 67,
    "Delete": 112,
    "Escape": 111,
    "Home": 3,
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
    // 映射符号键
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

// Map A-Z
for (let i = 0; i < 26; i++) {
    const char = String.fromCharCode(65 + i); // A-Z
    const code = "Key" + char;
    ANDROID_KEYCODES[code] = 29 + i; // AKEYCODE_A starts at 29
}

// Map 0-9
for (let i = 0; i < 10; i++) {
    const code = "Digit" + i;
    ANDROID_KEYCODES[code] = 7 + i; // AKEYCODE_0 starts at 7
}

// if not UHID, don't map F1-F12
// Map F1-F12
// for (let i = 1; i <= 12; i++) {
//     const code = "F" + i;
//     ANDROID_KEYCODES[code] = 130 + i; // AKEYCODE_F1 starts at 131
// }

function getAndroidKeyCode(e) {
    if (ANDROID_KEYCODES[e.code]) {
        return ANDROID_KEYCODES[e.code];
    }
    // Fallback for some keys that might not match e.code exactly or need special handling
    return null;
}

document.addEventListener('keydown', (e) => {
    // Check if UHID keyboard is enabled
    if (typeof uhidKeyboardEnabled !== 'undefined' && uhidKeyboardEnabled) {
        return;
    }

    // Ignore if typing in an input field (if we had any)
    if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') {
        return;
    }

    const keyCode = getAndroidKeyCode(e);
    if (keyCode !== null) {
        // Prevent default behavior for some keys to avoid browser scrolling/shortcuts
        if (["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight", "Space", "Tab", "Backspace"].includes(e.code)) {
            e.preventDefault();
        }
        // Repeat handling? Scrcpy protocol has repeat field but we are sending down/up.
        // If we hold a key, browser sends multiple keydowns.
        // We can just forward them.
        sendKeyboardEvent(TYPE_KEY_ACTION_DOWN, keyCode);
    }
});

document.addEventListener('keyup', (e) => {
    // Check if UHID keyboard is enabled
    if (typeof uhidKeyboardEnabled !== 'undefined' && uhidKeyboardEnabled) {
        return;
    }

    const keyCode = getAndroidKeyCode(e);
    if (keyCode !== null) {
        sendKeyboardEvent(TYPE_KEY_ACTION_UP, keyCode);
    }
});

function sendKeyboardEvent(action, keyCode) {
    // console.log(`Sending keyboard event: action=${action}, keyCode=${keyCode}`);
    // if (window.ws && window.ws.readyState === WebSocket.OPEN) {
    //     const packet = createKeyPacket(action, keyCode);
    //     window.ws.send(packet);
    // }
    const packet = createKeyPacket(action, keyCode);
    sendDataChannelMessage(DATA_CHANNEL_ORDERED, packet);
}

function createKeyPacket(action, keyCode) {
    const buffer = new ArrayBuffer(4);
    const view = new DataView(buffer);
    view.setUint8(0, TYPE_KEY);
    view.setUint8(1, action);
    view.setUint16(2, keyCode);
    return buffer;
}
