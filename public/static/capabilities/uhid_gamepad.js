/**
 * UHID Virtual Gamepad
 * 集成虚拟摇杆 UI 与 UHID 通信协议
 */

(function() {
    // --- 1. UHID 协议常量与定义 ---

    const UHID_GAMEPAD_MSG_CREATE = 12;
    const UHID_GAMEPAD_MSG_INPUT = 13;
    const UHID_GAMEPAD_MSG_DESTROY = 14;

    const UHID_GAMEPAD_ID = 4;
    const UHID_GAMEPAD_NAME = "Virtual Gamepad";

    // 状态变量
    let uhidGamepadEnabled = false;
    let uhidGamepadInitialized = false;

    // 手柄状态 (Buttons: 32 bits, Axes: 4 bytes)
    let gamepadState = {
        buttons: 0,
        x: 0,
        y: 0,
        z: 0,
        rz: 0
    };

    // 按键映射 (Bit positions)
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
    const BTN_MODE = 12;   // Home/Guide
    const BTN_THUMBL = 13; // L3
    const BTN_THUMBR = 14; // R3
    // 15-31 预留给更多按键

    // HID 描述符 (扩容至 32 个按键)
    const GAMEPAD_REPORT_DESCRIPTOR = new Uint8Array([
        0x05, 0x01,        // Usage Page (Generic Desktop Ctrls)
        0x09, 0x05,        // Usage (Game Pad)
        0xA1, 0x01,        // Collection (Application)
        0xA1, 0x00,        //   Collection (Physical)
        
        // Buttons (32 buttons)
        0x05, 0x09,        //     Usage Page (Button)
        0x19, 0x01,        //     Usage Minimum (0x01)
        0x29, 0x20,        //     Usage Maximum (0x20 = 32) <--- 修改点
        0x15, 0x00,        //     Logical Minimum (0)
        0x25, 0x01,        //     Logical Maximum (1)
        0x75, 0x01,        //     Report Size (1)
        0x95, 0x20,        //     Report Count (32)         <--- 修改点
        0x81, 0x02,        //     Input (Data,Var,Abs)
        
        // Axes (4 axes: X, Y, Z, Rz)
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

    // --- 2. 核心通信函数 ---

    function initUHIDGamepad() {
        if (uhidGamepadInitialized) return;

        // 尝试先销毁旧设备 (如果存在)
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

    // 暴露给外部调用的开关函数
    window.toggleUHIDGamepad = function() {
        const btn = document.getElementById('uhidGamepadToggleBtn');
        
        if (!uhidGamepadEnabled) {
            initUHIDGamepad();
            uhidGamepadEnabled = true;
            createVirtualGamepadUI();
            console.log("UHID Gamepad enabled");
            if (btn) btn.classList.add('active');
        } else {
            removeVirtualGamepadUI();
            destroyUHIDGamepad();
            uhidGamepadEnabled = false;
            console.log("UHID Gamepad disabled");
            if (btn) btn.classList.remove('active');
        }
    };

    function sendGamepadReport() {
        if (!uhidGamepadEnabled || !uhidGamepadInitialized) return;

        const packet = createUHIDGamepadInputPacket(gamepadState);
        // if (window.ws && window.ws.readyState === WebSocket.OPEN) {
        //     window.ws.send(packet);
        // }
        sendDataChannelMessage(DATA_CHANNEL_UNORDERED, packet);
    }

    // --- 3. 协议包构造函数 ---

    function createUHIDGamepadCreatePacket() {
        const encoder = new TextEncoder();
        const rawName = UHID_GAMEPAD_NAME;
        const nameBytes = encoder.encode(rawName).slice(0, 255);
        const descriptor = GAMEPAD_REPORT_DESCRIPTOR;

        // Type(1) + ID(2) + Vendor(2) + Product(2) + NameLen(1) + NameBytes + DescLen(2) + DescBytes
        const buffer = new ArrayBuffer(8 + nameBytes.length + 2 + descriptor.length);
        const view = new DataView(buffer);
        const uint8View = new Uint8Array(buffer);

        let offset = 0;
        view.setUint8(offset, UHID_GAMEPAD_MSG_CREATE); offset += 1;
        view.setUint16(offset, UHID_GAMEPAD_ID); offset += 2;
        view.setUint16(offset, 0x18d1); offset += 2; // Vendor: Google
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
        // Report Size: Buttons(4 bytes) + Axes(4 bytes) = 8 bytes
        const reportSize = 8;
        const buffer = new ArrayBuffer(1 + 2 + 2 + reportSize);
        const view = new DataView(buffer);

        let offset = 0;
        view.setUint8(offset, UHID_GAMEPAD_MSG_INPUT); offset += 1;
        view.setUint16(offset, UHID_GAMEPAD_ID); offset += 2;
        view.setUint16(offset, reportSize); offset += 2;

        // HID Report Data
        // Buttons: 32 bits (4 bytes)
        view.setUint32(offset, state.buttons, true); offset += 4; 
        
        // Axes: 4 bytes
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

    // --- 4. 虚拟手柄 UI 构建 ---

    function createVirtualGamepadUI() {
        // 查找视频容器，如果没有则挂载到 body
        let container = document.getElementById('video-container'); // 假设你的视频容器ID
        if (!container) container = document.body;

        // 注入 CSS 样式
        injectStyles();

        const gamepadDiv = document.createElement('div');
        gamepadDiv.id = 'virtual-gamepad';
        
        // --- HTML 结构构建 ---
        
        // 1. 左侧摇杆
        const leftControls = document.createElement('div');
        leftControls.className = 'gamepad-controls left';
        leftControls.id = 'joystick-base';
        const stick = document.createElement('div');
        stick.className = 'joystick-stick';
        stick.id = 'joystick-stick';
        leftControls.appendChild(stick);

        // 2. 中间功能键 (L3, MENU, R3)
        const centerControls = document.createElement('div');
        centerControls.className = 'gamepad-controls center';
        
        const btnL3 = createButton('L3', 'gp-btn btn-stick', '50px', '50px', 'L3', true);
        const btnHome = createButton('MENU', 'gp-btn btn-home', '60px', '40px', 'HOME', true);
        const btnR3 = createButton('R3', 'gp-btn btn-stick', '50px', '50px', 'R3', true);
        
        // 绑定事件
        setupActionButton(btnL3, BTN_THUMBL); // L3 -> THUMBL
        setupActionButton(btnHome, BTN_MODE);   // MENU -> MODE
        setupActionButton(btnR3, BTN_THUMBR); // R3 -> THUMBR

        centerControls.appendChild(btnL3);
        centerControls.appendChild(btnHome);
        centerControls.appendChild(btnR3);

        // 3. 右侧 ABXY
        const rightControls = document.createElement('div');
        rightControls.className = 'gamepad-controls right';

        // 布局位置 (相对于 180x180 容器)
        const btnY = createButton('Y', 'gp-btn btn-y', '60px', '60px', 'Y');
        Object.assign(btnY.style, { top: '0', left: '60px' });
        
        const btnA = createButton('A', 'gp-btn btn-a', '60px', '60px', 'A');
        Object.assign(btnA.style, { top: '120px', left: '60px' });
        
        const btnX = createButton('X', 'gp-btn btn-x', '60px', '60px', 'X');
        Object.assign(btnX.style, { top: '60px', left: '0' });
        
        const btnB = createButton('B', 'gp-btn btn-b', '60px', '60px', 'B');
        Object.assign(btnB.style, { top: '60px', left: '120px' });

        setupActionButton(btnA, BTN_A);
        setupActionButton(btnB, BTN_B);
        setupActionButton(btnX, BTN_X);
        setupActionButton(btnY, BTN_Y);

        rightControls.appendChild(btnY);
        rightControls.appendChild(btnA);
        rightControls.appendChild(btnX);
        rightControls.appendChild(btnB);

        // 组装
        gamepadDiv.appendChild(leftControls);
        gamepadDiv.appendChild(centerControls);
        gamepadDiv.appendChild(rightControls);

        container.appendChild(gamepadDiv);

        // 初始化摇杆逻辑
        initJoystickLogic(leftControls, stick);
    }

    function removeVirtualGamepadUI() {
        const gamepadDiv = document.getElementById('virtual-gamepad');
        if (gamepadDiv) {
            gamepadDiv.remove();
        }
        // 移除注入的样式? 通常没必要，留着也无妨
    }

    function createButton(text, className, w, h, keyName, releativePosition=false) {
        const btn = document.createElement('div');
        btn.innerText = text;
        btn.className = className;
        btn.style.width = w;
        btn.style.height = h;
        if (releativePosition) {
            btn.style.position = 'relative';
        }
        // btn.style.position = 'absolute';
        if (keyName) btn.setAttribute('data-key', keyName);
        return btn;
    }

    function injectStyles() {
        if (document.getElementById('gamepad-styles')) return;
        const style = document.createElement('style');
        style.id = 'gamepad-styles';
        style.innerHTML = `
            #virtual-gamepad {
                position: absolute; top: 0; left: 0; width: 100%; height: 100%;
                pointer-events: none; z-index: 100;
                display: flex; justify-content: space-between; align-items: flex-end;
                padding: 40px; box-sizing: border-box;
                user-select: none; touch-action: none;
            }
            .gamepad-controls { pointer-events: auto; position: relative; }
            .gamepad-controls.left {
                width: 180px; height: 180px;
                background: rgba(255, 255, 255, 0.1); border-radius: 50%;
                margin-bottom: 20px; backdrop-filter: blur(2px);
                border: 2px solid rgba(255,255,255,0.15);
                display: flex; justify-content: center; align-items: center;
            }
            .gamepad-controls.right { width: 180px; height: 180px; margin-bottom: 20px; }
            .gamepad-controls.center {
                position: absolute; bottom: 30px; left: 50%; transform: translateX(-50%);
                display: flex; gap: 20px; pointer-events: auto;
            }
            .joystick-stick {
                width: 80px; height: 80px;
                background: radial-gradient(circle at 30% 30%, #555, #222);
                border-radius: 50%; border: 2px solid rgba(255,255,255,0.3);
                box-shadow: 0 5px 15px rgba(0,0,0,0.5);
                position: absolute; transform: translate(0px, 0px); cursor: pointer;
                transition: transform 0.1s;
            }
            .joystick-stick.active { background: radial-gradient(circle at 30% 30%, #666, #333); transition: none; }
            .gp-btn {
                position: absolute; display: flex; justify-content: center; align-items: center;
                color: white; font-weight: bold; cursor: pointer;
                border: 1px solid rgba(255, 255, 255, 0.3);
                background-color: rgba(255, 255, 255, 0.15);
                border-radius: 50%; transition: transform 0.1s;
                box-shadow: 0 4px 6px rgba(0,0,0,0.3);
                text-shadow: 0 1px 2px rgba(0,0,0,0.5);
            }
            .gp-btn:active, .gp-btn.active { transform: scale(0.95); background-color: rgba(255, 255, 255, 0.4) !important; }
            .btn-y { background-color: rgba(255, 200, 0, 0.25); color: #ffd700; }
            .btn-a { background-color: rgba(0, 255, 100, 0.25); color: #00ff66; }
            .btn-x { background-color: rgba(0, 100, 255, 0.25); color: #00bfff; }
            .btn-b { background-color: rgba(255, 50, 50, 0.25); color: #ff4444; }
            .btn-home { background-color: rgba(255, 255, 255, 0.1); border-radius: 12px; font-size: 12px;}
            .btn-stick { background-color: rgba(200, 200, 200, 0.2); font-size: 12px;}
        `;
        document.head.appendChild(style);
    }

    // --- 5. 事件绑定逻辑 ---

    function setupActionButton(element, btnIndex) {
        const handleDown = (e) => {
            e.preventDefault();
            element.classList.add('active');
            if (navigator.vibrate) navigator.vibrate(10);
            
            gamepadState.buttons |= (1 << btnIndex);
            sendGamepadReport();
        };
        const handleUp = (e) => {
            e.preventDefault();
            element.classList.remove('active');
            
            gamepadState.buttons &= ~(1 << btnIndex);
            sendGamepadReport();
        };

        element.addEventListener('mousedown', handleDown);
        element.addEventListener('mouseup', handleUp);
        element.addEventListener('mouseleave', handleUp);
        element.addEventListener('touchstart', handleDown);
        element.addEventListener('touchend', handleUp);
    }

    function initJoystickLogic(base, stick) {
        let isDragging = false;
        const maxDistance = 50; // px

        function getCoords(e) {
            if (e.touches && e.touches.length > 0) return { x: e.touches[0].clientX, y: e.touches[0].clientY };
            return { x: e.clientX, y: e.clientY };
        }

        function updateJoystick(e, centerX, centerY) {
            const coords = getCoords(e);
            let dx = coords.x - centerX;
            let dy = coords.y - centerY;
            
            const distance = Math.sqrt(dx * dx + dy * dy);
            if (distance > maxDistance) {
                const ratio = maxDistance / distance;
                dx *= ratio;
                dy *= ratio;
            }
            
            stick.style.transform = `translate(${dx}px, ${dy}px)`;
            
            // 转换为 -127 ~ 127
            // 注意: 游戏手柄通常 Y 轴向上为负，向下为正，与屏幕坐标一致
            gamepadState.x = Math.round((dx / maxDistance) * 127);
            gamepadState.y = Math.round((dy / maxDistance) * 127);
            
            sendGamepadReport();
        }

        function start(e) {
            e.preventDefault();
            isDragging = true;
            stick.classList.add('active');
            const rect = base.getBoundingClientRect();
            const centerX = rect.left + rect.width / 2;
            const centerY = rect.top + rect.height / 2;
            updateJoystick(e, centerX, centerY);
        }

        function move(e) {
            if (!isDragging) return;
            e.preventDefault();
            const rect = base.getBoundingClientRect();
            updateJoystick(e, rect.left + rect.width / 2, rect.top + rect.height / 2);
        }

        function end(e) {
            e.preventDefault();
            isDragging = false;
            stick.classList.remove('active');
            stick.style.transform = `translate(0px, 0px)`;
            
            gamepadState.x = 0;
            gamepadState.y = 0;
            sendGamepadReport();
        }

        base.addEventListener('mousedown', start);
        document.addEventListener('mousemove', move);
        document.addEventListener('mouseup', end);

        base.addEventListener('touchstart', start);
        base.addEventListener('touchmove', move);
        base.addEventListener('touchend', end);
        base.addEventListener('touchcancel', end);
    }

})();