import { addControlButton, addControlSeparator } from './control_ui.js';

const TYPE_KEY = 0x00; // key event
const TYPE_KEY_ACTION_DOWN = 0;
const TYPE_KEY_ACTION_UP = 1;
function homeButton() {
    // KEYCODE_HOME = 3
    const KEYCODE_HOME = 3;

    // Send Down
    let p = createKeyPacket(TYPE_KEY_ACTION_DOWN, KEYCODE_HOME);
    sendButtonEvent(p);

    // Send Up
    p = createKeyPacket(TYPE_KEY_ACTION_UP, KEYCODE_HOME);
    sendButtonEvent(p);
}

function volumeUpButton() {
    // 优先走内核 UHID 虚拟音量键（Consumer Control）——对系统而言等同真实
    // 外接键盘，系统级界面也接受；不可用时回退到 scrcpy 按键注入。
    if (window.uhidVolumePress && window.uhidVolumePress('up')) return;
    const KEYCODE_VOLUME_UP = 24;
    let p = createKeyPacket(TYPE_KEY_ACTION_DOWN, KEYCODE_VOLUME_UP);
    sendButtonEvent(p);
    p = createKeyPacket(TYPE_KEY_ACTION_UP, KEYCODE_VOLUME_UP);
    sendButtonEvent(p);
}

function volumeDownButton() {
    if (window.uhidVolumePress && window.uhidVolumePress('down')) return;
    const KEYCODE_VOLUME_DOWN = 25;
    let p = createKeyPacket(TYPE_KEY_ACTION_DOWN, KEYCODE_VOLUME_DOWN);
    sendButtonEvent(p);
    p = createKeyPacket(TYPE_KEY_ACTION_UP, KEYCODE_VOLUME_DOWN);
    sendButtonEvent(p);
}

function powerButton() {
    const KEYCODE_POWER = 26;
    let p = createKeyPacket(TYPE_KEY_ACTION_DOWN, KEYCODE_POWER);
    sendButtonEvent(p);
    p = createKeyPacket(TYPE_KEY_ACTION_UP, KEYCODE_POWER);
    sendButtonEvent(p);
}

function pressButton(keyCode) {
    let p = createKeyPacket(TYPE_KEY_ACTION_DOWN, keyCode);
    sendButtonEvent(p);
}

function releaseButton(keyCode) {
    let p = createKeyPacket(TYPE_KEY_ACTION_UP, keyCode);
    sendButtonEvent(p);
}

function backButton() {
    const KEYCODE_BACK = 4;
    let p = createKeyPacket(TYPE_KEY_ACTION_DOWN, KEYCODE_BACK);
    sendButtonEvent(p);
    p = createKeyPacket(TYPE_KEY_ACTION_UP, KEYCODE_BACK);
    sendButtonEvent(p);
}

function menuButton() {
    const KEYCODE_APP_SWITCH = 187;
    let p = createKeyPacket(TYPE_KEY_ACTION_DOWN, KEYCODE_APP_SWITCH);
    sendButtonEvent(p);
    p = createKeyPacket(TYPE_KEY_ACTION_UP, KEYCODE_APP_SWITCH);
    sendButtonEvent(p);
}

function rotateButton() {
    let p = createRotatePacket();
    sendButtonEvent(p);
    window.MANNUAL_ROTATE = true;
    setTimeout(() => {
        updateVideoCache();
    }, 500);
}

function sendButtonEvent(packet) {
    window.sendDataChannelMessage(window.dataChannelOrdered, packet);
}

const TYPE_ROTATE = 0x0B; // rotate event
function createRotatePacket() {
    const buffer = new ArrayBuffer(1);
    const view = new DataView(buffer);
    view.setUint8(0, TYPE_ROTATE);
    return buffer;
}
function createKeyPacket(action, keyCode) {
    const buffer = new ArrayBuffer(4);
    const view = new DataView(buffer);
    view.setUint8(0, TYPE_KEY);
    view.setUint8(1, action);
    view.setUint16(2, keyCode);
    return buffer;
}

window.sendButtonEvent = sendButtonEvent;
window.createRotatePacket = createRotatePacket;

addControlSeparator();
addControlButton('volumeUpButton', 'volume up',
    'M3 9v6h4l5 5V4L7 9H3zm13.5 3c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25 2.5-4.02zM14 3.23v2.06c2.89.86 5 3.54 5 6.71s-2.11 5.85-5 6.71v2.06c4.01-.91 7-4.49 7-8.77s-2.99-7.86-7-8.77z')
    .addEventListener('click', volumeUpButton);
addControlButton('volumeDownButton', 'volume down',
    'M18.5 12c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25-2.5-4.02zM5 9v6h4l5 5V4L9 9H5z')
    .addEventListener('click', volumeDownButton);
// window.document.querySelector('#powerButton').addEventListener('click', powerButton);
const power = addControlButton('powerButton', 'power',
    'M13 3h-2v10h2V3zm4.83 2.17-1.42 1.42C17.99 7.86 19 9.81 19 12c0 3.87-3.13 7-7 7s-7-3.13-7-7c0-2.19 1.01-4.14 2.58-5.42L6.17 5.17C4.23 6.82 3 9.26 3 12c0 4.97 4.03 9 9 9s9-4.03 9-9c0-2.74-1.23-5.18-3.17-6.83z');
power.addEventListener('mousedown', () => pressButton(26));
power.addEventListener('mouseup', () => releaseButton(26));
power.addEventListener('mouseleave', () => releaseButton(26));
addControlButton('backButton', 'back', 'M20 11H7.83l5.59-5.59L12 4l-8 8 8 8 1.41-1.41L7.83 13H20v-2z')
    .addEventListener('click', backButton);
addControlButton('homeButton', 'home', 'M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z')
    .addEventListener('click', homeButton);
addControlButton('menuButton', 'menu', 'M3 18h18v-2H3v2zm0-5h18v-2H3v2zm0-7v2h18V6H3z')
    .addEventListener('click', menuButton);
addControlSeparator();
addControlButton('rotateButton', 'rotate', 'M5 10a7 7 0 0 1 7-7v3l4-4-4-4v3a9 9 0 0 0-9 9h2zm14 4a7 7 0 0 1-7 7v-3l-4 4 4 4v-3a9 9 0 0 0 9-9h-2z')
    .addEventListener('click', rotateButton);
