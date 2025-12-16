
const UHID_GAMEPAD_MSG_CREATE = 12;
const UHID_GAMEPAD_MSG_INPUT = 13;
const UHID_GAMEPAD_MSG_DESTROY = 14;

const UHID_GAMEPAD_ID = 4;
const UHID_GAMEPAD_NAME = "Scrcpy Gamepad";

let uhidGamepadEnabled = false;
let uhidGamepadInitialized = false;

// Gamepad State
// Buttons: 16 bits (2 bytes)
// Axes: X, Y, Z, Rz (4 bytes, 8-bit signed -127 to 127)
let gamepadState = {
    buttons: 0,
    x: 0,
    y: 0,
    z: 0,
    rz: 0
};

// HID Descriptor for a Generic Gamepad
// 16 Buttons, 4 Axes (X, Y, Z, Rz)
const GAMEPAD_REPORT_DESCRIPTOR = new Uint8Array([
    0x05, 0x01,        // Usage Page (Generic Desktop Ctrls)
    0x09, 0x05,        // Usage (Game Pad)
    0xA1, 0x01,        // Collection (Application)
    0xA1, 0x00,        //   Collection (Physical)
    0x05, 0x09,        //     Usage Page (Button)
    0x19, 0x01,        //     Usage Minimum (0x01)
    0x29, 0x10,        //     Usage Maximum (0x10)
    0x15, 0x00,        //     Logical Minimum (0)
    0x25, 0x01,        //     Logical Maximum (1)
    0x75, 0x01,        //     Report Size (1)
    0x95, 0x10,        //     Report Count (16)
    0x81, 0x02,        //     Input (Data,Var,Abs)
    0x05, 0x01,        //     Usage Page (Generic Desktop Ctrls)
    0x09, 0x30,        //     Usage (X)
    0x09, 0x31,        //     Usage (Y)
    0x09, 0x32,        //     Usage (Z)
    0x09, 0x35,        //     Usage (Rz)
    0x15, 0x81,        //     Logical Minimum (-127)
    0x25, 0x7F,        //     Logical Maximum (127)
    0x75, 0x08,        //     Report Size (8)
    0x95, 0x04,        //     Report Count (4)
    0x81, 0x02,        //     Input (Data,Var,Abs)
    0xC0,              //   End Collection
    0xC0               // End Collection
]);

// Button Mappings (Bit positions)
const BTN_A = 0;
const BTN_B = 1;
const BTN_C = 2;
const BTN_X = 3;
const BTN_Y = 4;
const BTN_Z = 5;
const BTN_L1 = 6;
const BTN_R1 = 7;
const BTN_L2 = 8;
const BTN_R2 = 9;
const BTN_SELECT = 10;
const BTN_START = 11;
const BTN_MODE = 12;
const BTN_THUMBL = 13;
const BTN_THUMBR = 14;

function initUHIDGamepad() {
    if (uhidGamepadInitialized) return;

    if (window.ws && window.ws.readyState === WebSocket.OPEN) {
        window.ws.send(createUHIDGamepadDestroyPacket());
    }

    const packet = createUHIDGamepadCreatePacket();
    if (window.ws && window.ws.readyState === WebSocket.OPEN) {
        window.ws.send(packet);
        uhidGamepadInitialized = true;
        console.log("UHID Gamepad device created");
    }
}

function destroyUHIDGamepad() {
    if (!uhidGamepadInitialized) return;

    const packet = createUHIDGamepadDestroyPacket();
    if (window.ws && window.ws.readyState === WebSocket.OPEN) {
        window.ws.send(packet);
        uhidGamepadInitialized = false;
        uhidGamepadEnabled = false;
        console.log("UHID Gamepad device destroyed");
    }
}

function toggleUHIDGamepad() {
    if (!uhidGamepadEnabled) {
        initUHIDGamepad();
        uhidGamepadEnabled = true;
        createVirtualGamepadUI();
        console.log("UHID Gamepad enabled");
    } else {
        removeVirtualGamepadUI();
        destroyUHIDGamepad();
        uhidGamepadEnabled = false;
        console.log("UHID Gamepad disabled");
    }
}

function sendGamepadReport() {
    if (!uhidGamepadEnabled || !uhidGamepadInitialized) return;

    const packet = createUHIDGamepadInputPacket(gamepadState);
    if (window.ws && window.ws.readyState === WebSocket.OPEN) {
        window.ws.send(packet);
    }
}

