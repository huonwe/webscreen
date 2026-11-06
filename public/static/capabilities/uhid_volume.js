/* UHID 音量键 —— 内核级虚拟 Consumer Control 设备
 *
 * 为什么不用 scrcpy 的按键注入（TYPE_INJECT_KEYCODE）：
 *   注入走的是 Android 系统输入管道（InputManager.injectInputEvent），
 *   部分系统级界面会过滤这类"注入"事件，表现为点了没反应。
 *   UHID 则是内核虚拟 HID 设备，对系统而言与插了一个真实外接键盘无异，
 *   兼容性最强 —— 与虚拟键盘/鼠标同一机制（已在真机验证可用）。
 *
 * 协议：复用前端 → Go 的自定义 dataChannel 消息（见 sdriver/scrcpy/messages.go）
 *   12 = UHID_CREATE, 13 = UHID_INPUT, 14 = UHID_DESTROY
 *   设备号：1=键盘(uhid_keyboard.js) 2=鼠标(uhid_mouse.js) 4=手柄(uhid_gamepad.js)
 *          → 本设备固定用 3
 *
 * report（1 字节）：bit0 = Volume Increment，bit1 = Volume Decrement
 *   Consumer Usage 0xE9/0xEA 由内核 HID 驱动映射为 KEY_VOLUMEUP/DOWN(115/114)，
 *   Android 再映射为 KEYCODE_VOLUME_UP/DOWN(24/25) —— 与物理音量键完全同源。
 */
(function () {
    const UHID_VOLUME_MSG_CREATE = 12;
    const UHID_VOLUME_MSG_INPUT = 13;
    const UHID_VOLUME_MSG_DESTROY = 14;

    const UHID_VOLUME_ID = 3;
    const UHID_VOLUME_NAME = "Virtual Volume Keys";
    const UHID_VOLUME_VENDOR = 0x18d1;
    const UHID_VOLUME_PRODUCT = 0x0003;

    const VOLUME_BIT_UP = 0x01;
    const VOLUME_BIT_DOWN = 0x02;

    // Consumer Control 报告描述符
    const VOLUME_REPORT_DESCRIPTOR = new Uint8Array([
        0x05, 0x0C,       // Usage Page (Consumer)
        0x09, 0x01,       // Usage (Consumer Control)
        0xA1, 0x01,       // Collection (Application)
        0x15, 0x00,       //   Logical Minimum (0)
        0x25, 0x01,       //   Logical Maximum (1)
        0x75, 0x01,       //   Report Size (1)
        0x95, 0x02,       //   Report Count (2)
        0x09, 0xE9,       //   Usage (Volume Increment)
        0x09, 0xEA,       //   Usage (Volume Decrement)
        0x81, 0x02,       //   Input (Data,Var,Abs)
        0x95, 0x06,       //   Report Count (6)   ← 补齐到 1 字节
        0x81, 0x01,       //   Input (Const,Array,Abs)
        0xC0              // End Collection
    ]);

    let deviceCreated = false;
    let boundChannel = null;

    function channel() {
        const ch = window.dataChannelOrdered;
        if (!ch || ch.readyState !== 'open') return null;
        return ch;
    }

    function send(buffer) {
        const ch = channel();
        if (!ch) return false;
        try {
            ch.send(buffer);
            return true;
        } catch (e) {
            console.warn('[uhid-volume] send failed:', e);
            return false;
        }
    }

    function createPacket() {
        const nameBytes = new TextEncoder().encode(UHID_VOLUME_NAME).slice(0, 255);
        const descriptor = VOLUME_REPORT_DESCRIPTOR;
        const buffer = new ArrayBuffer(8 + nameBytes.length + 2 + descriptor.length);
        const view = new DataView(buffer);
        const bytes = new Uint8Array(buffer);

        let offset = 0;
        view.setUint8(offset, UHID_VOLUME_MSG_CREATE); offset += 1;
        view.setUint16(offset, UHID_VOLUME_ID); offset += 2;
        view.setUint16(offset, UHID_VOLUME_VENDOR); offset += 2;
        view.setUint16(offset, UHID_VOLUME_PRODUCT); offset += 2;
        view.setUint8(offset, nameBytes.length); offset += 1;
        bytes.set(nameBytes, offset); offset += nameBytes.length;
        view.setUint16(offset, descriptor.length); offset += 2;
        bytes.set(descriptor, offset);
        return buffer;
    }

    function inputPacket(report) {
        const buffer = new ArrayBuffer(1 + 2 + 2 + 1);
        const view = new DataView(buffer);
        let offset = 0;
        view.setUint8(offset, UHID_VOLUME_MSG_INPUT); offset += 1;
        view.setUint16(offset, UHID_VOLUME_ID); offset += 2;
        view.setUint16(offset, 1); offset += 2;   // report size
        view.setUint8(offset, report);
        return buffer;
    }

    function destroyPacket() {
        const buffer = new ArrayBuffer(3);
        const view = new DataView(buffer);
        view.setUint8(0, UHID_VOLUME_MSG_DESTROY);
        view.setUint16(1, UHID_VOLUME_ID);
        return buffer;
    }

    // 确保设备存在；重连（dataChannel 换对象）后服务端设备已消失，需重建
    function ensureDevice() {
        const ch = channel();
        if (!ch) return false;
        if (ch !== boundChannel) {
            boundChannel = ch;
            deviceCreated = false;
        }
        if (deviceCreated) return true;
        if (!send(createPacket())) return false;
        deviceCreated = true;
        console.log('[uhid-volume] Consumer Control 设备已创建 (id=' + UHID_VOLUME_ID + ')');
        return true;
    }

    /**
     * 发音量键（内核虚拟 HID，与物理音量键同源）
     * @param {'up'|'down'} direction
     * @returns {boolean} 是否已通过 UHID 发出（false 时调用方可回退到按键注入）
     */
    window.uhidVolumePress = function (direction) {
        const fresh = !deviceCreated;
        if (!ensureDevice()) return false;

        const bit = direction === 'down' ? VOLUME_BIT_DOWN : VOLUME_BIT_UP;
        // 刚创建设备时稍等其就绪，否则首个 report 可能被丢弃
        const start = fresh ? 150 : 0;

        setTimeout(function () { send(inputPacket(bit)); }, start);          // 按下
        setTimeout(function () { send(inputPacket(0)); }, start + 100);      // 抬起
        return true;
    };

    // 提供给外部：主动销毁（例如关闭串流前）
    window.uhidVolumeDestroy = function () {
        if (!deviceCreated) return;
        send(destroyPacket());
        deviceCreated = false;
        boundChannel = null;
    };
})();
