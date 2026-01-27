
(function() {
const UHID_KEYBOARD_MSG_CREATE = 12;
const UHID_KEYBOARD_MSG_INPUT = 13;
const UHID_KEYBOARD_MSG_DESTROY = 14;

const UHID_KEYBOARD_ID = 1;
const UHID_KEYBOARD_NAME = "Virtual Keyboard";

var uhidKeyboardEnabled = false;
let uhidKeyboardInitialized = false;

// 记录当前按下的键 (HID Usage IDs)
let pressedKeys = new Set();
let currentModifiers = 0;

// 键盘 HID 描述符 (Standard Keyboard)
const KEYBOARD_REPORT_DESCRIPTOR = new Uint8Array([
    0x05, 0x01,       // Usage Page (Generic Desktop)
    0x09, 0x06,       // Usage (Keyboard)
    0xA1, 0x01,       // Collection (Application)
    0x05, 0x07,       //   Usage Page (Key Codes)
    0x19, 0xE0,       //   Usage Minimum (224)
    0x29, 0xE7,       //   Usage Maximum (231)
    0x15, 0x00,       //   Logical Minimum (0)
    0x25, 0x01,       //   Logical Maximum (1)
    0x75, 0x01,       //   Report Size (1)
    0x95, 0x08,       //   Report Count (8)
    0x81, 0x02,       //   Input (Data, Variable, Absolute) ; Modifier byte
    0x95, 0x01,       //   Report Count (1)
    0x75, 0x08,       //   Report Size (8)
    0x81, 0x01,       //   Input (Constant) ; Reserved byte
    0x95, 0x05,       //   Report Count (5)
    0x75, 0x01,       //   Report Size (1)
    0x05, 0x08,       //   Usage Page (LEDs)
    0x19, 0x01,       //   Usage Minimum (1)
    0x29, 0x05,       //   Usage Maximum (5)
    0x91, 0x02,       //   Output (Data, Variable, Absolute) ; LED report
    0x95, 0x01,       //   Report Count (1)
    0x75, 0x03,       //   Report Size (3)
    0x91, 0x01,       //   Output (Constant) ; LED report padding
    0x95, 0x06,       //   Report Count (6)
    0x75, 0x08,       //   Report Size (8)
    0x15, 0x00,       //   Logical Minimum (0)
    0x25, 0x65,       //   Logical Maximum (101)
    0x05, 0x07,       //   Usage Page (Key Codes)
    0x19, 0x00,       //   Usage Minimum (0)
    0x29, 0x65,       //   Usage Maximum (101)
    0x81, 0x00,       //   Input (Data, Array) ; Key arrays (6 bytes)
    0xC0              // End Collection
]);

// JS Code -> HID Usage ID 映射
const KEY_MAP = {
    'KeyA': 0x04, 'KeyB': 0x05, 'KeyC': 0x06, 'KeyD': 0x07, 'KeyE': 0x08,
    'KeyF': 0x09, 'KeyG': 0x0A, 'KeyH': 0x0B, 'KeyI': 0x0C, 'KeyJ': 0x0D,
    'KeyK': 0x0E, 'KeyL': 0x0F, 'KeyM': 0x10, 'KeyN': 0x11, 'KeyO': 0x12,
    'KeyP': 0x13, 'KeyQ': 0x14, 'KeyR': 0x15, 'KeyS': 0x16, 'KeyT': 0x17,
    'KeyU': 0x18, 'KeyV': 0x19, 'KeyW': 0x1A, 'KeyX': 0x1B, 'KeyY': 0x1C, 'KeyZ': 0x1D,
    'Digit1': 0x1E, 'Digit2': 0x1F, 'Digit3': 0x20, 'Digit4': 0x21, 'Digit5': 0x22,
    'Digit6': 0x23, 'Digit7': 0x24, 'Digit8': 0x25, 'Digit9': 0x26, 'Digit0': 0x27,
    'Enter': 0x28, 'Escape': 0x29, 'Backspace': 0x2A, 'Tab': 0x2B, 'Space': 0x2C,
    'Minus': 0x2D, 'Equal': 0x2E, 'BracketLeft': 0x2F, 'BracketRight': 0x30,
    'Backslash': 0x31, 'Semicolon': 0x33, 'Quote': 0x34, 'Backquote': 0x35,
    'Comma': 0x36, 'Period': 0x37, 'Slash': 0x38, 'CapsLock': 0x39,
    'F1': 0x3A, 'F2': 0x3B, 'F3': 0x3C, 'F4': 0x3D, 'F5': 0x3E, 'F6': 0x3F,
    'F7': 0x40, 'F8': 0x41, 'F9': 0x42, 'F10': 0x43, 'F11': 0x44, 'F12': 0x45,
    'PrintScreen': 0x46, 'ScrollLock': 0x47, 'Pause': 0x48, 'Insert': 0x49,
    'Home': 0x4A, 'PageUp': 0x4B, 'Delete': 0x4C, 'End': 0x4D, 'PageDown': 0x4E,
    'ArrowRight': 0x4F, 'ArrowLeft': 0x50, 'ArrowDown': 0x51, 'ArrowUp': 0x52,
    'NumLock': 0x53, 'NumpadDivide': 0x54, 'NumpadMultiply': 0x55, 'NumpadSubtract': 0x56,
    'NumpadAdd': 0x57, 'NumpadEnter': 0x58, 'Numpad1': 0x59, 'Numpad2': 0x5A,
    'Numpad3': 0x5B, 'Numpad4': 0x5C, 'Numpad5': 0x5D, 'Numpad6': 0x5E,
    'Numpad7': 0x5F, 'Numpad8': 0x60, 'Numpad9': 0x61, 'Numpad0': 0x62, 'NumpadDecimal': 0x63,
    'ContextMenu': 0x65,
};

const MODIFIER_MAP = {
    'ControlLeft': 0x01, 'ShiftLeft': 0x02, 'AltLeft': 0x04, 'MetaLeft': 0x08,
    'ControlRight': 0x10, 'ShiftRight': 0x20, 'AltRight': 0x40, 'MetaRight': 0x80
};

function initUHIDKeyboard() {
    if (uhidKeyboardInitialized) return;

    if (window.ws && window.ws.readyState === WebSocket.OPEN) {
        window.ws.send(createUHIDKeyboardDestroyPacket());
    }

    const packet = createUHIDKeyboardCreatePacket();
    if (window.ws && window.ws.readyState === WebSocket.OPEN) {
        window.ws.send(packet);
        uhidKeyboardInitialized = true;
        console.log("UHID Keyboard device created");
    }
}

function destroyUHIDKeyboard() {
    if (!uhidKeyboardInitialized) return;

    const packet = createUHIDKeyboardDestroyPacket();
    if (window.ws && window.ws.readyState === WebSocket.OPEN) {
        window.ws.send(packet);
        uhidKeyboardInitialized = false;
        uhidKeyboardEnabled = false;
        console.log("UHID Keyboard device destroyed");
    }
}

function toggleUHIDKeyboard() {
    const btn = document.getElementById('uhidKeyboardToggleBtn');
    if (!uhidKeyboardEnabled) {
        initUHIDKeyboard();
        uhidKeyboardEnabled = true;
        console.log("UHID Keyboard enabled");
        if (btn) btn.classList.add('active');
    } else {
        destroyUHIDKeyboard();
        uhidKeyboardEnabled = false;
        console.log("UHID Keyboard disabled");
        if (btn) btn.classList.remove('active');
    }
}

function sendKeyboardReport() {
    if (!uhidKeyboardEnabled || !uhidKeyboardInitialized) return;

    const packet = createUHIDKeyboardInputPacket(currentModifiers, Array.from(pressedKeys));
    // if (window.ws && window.ws.readyState === WebSocket.OPEN) {
    //     window.ws.send(packet);
    // }
    sendDataChannelMessage(window.dataChannelOrdered, packet);
}

// 事件监听
window.addEventListener('keydown', (event) => {
    if (!uhidKeyboardEnabled) return;
    
    // 阻止默认行为 (拦截所有按键，包括 F1-F12)
    event.preventDefault();

    let changed = false;

    // 处理修饰键
    if (MODIFIER_MAP[event.code]) {
        const newMods = currentModifiers | MODIFIER_MAP[event.code];
        if (newMods !== currentModifiers) {
            currentModifiers = newMods;
            changed = true;
        }
    } 
    // 处理普通键
    else if (KEY_MAP[event.code]) {
        const hidCode = KEY_MAP[event.code];
        if (!pressedKeys.has(hidCode)) {
            pressedKeys.add(hidCode);
            changed = true;
        }
    }

    if (changed) {
        sendKeyboardReport();
    }
});

window.addEventListener('keyup', (event) => {
    if (!uhidKeyboardEnabled) return;
    
    event.preventDefault();

    let changed = false;

    // 处理修饰键
    if (MODIFIER_MAP[event.code]) {
        const newMods = currentModifiers & ~MODIFIER_MAP[event.code];
        if (newMods !== currentModifiers) {
            currentModifiers = newMods;
            changed = true;
        }
    } 
    // 处理普通键
    else if (KEY_MAP[event.code]) {
        const hidCode = KEY_MAP[event.code];
        if (pressedKeys.has(hidCode)) {
            pressedKeys.delete(hidCode);
            changed = true;
        }
    }

    if (changed) {
        sendKeyboardReport();
    }
});

// 窗口失去焦点时重置所有按键
window.addEventListener('blur', () => {
    if (uhidKeyboardEnabled && (pressedKeys.size > 0 || currentModifiers !== 0)) {
        pressedKeys.clear();
        currentModifiers = 0;
        sendKeyboardReport();
    }
});


// ========== Packet Creation ==========

function createUHIDKeyboardCreatePacket() {
    const encoder = new TextEncoder();
    const rawName = UHID_KEYBOARD_NAME;
    const nameBytes = encoder.encode(rawName).slice(0, 255);
    const descriptor = KEYBOARD_REPORT_DESCRIPTOR;

    const buffer = new ArrayBuffer(8 + nameBytes.length + 2 + descriptor.length);
    const view = new DataView(buffer);
    const uint8View = new Uint8Array(buffer);

    let offset = 0;
    view.setUint8(offset, UHID_KEYBOARD_MSG_CREATE); offset += 1;
    view.setUint16(offset, UHID_KEYBOARD_ID); offset += 2;
    view.setUint16(offset, 0x18d1); offset += 2; // Vendor
    view.setUint16(offset, 0x0001); offset += 2; // Product
    view.setUint8(offset, nameBytes.length); offset += 1;
    
    if (nameBytes.length > 0) {
        uint8View.set(nameBytes, offset);
        offset += nameBytes.length;
    }

    view.setUint16(offset, descriptor.length); offset += 2;
    uint8View.set(descriptor, offset);

    return buffer;
}

function createUHIDKeyboardInputPacket(modifiers, keys) {
    // Report Size: 8 bytes
    // Byte 0: Modifiers
    // Byte 1: Reserved (0)
    // Byte 2-7: Key codes (up to 6)
    
    const reportSize = 8;
    const buffer = new ArrayBuffer(1 + 2 + 2 + reportSize);
    const view = new DataView(buffer);
    const uint8View = new Uint8Array(buffer);

    let offset = 0;
    view.setUint8(offset, UHID_KEYBOARD_MSG_INPUT); offset += 1;
    view.setUint16(offset, UHID_KEYBOARD_ID); offset += 2;
    view.setUint16(offset, reportSize); offset += 2;

    // HID Report
    view.setUint8(offset, modifiers); offset += 1;
    view.setUint8(offset, 0); offset += 1; // Reserved

    // Fill up to 6 keys
    for (let i = 0; i < 6; i++) {
        if (i < keys.length) {
            view.setUint8(offset, keys[i]);
        } else {
            view.setUint8(offset, 0);
        }
        offset += 1;
    }

    return buffer;
}

function createUHIDKeyboardDestroyPacket() {
    const buffer = new ArrayBuffer(3);
    const view = new DataView(buffer);
    view.setUint8(0, UHID_KEYBOARD_MSG_DESTROY);
    view.setUint16(1, UHID_KEYBOARD_ID);
    return buffer;
}
})();