// ========== Virtual UI ==========

function createVirtualGamepadUI() {
    const container = document.querySelector('.video-container');
    if (!container) return;

    const gamepadDiv = document.createElement('div');
    gamepadDiv.id = 'virtual-gamepad';
    gamepadDiv.style.position = 'absolute';
    gamepadDiv.style.top = '0';
    gamepadDiv.style.left = '0';
    gamepadDiv.style.width = '100%';
    gamepadDiv.style.height = '100%';
    gamepadDiv.style.pointerEvents = 'none'; // Allow clicks to pass through empty areas
    gamepadDiv.style.zIndex = '100';
    gamepadDiv.style.display = 'flex';
    gamepadDiv.style.justifyContent = 'space-between';
    gamepadDiv.style.alignItems = 'flex-end';
    gamepadDiv.style.padding = '20px';
    gamepadDiv.style.boxSizing = 'border-box';

    // Left Side: D-Pad
    const leftControls = document.createElement('div');
    leftControls.className = 'gamepad-controls left';
    leftControls.style.pointerEvents = 'auto';
    leftControls.style.width = '150px';
    leftControls.style.height = '150px';
    leftControls.style.position = 'relative';
    leftControls.style.background = 'rgba(255, 255, 255, 0.1)';
    leftControls.style.borderRadius = '50%';
    leftControls.style.marginBottom = '20px';
    leftControls.style.marginLeft = '20px';

    // D-Pad Buttons
    const dpadUp = createButton('Up', '50px', '0', '50px', '50px');
    const dpadDown = createButton('Down', '50px', '100px', '50px', '50px');
    const dpadLeft = createButton('Left', '0', '50px', '50px', '50px');
    const dpadRight = createButton('Right', '100px', '50px', '50px', '50px');

    // D-Pad Logic (Simulate Axis)
    setupDpadButton(dpadUp, 0, -127);
    setupDpadButton(dpadDown, 0, 127);
    setupDpadButton(dpadLeft, -127, 0);
    setupDpadButton(dpadRight, 127, 0);

    leftControls.appendChild(dpadUp);
    leftControls.appendChild(dpadDown);
    leftControls.appendChild(dpadLeft);
    leftControls.appendChild(dpadRight);

    // Right Side: Action Buttons
    const rightControls = document.createElement('div');
    rightControls.className = 'gamepad-controls right';
    rightControls.style.pointerEvents = 'auto';
    rightControls.style.width = '150px';
    rightControls.style.height = '150px';
    rightControls.style.position = 'relative';
    rightControls.style.marginBottom = '20px';
    rightControls.style.marginRight = '20px';

    // ABXY Buttons
    // Layout:
    //      Y
    //   X     B
    //      A
    const btnY = createButton('Y', '50px', '0', '50px', '50px', 'rgba(255, 255, 0, 0.3)');
    const btnA = createButton('A', '50px', '100px', '50px', '50px', 'rgba(0, 255, 0, 0.3)');
    const btnX = createButton('X', '0', '50px', '50px', '50px', 'rgba(0, 0, 255, 0.3)');
    const btnB = createButton('B', '100px', '50px', '50px', '50px', 'rgba(255, 0, 0, 0.3)');

    setupActionButton(btnA, BTN_A);
    setupActionButton(btnB, BTN_B);
    setupActionButton(btnX, BTN_X);
    setupActionButton(btnY, BTN_Y);

    rightControls.appendChild(btnY);
    rightControls.appendChild(btnA);
    rightControls.appendChild(btnX);
    rightControls.appendChild(btnB);

    gamepadDiv.appendChild(leftControls);
    gamepadDiv.appendChild(rightControls);

    container.appendChild(gamepadDiv);
}

function removeVirtualGamepadUI() {
    const gamepadDiv = document.getElementById('virtual-gamepad');
    if (gamepadDiv) {
        gamepadDiv.remove();
    }
}

