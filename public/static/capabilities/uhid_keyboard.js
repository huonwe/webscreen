
(function () {
    const UHID_KEYBOARD_MSG_CREATE = 12;
    const UHID_KEYBOARD_MSG_INPUT = 13;
    const UHID_KEYBOARD_MSG_DESTROY = 14;

    const UHID_KEYBOARD_ID = 1;
    const UHID_KEYBOARD_NAME = "Virtual Keyboard";

    window.uhidKeyboardEnabled = false;
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

        sendDataChannelMessage(window.dataChannelOrdered, createUHIDKeyboardDestroyPacket());

        const packet = createUHIDKeyboardCreatePacket();
        sendDataChannelMessage(window.dataChannelOrdered, packet);
        uhidKeyboardInitialized = true;
        console.log("UHID Keyboard device created");
    }

    function destroyUHIDKeyboard() {
        if (!uhidKeyboardInitialized) return;

        const packet = createUHIDKeyboardDestroyPacket();
        sendDataChannelMessage(window.dataChannelOrdered, packet);
        uhidKeyboardInitialized = false;
        window.uhidKeyboardEnabled = false;
        console.log("UHID Keyboard device destroyed");
    }

    function toggleUHIDKeyboard() {
        const btn = document.getElementById('uhidKeyboardToggleBtn');
        if (!window.uhidKeyboardEnabled) {
            initUHIDKeyboard();
            window.uhidKeyboardEnabled = true;
            console.log("UHID Keyboard enabled");
            if (btn) btn.classList.add('active');
            showVirtualKeyboard();
        } else {
            destroyUHIDKeyboard();
            window.uhidKeyboardEnabled = false;
            console.log("UHID Keyboard disabled");
            if (btn) btn.classList.remove('active');
            hideVirtualKeyboard();
        }
    }

    document.querySelector('#uhidKeyboardToggleBtn').addEventListener('click', toggleUHIDKeyboard);

    function sendKeyboardReport() {
        if (!window.uhidKeyboardEnabled || !uhidKeyboardInitialized) return;

        const packet = createUHIDKeyboardInputPacket(currentModifiers, Array.from(pressedKeys));
        // if (window.ws && window.ws.readyState === WebSocket.OPEN) {
        //     window.ws.send(packet);
        // }
        sendDataChannelMessage(window.dataChannelOrdered, packet);
    }

    // 事件监听
    window.addEventListener('keydown', (event) => {
        if (!window.uhidKeyboardEnabled) return;

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
        if (!window.uhidKeyboardEnabled) return;

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
        if (window.uhidKeyboardEnabled && (pressedKeys.size > 0 || currentModifiers !== 0)) {
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
    // ==================== 屏幕虚拟键盘面板 ====================
    // 与 uhid_gamepad.js 保持一致的做法：自注入样式 + createElement 构建面板，
    // 点侧边栏的 UHID Keyboard 按钮显示/隐藏。按键直接注入 UHID 键盘的同一套状态
    // (pressedKeys / currentModifiers) 并复用 sendKeyboardReport()，不新增任何协议。

    const VK_KEY = {
        ESC: 0x29, BACKSPACE: 0x2A, TAB: 0x2B, ENTER: 0x28, SPACE: 0x2C,
        UP: 0x52, DOWN: 0x51, LEFT: 0x50, RIGHT: 0x4F, DEL: 0x4C,
        '1': 0x1E, '2': 0x1F, '3': 0x20, '4': 0x21, '5': 0x22, '6': 0x23,
        '7': 0x24, '8': 0x25, '9': 0x26, '0': 0x27,
        '-': 0x2D, '=': 0x2E, '[': 0x2F, ']': 0x30, '\\': 0x31,
        ';': 0x33, "'": 0x34, '`': 0x35, ',': 0x36, '.': 0x37, '/': 0x38,
        a: 0x04, b: 0x05, c: 0x06, d: 0x07, e: 0x08, f: 0x09, g: 0x0A, h: 0x0B,
        i: 0x0C, j: 0x0D, k: 0x0E, l: 0x0F, m: 0x10, n: 0x11, o: 0x12, p: 0x13,
        q: 0x14, r: 0x15, s: 0x16, t: 0x17, u: 0x18, v: 0x19, w: 0x1A, x: 0x1B,
        y: 0x1C, z: 0x1D
    };
    // HID modifier 位（与 MODIFIER_MAP 的取值一致）
    const VK_MOD = { CTRL: 0x01, SHIFT: 0x02, ALT: 0x04, GUI: 0x08 };

    // 布局：{ t: 显示文字, k: 键值, m: modifier 位, w: 宽度倍数 }
    const VK_ROWS = [
        [{ t: 'Esc', k: 'ESC', w: 1.2 }, { t: '1', k: '1' }, { t: '2', k: '2' }, { t: '3', k: '3' },
         { t: '4', k: '4' }, { t: '5', k: '5' }, { t: '6', k: '6' }, { t: '7', k: '7' },
         { t: '8', k: '8' }, { t: '9', k: '9' }, { t: '0', k: '0' }, { t: '-', k: '-' },
         { t: '⌫', k: 'BACKSPACE', w: 1.4 }],
        [{ t: 'Tab', k: 'TAB', w: 1.2 }, { t: 'q', k: 'q' }, { t: 'w', k: 'w' }, { t: 'e', k: 'e' },
         { t: 'r', k: 'r' }, { t: 't', k: 't' }, { t: 'y', k: 'y' }, { t: 'u', k: 'u' },
         { t: 'i', k: 'i' }, { t: 'o', k: 'o' }, { t: 'p', k: 'p' }, { t: '[', k: '[' },
         { t: ']', k: ']' }, { t: '\\', k: '\\' }],
        [{ t: 'Ctrl', k: 'CTRL', m: VK_MOD.CTRL, w: 1.3, tone: 'mod' }, { t: 'a', k: 'a' }, { t: 's', k: 's' },
         { t: 'd', k: 'd' }, { t: 'f', k: 'f' }, { t: 'g', k: 'g' }, { t: 'h', k: 'h' },
         { t: 'j', k: 'j' }, { t: 'k', k: 'k' }, { t: 'l', k: 'l' }, { t: ';', k: ';' },
         { t: "'", k: "'" }, { t: 'Enter', k: 'ENTER', w: 1.6, tone: 'accent' }],
        [{ t: 'Shift', k: 'SHIFT', m: VK_MOD.SHIFT, w: 1.6, tone: 'mod' }, { t: 'z', k: 'z' }, { t: 'x', k: 'x' },
         { t: 'c', k: 'c' }, { t: 'v', k: 'v' }, { t: 'b', k: 'b' }, { t: 'n', k: 'n' },
         { t: 'm', k: 'm' }, { t: ',', k: ',' }, { t: '.', k: '.' }, { t: '/', k: '/' },
         { t: '↑', k: 'UP', w: 1.1 }, { t: '⇧', k: 'SHIFT', m: VK_MOD.SHIFT, w: 1.1, tone: 'mod' }],
        [{ t: 'Alt', k: 'ALT', m: VK_MOD.ALT, w: 1.2, tone: 'mod' }, { t: '‹', k: 'LEFT', w: 1.1 },
         { t: '↓', k: 'DOWN', w: 1.1 }, { t: '›', k: 'RIGHT', w: 1.1 },
         { t: '空格', k: 'SPACE', w: 5.4 }, { t: 'Del', k: 'DEL', w: 1.2 }]
    ];

    let vkRoot = null;
    let vkStylesInjected = false;
    // 触屏按住时生效的键（pointerup 时释放）；修饰键走 click 切换，不在此列
    const vkHeldKeys = new Set();

    function vkInjectStyles() {
        if (vkStylesInjected) return;
        vkStylesInjected = true;
        const style = document.createElement('style');
        style.id = 'vk-styles';
        style.textContent = `
            #virtual-keyboard {
                position: fixed; left: 50%; bottom: 12px; transform: translateX(-50%);
                z-index: 9998; display: none; flex-direction: column; gap: 4px;
                padding: 8px 10px 10px; border-radius: 12px;
                background: rgba(20, 22, 28, 0.82); backdrop-filter: blur(8px);
                box-shadow: 0 6px 24px rgba(0, 0, 0, 0.45); touch-action: none;
                user-select: none; -webkit-user-select: none; max-width: min(96vw, 1100px);
            }
            #virtual-keyboard.vk-visible { display: flex; }
            #vk-drag {
                height: 18px; margin: -2px 0 2px; cursor: move; border-radius: 6px;
                background: linear-gradient(180deg, rgba(255,255,255,0.16), rgba(255,255,255,0.04));
                display: flex; align-items: center; justify-content: center;
                font-size: 11px; letter-spacing: 2px; color: rgba(255,255,255,0.55);
            }
            #vk-rows { display: flex; flex-direction: column; gap: 4px; }
            .vk-row { display: flex; gap: 4px; justify-content: center; }
            .vk-key {
                flex: 1 1 0; min-width: 26px; height: 40px; line-height: 1;
                display: flex; align-items: center; justify-content: center;
                border: none; border-radius: 7px; padding: 0 4px;
                background: rgba(255,255,255,0.10); color: #e8eaed;
                font-size: 14px; font-family: inherit; cursor: pointer;
                transition: background 0.08s, transform 0.08s;
            }
            .vk-key.vk-wide { flex-grow: 1; }
            .vk-key.vk-mod { background: rgba(255,255,255,0.18); font-size: 12px; }
            .vk-key.vk-accent { background: rgba(120, 170, 255, 0.32); }
            .vk-key.vk-down { background: rgba(140, 190, 255, 0.55); transform: translateY(1px); }
            .vk-key.vk-locked { background: rgba(140, 190, 255, 0.62); color: #10131a; }
            @media (max-width: 720px) {
                .vk-key { height: 34px; font-size: 12px; min-width: 20px; }
                #virtual-keyboard { padding: 6px 6px 8px; bottom: 6px; }
            }
        `;
        document.head.appendChild(style);
    }

    function vkKeyElement(item) {
        const el = document.createElement('button');
        el.type = 'button';
        el.className = 'vk-key' + (item.m ? ' vk-mod' : '') + (item.tone === 'accent' ? ' vk-accent' : '');
        el.textContent = item.t;
        el.dataset.vkKey = item.k;
        if (item.m) el.dataset.vkMod = String(item.m);
        if (item.w && item.w > 1) el.style.flexGrow = String(item.w);
        el.addEventListener('pointerdown', (ev) => {
            ev.preventDefault();
            if (item.m) {
                // 修饰键：点一下切换锁定
                vkToggleModifier(item.m, el);
                return;
            }
            el.setPointerCapture && el.setPointerCapture(ev.pointerId);
            el.classList.add('vk-down');
            vkHeldKeys.add(item.k);
            vkSendKey(item.k, true);
        });
        const release = (ev) => {
            if (item.m) return;
            if (ev) ev.preventDefault();
            if (!vkHeldKeys.has(item.k)) return;
            vkHeldKeys.delete(item.k);
            el.classList.remove('vk-down');
            vkSendKey(item.k, false);
        };
        el.addEventListener('pointerup', release);
        el.addEventListener('pointercancel', release);
        el.addEventListener('pointerleave', release);
        el.addEventListener('contextmenu', (ev) => ev.preventDefault());
        return el;
    }

    function vkToggleModifier(bit, el) {
        currentModifiers ^= bit;
        if (currentModifiers & bit) el.classList.add('vk-locked');
        else el.classList.remove('vk-locked');
        sendKeyboardReport();
    }

    // 把某个键按下/抬起注入 UHID 键盘状态并上报
    function vkSendKey(keyName, isDown) {
        if (!window.uhidKeyboardEnabled) return;
        if (keyName === 'CTRL' || keyName === 'SHIFT' || keyName === 'ALT') return;
        const hid = VK_KEY[keyName];
        if (hid === undefined) return;
        if (isDown) pressedKeys.add(hid);
        else pressedKeys.delete(hid);
        sendKeyboardReport();
    }

    // 释放所有仍按下的虚拟键（隐藏面板时调用，避免卡键）
    function vkReleaseAll() {
        let changed = false;
        vkHeldKeys.forEach((k) => {
            const hid = VK_KEY[k];
            if (hid !== undefined && pressedKeys.has(hid)) {
                pressedKeys.delete(hid);
                changed = true;
            }
        });
        vkHeldKeys.clear();
        if (currentModifiers) {
            currentModifiers = 0;
            changed = true;
        }
        document.querySelectorAll('#virtual-keyboard .vk-key.vk-locked, #virtual-keyboard .vk-key.vk-down')
            .forEach((el) => el.classList.remove('vk-locked', 'vk-down'));
        if (changed) sendKeyboardReport();
    }

    function buildVirtualKeyboard() {
        vkInjectStyles();
        vkRoot = document.createElement('div');
        vkRoot.id = 'virtual-keyboard';

        // 拖动把手：整块面板可拖到不挡画面的位置
        const handle = document.createElement('div');
        handle.id = 'vk-drag';
        handle.textContent = '⋯⋯';
        vkRoot.appendChild(handle);

        const rows = document.createElement('div');
        rows.id = 'vk-rows';
        VK_ROWS.forEach((row) => {
            const rowEl = document.createElement('div');
            rowEl.className = 'vk-row';
            row.forEach((item) => rowEl.appendChild(vkKeyElement(item)));
            rows.appendChild(rowEl);
        });
        vkRoot.appendChild(rows);

        (document.body || document.documentElement).appendChild(vkRoot);
        vkSetupDrag(handle);
    }

    function vkSetupDrag(handle) {
        let dragging = false, startX = 0, startY = 0, startLeft = 0, startTop = 0;
        handle.addEventListener('pointerdown', (ev) => {
            dragging = true;
            const rect = vkRoot.getBoundingClientRect();
            startX = ev.clientX; startY = ev.clientY;
            startLeft = rect.left; startTop = rect.top;
            // 拖动后改用 left/top 定位，脱离底部居中
            vkRoot.style.transform = 'none';
            vkRoot.style.left = rect.left + 'px';
            vkRoot.style.top = rect.top + 'px';
            vkRoot.style.bottom = 'auto';
            handle.setPointerCapture && handle.setPointerCapture(ev.pointerId);
        });
        handle.addEventListener('pointermove', (ev) => {
            if (!dragging) return;
            const w = vkRoot.offsetWidth, h = vkRoot.offsetHeight;
            let left = startLeft + (ev.clientX - startX);
            let top = startTop + (ev.clientY - startY);
            left = Math.max(4, Math.min(window.innerWidth - w - 4, left));
            top = Math.max(4, Math.min(window.innerHeight - h - 4, top));
            vkRoot.style.left = left + 'px';
            vkRoot.style.top = top + 'px';
        });
        const stop = () => { dragging = false; };
        handle.addEventListener('pointerup', stop);
        handle.addEventListener('pointercancel', stop);
    }

    function showVirtualKeyboard() {
        if (!vkRoot) buildVirtualKeyboard();
        if (vkRoot) vkRoot.classList.add('vk-visible');
    }

    function hideVirtualKeyboard() {
        if (vkRoot) vkRoot.classList.remove('vk-visible');
        vkReleaseAll();
    }

    // 供其它模块调用（例如切到别的输入方式时强制收起）
    window.showVirtualKeyboard = showVirtualKeyboard;
    window.hideVirtualKeyboard = hideVirtualKeyboard;
})();
