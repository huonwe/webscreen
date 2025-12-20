var MANNUAL_ROTATE = false

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
    const KEYCODE_VOLUME_UP = 24;
    let p = createKeyPacket(TYPE_KEY_ACTION_DOWN, KEYCODE_VOLUME_UP);
    sendButtonEvent(p);
    p = createKeyPacket(TYPE_KEY_ACTION_UP, KEYCODE_VOLUME_UP);
    sendButtonEvent(p);
}

function volumeDownButton() {
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
    MANNUAL_ROTATE = true;
}

function sendButtonEvent(packet) {
    if (window.ws && window.ws.readyState === WebSocket.OPEN) {
        window.ws.send(packet);
    } else {
        console.warn("WebSocket is not open. Cannot send button event.");
    }
}