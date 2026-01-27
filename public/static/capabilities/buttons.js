(function () {
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
        window.MANNUAL_ROTATE = true;
        setTimeout(() => {
            updateVideoCache();
        }, 500);
    }

    function sendButtonEvent(packet) {
        sendDataChannelMessage(window.dataChannelOrdered, packet);
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

    window.document.querySelector('#volumeUpButton').addEventListener('click', volumeUpButton);
    window.document.querySelector('#volumeDownButton').addEventListener('click', volumeDownButton);
    // window.document.querySelector('#powerButton').addEventListener('click', powerButton);
    window.document.querySelector('#powerButton').addEventListener('mousedown', () => pressButton(26));
    window.document.querySelector('#powerButton').addEventListener('mouseup', () => releaseButton(26));
    window.document.querySelector('#powerButton').addEventListener('mouseleave', () => releaseButton(26));
    window.document.querySelector('#homeButton').addEventListener('click', homeButton);
    window.document.querySelector('#backButton').addEventListener('click', backButton);
    window.document.querySelector('#menuButton').addEventListener('click', menuButton);
    window.document.querySelector('#rotateButton').addEventListener('click', rotateButton);
})();