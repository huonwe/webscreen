
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

function getAndroidKeyCode(e) {
    if (ANDROID_KEYCODES[e.code]) {
        return ANDROID_KEYCODES[e.code];
    }
    // Fallback for some keys that might not match e.code exactly or need special handling
    return null;
}

document.addEventListener('keydown', (e) => {
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
    const keyCode = getAndroidKeyCode(e);
    if (keyCode !== null) {
        sendKeyboardEvent(TYPE_KEY_ACTION_UP, keyCode);
    }
});

function sendKeyboardEvent(action, keyCode) {
    // console.log(`Sending keyboard event: action=${action}, keyCode=${keyCode}`);
    if (window.ws && window.ws.readyState === WebSocket.OPEN) {
        const packet = createKeyPacket(action, keyCode);
        window.ws.send(packet);
    }
}
