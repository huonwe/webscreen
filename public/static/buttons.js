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



function sendButtonEvent(packet) {
    if (window.ws && window.ws.readyState === WebSocket.OPEN) {
        window.ws.send(packet);
    } else {
        console.warn("WebSocket is not open. Cannot send button event.");
    }
}