function createButton(text, left, top, width, height, color = 'rgba(255, 255, 255, 0.2)') {
    const btn = document.createElement('div');
    btn.innerText = text;
    btn.style.position = 'absolute';
    btn.style.left = left;
    btn.style.top = top;
    btn.style.width = width;
    btn.style.height = height;
    btn.style.backgroundColor = color;
    btn.style.borderRadius = '50%';
    btn.style.display = 'flex';
    btn.style.justifyContent = 'center';
    btn.style.alignItems = 'center';
    btn.style.userSelect = 'none';
    btn.style.color = 'white';
    btn.style.fontWeight = 'bold';
    btn.style.cursor = 'pointer';
    btn.style.border = '1px solid rgba(255,255,255,0.3)';
    return btn;
}

function setupDpadButton(element, dx, dy) {
    const handleDown = (e) => {
        e.preventDefault();
        if (dx !== 0) gamepadState.x = dx;
        if (dy !== 0) gamepadState.y = dy;
        element.style.backgroundColor = 'rgba(255, 255, 255, 0.5)';
        sendGamepadReport();
    };
    const handleUp = (e) => {
        e.preventDefault();
        if (dx !== 0) gamepadState.x = 0;
        if (dy !== 0) gamepadState.y = 0;
        element.style.backgroundColor = 'rgba(255, 255, 255, 0.2)';
        sendGamepadReport();
    };

    element.addEventListener('mousedown', handleDown);
    element.addEventListener('mouseup', handleUp);
    element.addEventListener('mouseleave', handleUp);
    element.addEventListener('touchstart', handleDown);
    element.addEventListener('touchend', handleUp);
}

function setupActionButton(element, btnIndex) {
    const handleDown = (e) => {
        e.preventDefault();
        gamepadState.buttons |= (1 << btnIndex);
        element.style.opacity = '0.8';
        sendGamepadReport();
    };
    const handleUp = (e) => {
        e.preventDefault();
        gamepadState.buttons &= ~(1 << btnIndex);
        element.style.opacity = '1.0';
        sendGamepadReport();
    };

    element.addEventListener('mousedown', handleDown);
    element.addEventListener('mouseup', handleUp);
    element.addEventListener('mouseleave', handleUp);
    element.addEventListener('touchstart', handleDown);
    element.addEventListener('touchend', handleUp);
}

// ========== Packet Creation ==========

function createUHIDGamepadCreatePacket() {
    const encoder = new TextEncoder();
    const rawName = UHID_GAMEPAD_NAME;
    const nameBytes = encoder.encode(rawName).slice(0, 255);
    const descriptor = GAMEPAD_REPORT_DESCRIPTOR;

    const buffer = new ArrayBuffer(8 + nameBytes.length + 2 + descriptor.length);
    const view = new DataView(buffer);
    const uint8View = new Uint8Array(buffer);

    let offset = 0;
    view.setUint8(offset, UHID_GAMEPAD_MSG_CREATE); offset += 1;
    view.setUint16(offset, UHID_GAMEPAD_ID); offset += 2;
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

function createUHIDGamepadInputPacket(state) {
    // Report Size: 6 bytes
    // Byte 0-1: Buttons (Little Endian in HID report usually, but let's check descriptor)
    // Descriptor says: Report Count 16, Report Size 1. So 16 bits.
    // Byte 2: X
    // Byte 3: Y
    // Byte 4: Z
    // Byte 5: Rz
    
    const reportSize = 6;
    const buffer = new ArrayBuffer(1 + 2 + 2 + reportSize);
    const view = new DataView(buffer);

    let offset = 0;
    view.setUint8(offset, UHID_GAMEPAD_MSG_INPUT); offset += 1;
    view.setUint16(offset, UHID_GAMEPAD_ID); offset += 2;
    view.setUint16(offset, reportSize); offset += 2;

    // HID Report Data
    // Buttons (16 bits) - Little Endian for HID usually
    view.setUint16(offset, state.buttons, true); offset += 2;
    
    view.setInt8(offset, state.x); offset += 1;
    view.setInt8(offset, state.y); offset += 1;
    view.setInt8(offset, state.z); offset += 1;
    view.setInt8(offset, state.rz); offset += 1;

    return buffer;
}

function createUHIDGamepadDestroyPacket() {
    const buffer = new ArrayBuffer(3);
    const view = new DataView(buffer);
    view.setUint8(0, UHID_GAMEPAD_MSG_DESTROY);
    view.setUint16(1, UHID_GAMEPAD_ID);
    return buffer;
}
