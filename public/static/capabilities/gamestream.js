import { addControlButton } from './control_ui.js';

const TYPE_MOUSE = 0x01;
const TYPE_TOUCH = 0x02;
const TYPE_GAMEPAD = 0x11;

const MOUSE_DOWN = 0;
const MOUSE_UP = 1;
const MOUSE_MOVE = 2;

const TOUCH_MOVE = 0;
const TOUCH_DOWN = 1;
const TOUCH_UP = 2;

// Moonlight / XInput button bitmasks
const GS_BTN = {
    UP: 0x0001,
    DOWN: 0x0002,
    LEFT: 0x0004,
    RIGHT: 0x0008,
    START: 0x0010,
    BACK: 0x0020,
    L3: 0x0040,
    R3: 0x0080,
    LB: 0x0100,
    RB: 0x0200,
    HOME: 0x0400,
    A: 0x1000,
    B: 0x2000,
    X: 0x4000,
    Y: 0x8000,
};

let gsTouchEnabled = false;
let gsGamepadEnabled = false;
let gsPhysicalRaf = null;
let gsLastPhysicalSig = '';

const gsPadState = {
    controllerNumber: 0,
    activeMask: 0x01,
    buttons: 0,
    lt: 0,
    rt: 0,
    lx: 0,
    ly: 0,
    rx: 0,
    ry: 0,
};

function sendOrderedPacket(buf) {
    if (typeof window.sendDataChannelMessage === 'function' && window.dataChannelOrdered) {
        window.sendDataChannelMessage(window.dataChannelOrdered, buf);
    }
}

function sendUnorderedPacket(buf) {
    if (typeof window.sendDataChannelMessage === 'function' && window.dataChannelUnordered) {
        window.sendDataChannelMessage(window.dataChannelUnordered, buf);
    } else {
        sendOrderedPacket(buf);
    }
}

// =========================================================================
// 1. GameStream Trackpad (Default) & Touch Screen (Toggleable)
// =========================================================================

function sendMousePacket(action, dx, dy, buttons, wheelX, wheelY) {
    const buf = new ArrayBuffer(18);
    const v = new DataView(buf);
    v.setUint8(0, TYPE_MOUSE);
    v.setUint8(1, action);
    v.setInt32(2, Math.round(dx), false);
    v.setInt32(6, Math.round(dy), false);
    v.setUint32(10, buttons >>> 0, false);
    v.setInt16(14, Math.round(wheelX), false);
    v.setInt16(16, Math.round(wheelY), false);
    if (action === MOUSE_MOVE && wheelX === 0 && wheelY === 0) {
        sendUnorderedPacket(buf);
    } else {
        sendOrderedPacket(buf);
    }
}

function getVideoCoords(clientX, clientY) {
    if (typeof window.getScreenCoordinates === 'function') {
        return window.getScreenCoordinates(clientX, clientY);
    }
    const video = document.getElementById('remoteVideo');
    if (!video) return null;
    const rect = window.cachedRect && window.cachedRect.VideoRect ? window.cachedRect.VideoRect : video.getBoundingClientRect();
    const content = window.cachedRect && window.cachedRect.ContentRect && window.cachedRect.ContentRect.width > 0
        ? window.cachedRect.ContentRect
        : { left: 0, top: 0, width: rect.width, height: rect.height };
    const mediaW = (window.cachedRect && window.cachedRect.MediaWidth) || video.width || video.videoWidth || 1920;
    const mediaH = (window.cachedRect && window.cachedRect.MediaHeight) || video.height || video.videoHeight || 1080;

    const relX = clientX - rect.left - content.left;
    const relY = clientY - rect.top - content.top;
    if (relX < 0 || relY < 0 || relX > content.width || relY > content.height) return null;

    return {
        x: Math.max(0, Math.min(mediaW - 1, Math.round((relX / content.width) * mediaW))),
        y: Math.max(0, Math.min(mediaH - 1, Math.round((relY / content.height) * mediaH))),
    };
}

function sendTouchPacket(action, ptrId, x, y, pressure) {
    const buf = new ArrayBuffer(10);
    const v = new DataView(buf);
    v.setUint8(0, TYPE_TOUCH);
    v.setUint8(1, action);
    v.setUint8(2, ptrId & 0xFF);
    v.setUint16(3, x, false);
    v.setUint16(5, y, false);
    v.setUint16(7, pressure, false);
    v.setUint8(9, action === TOUCH_UP ? 0 : 1);
    if (action === TOUCH_MOVE) {
        sendUnorderedPacket(buf);
    } else {
        sendOrderedPacket(buf);
    }
}

const activeTouches = new Map();

function initGameStreamTouchAndTrackpad() {
    const video = document.getElementById('remoteVideo');
    if (!video) return;

    // --- Trackpad state (active when !gsTouchEnabled) ---
    const TRACKPAD_SENSITIVITY = 1.5;
    let tpMode = 'idle'; // 'idle' | 'one' | 'two'
    let tpStartTime = 0;
    let tpMovedDist = 0;
    let tpLastX = 0;
    let tpLastY = 0;
    let tpAccumX = 0;
    let tpAccumY = 0;
    let tpScrollAccumX = 0;
    let tpScrollAccumY = 0;

    const avgTwoFingers = (touches) => ({
        x: (touches[0].clientX + touches[1].clientX) / 2,
        y: (touches[0].clientY + touches[1].clientY) / 2,
    });

    const handleDirectTouch = (event, action) => {
        event.preventDefault();
        for (let i = 0; i < event.changedTouches.length; i++) {
            const t = event.changedTouches[i];
            let ptrId = activeTouches.get(t.identifier);
            if (action === TOUCH_DOWN) {
                ptrId = t.identifier % 10;
                activeTouches.set(t.identifier, ptrId);
            } else if (ptrId === undefined) {
                continue;
            }
            const coords = getVideoCoords(t.clientX, t.clientY);
            if (coords) {
                const force = typeof t.force === 'number' && t.force > 0 ? t.force : 1.0;
                const pressure = action === TOUCH_UP ? 0 : Math.min(65535, Math.round(force * 65535));
                sendTouchPacket(action, ptrId, coords.x, coords.y, pressure);
            }
            if (action === TOUCH_UP) {
                activeTouches.delete(t.identifier);
            }
        }
    };

    video.addEventListener('touchstart', (e) => {
        if (gsTouchEnabled) {
            handleDirectTouch(e, TOUCH_DOWN);
            return;
        }
        e.preventDefault();
        if (e.touches.length === 1) {
            tpMode = 'one';
            tpStartTime = Date.now();
            tpMovedDist = 0;
            tpAccumX = 0;
            tpAccumY = 0;
            tpLastX = e.touches[0].clientX;
            tpLastY = e.touches[0].clientY;
        } else if (e.touches.length === 2) {
            tpMode = 'two';
            tpStartTime = Date.now();
            tpMovedDist = 0;
            tpScrollAccumX = 0;
            tpScrollAccumY = 0;
            const avg = avgTwoFingers(e.touches);
            tpLastX = avg.x;
            tpLastY = avg.y;
        }
    }, { passive: false });

    video.addEventListener('touchmove', (e) => {
        if (gsTouchEnabled) {
            handleDirectTouch(e, TOUCH_MOVE);
            return;
        }
        e.preventDefault();
        if (tpMode === 'one' && e.touches.length === 1) {
            const dx = e.touches[0].clientX - tpLastX;
            const dy = e.touches[0].clientY - tpLastY;
            tpLastX = e.touches[0].clientX;
            tpLastY = e.touches[0].clientY;
            tpMovedDist += Math.hypot(dx, dy);

            tpAccumX += dx * TRACKPAD_SENSITIVITY;
            tpAccumY += dy * TRACKPAD_SENSITIVITY;
            const sendX = Math.trunc(tpAccumX);
            const sendY = Math.trunc(tpAccumY);
            if (sendX !== 0 || sendY !== 0) {
                tpAccumX -= sendX;
                tpAccumY -= sendY;
                sendMousePacket(MOUSE_MOVE, sendX, sendY, 0, 0, 0);
            }
        } else if (tpMode === 'two' && e.touches.length >= 2) {
            const avg = avgTwoFingers(e.touches);
            const dx = avg.x - tpLastX;
            const dy = avg.y - tpLastY;
            tpLastX = avg.x;
            tpLastY = avg.y;
            tpMovedDist += Math.hypot(dx, dy);

            tpScrollAccumX += dx / 24;
            tpScrollAccumY += dy / 24;
            const sx = Math.trunc(tpScrollAccumX);
            const sy = Math.trunc(tpScrollAccumY);
            if (sx !== 0 || sy !== 0) {
                tpScrollAccumX -= sx;
                tpScrollAccumY -= sy;
                sendMousePacket(MOUSE_MOVE, 0, 0, 0, sx, sy);
            }
        }
    }, { passive: false });

    const endTrackpadOrTouch = (e) => {
        if (gsTouchEnabled) {
            handleDirectTouch(e, TOUCH_UP);
            return;
        }
        e.preventDefault();
        const elapsed = Date.now() - tpStartTime;
        if (e.touches.length === 0) {
            if (tpMode === 'one' && elapsed < 250 && tpMovedDist < 8) {
                // Single-finger tap -> Left click
                sendMousePacket(MOUSE_DOWN, 0, 0, 1, 0, 0);
                sendMousePacket(MOUSE_UP, 0, 0, 1, 0, 0);
            } else if (tpMode === 'two' && elapsed < 300 && tpMovedDist < 12) {
                // Two-finger tap -> Right click
                sendMousePacket(MOUSE_DOWN, 0, 0, 2, 0, 0);
                sendMousePacket(MOUSE_UP, 0, 0, 2, 0, 0);
            }
            tpMode = 'idle';
        }
    };

    video.addEventListener('touchend', endTrackpadOrTouch, { passive: false });
    video.addEventListener('touchcancel', endTrackpadOrTouch, { passive: false });
}

// =========================================================================
// 2. GameStream Gamepad (Virtual Overlay + Physical Gamepad API)
// =========================================================================

function createGamepadPacket(state) {
    const buf = new ArrayBuffer(18);
    const v = new DataView(buf);
    v.setUint8(0, TYPE_GAMEPAD);
    v.setUint8(1, state.controllerNumber & 0x03);
    v.setUint16(2, state.activeMask, false);
    v.setUint32(4, state.buttons >>> 0, false);
    v.setUint8(8, state.lt & 0xFF);
    v.setUint8(9, state.rt & 0xFF);
    v.setInt16(10, state.lx, false);
    v.setInt16(12, state.ly, false);
    v.setInt16(14, state.rx, false);
    v.setInt16(16, state.ry, false);
    return buf;
}

function sendGamepadState(reliable = false) {
    if (!gsGamepadEnabled) return;
    const pkt = createGamepadPacket(gsPadState);
    if (reliable) {
        sendOrderedPacket(pkt);
    } else {
        sendUnorderedPacket(pkt);
    }
}

function disconnectGamepad() {
    const offState = {
        controllerNumber: 0,
        activeMask: 0,
        buttons: 0,
        lt: 0,
        rt: 0,
        lx: 0,
        ly: 0,
        rx: 0,
        ry: 0,
    };
    sendOrderedPacket(createGamepadPacket(offState));
}

function injectGamepadStyles() {
    if (document.getElementById('gs-gamepad-styles')) return;
    const style = document.createElement('style');
    style.id = 'gs-gamepad-styles';
    style.textContent = `
        #gs-virtual-gamepad {
            position: absolute;
            inset: 0;
            pointer-events: none;
            z-index: 25;
            display: flex;
            justify-content: space-between;
            align-items: flex-end;
            padding: 20px 28px;
            user-select: none;
            touch-action: none;
        }
        .gs-pad-group {
            pointer-events: auto;
            display: flex;
            flex-direction: column;
            align-items: center;
            gap: 14px;
        }
        .gs-pad-row {
            display: flex;
            gap: 10px;
            align-items: center;
        }
        .gs-pad-btn {
            width: 50px;
            height: 50px;
            border-radius: 50%;
            background: rgba(20, 22, 26, 0.65);
            border: 1.5px solid rgba(255, 255, 255, 0.25);
            color: #fff;
            font-size: 13px;
            font-weight: 700;
            display: flex;
            align-items: center;
            justify-content: center;
            backdrop-filter: blur(4px);
            touch-action: none;
        }
        .gs-pad-btn.active {
            background: rgba(168, 199, 250, 0.75);
            color: #062e6f;
            transform: scale(0.94);
        }
        .gs-pad-shoulder {
            width: 64px;
            height: 34px;
            border-radius: 17px;
            font-size: 12px;
        }
        .gs-pad-small {
            width: 54px;
            height: 28px;
            border-radius: 14px;
            font-size: 11px;
        }
        .gs-pad-abxy {
            display: grid;
            grid-template-columns: repeat(3, 46px);
            grid-template-rows: repeat(3, 46px);
            gap: 4px;
            place-items: center;
        }
        .gs-stick-base {
            width: 112px;
            height: 112px;
            border-radius: 50%;
            background: rgba(20, 22, 26, 0.45);
            border: 1.5px solid rgba(255, 255, 255, 0.22);
            position: relative;
            touch-action: none;
        }
        .gs-stick-knob {
            width: 46px;
            height: 46px;
            border-radius: 50%;
            background: rgba(255, 255, 255, 0.45);
            position: absolute;
            top: 50%;
            left: 50%;
            transform: translate(-50%, -50%);
            pointer-events: none;
        }
        .gs-dpad {
            display: grid;
            grid-template-columns: repeat(3, 38px);
            grid-template-rows: repeat(3, 38px);
            gap: 3px;
            place-items: center;
        }
        .gs-dpad .gs-pad-btn {
            width: 38px;
            height: 38px;
            border-radius: 10px;
            font-size: 12px;
        }
    `;
    document.head.appendChild(style);
}

function bindPadButton(el, bit, triggerKey) {
    const down = (e) => {
        e.preventDefault();
        e.stopPropagation();
        el.classList.add('active');
        if (bit) gsPadState.buttons |= bit;
        if (triggerKey) gsPadState[triggerKey] = 255;
        sendGamepadState(true);
    };
    const up = (e) => {
        e.preventDefault();
        e.stopPropagation();
        el.classList.remove('active');
        if (bit) gsPadState.buttons &= ~bit;
        if (triggerKey) gsPadState[triggerKey] = 0;
        sendGamepadState(true);
    };
    el.addEventListener('touchstart', down, { passive: false });
    el.addEventListener('touchend', up, { passive: false });
    el.addEventListener('touchcancel', up, { passive: false });
    el.addEventListener('mousedown', down);
    el.addEventListener('mouseup', up);
    el.addEventListener('mouseleave', up);
}

function makePadBtn(label, className, bit, triggerKey) {
    const b = document.createElement('button');
    b.type = 'button';
    b.className = `gs-pad-btn ${className || ''}`;
    b.textContent = label;
    bindPadButton(b, bit, triggerKey);
    return b;
}

function makeStick(isLeft) {
    const base = document.createElement('div');
    base.className = 'gs-stick-base';
    const knob = document.createElement('div');
    knob.className = 'gs-stick-knob';
    base.appendChild(knob);

    let activeId = null;
    const maxDist = 36;

    const update = (clientX, clientY) => {
        const rect = base.getBoundingClientRect();
        const cx = rect.left + rect.width / 2;
        const cy = rect.top + rect.height / 2;
        let dx = clientX - cx;
        let dy = clientY - cy;
        const dist = Math.hypot(dx, dy);
        if (dist > maxDist) {
            dx = (dx / dist) * maxDist;
            dy = (dy / dist) * maxDist;
        }
        knob.style.transform = `translate(calc(-50% + ${dx}px), calc(-50% + ${dy}px))`;

        const nx = Math.round((dx / maxDist) * 32767);
        // GameStream / XInput Y axis is positive UP
        const ny = Math.round((-dy / maxDist) * 32767);
        if (isLeft) {
            gsPadState.lx = nx;
            gsPadState.ly = ny;
        } else {
            gsPadState.rx = nx;
            gsPadState.ry = ny;
        }
        sendGamepadState(false);
    };

    const reset = () => {
        activeId = null;
        knob.style.transform = 'translate(-50%, -50%)';
        if (isLeft) {
            gsPadState.lx = 0;
            gsPadState.ly = 0;
        } else {
            gsPadState.rx = 0;
            gsPadState.ry = 0;
        }
        sendGamepadState(true);
    };

    base.addEventListener('touchstart', (e) => {
        e.preventDefault();
        e.stopPropagation();
        if (e.changedTouches.length > 0) {
            activeId = e.changedTouches[0].identifier;
            update(e.changedTouches[0].clientX, e.changedTouches[0].clientY);
        }
    }, { passive: false });

    base.addEventListener('touchmove', (e) => {
        e.preventDefault();
        e.stopPropagation();
        for (let i = 0; i < e.changedTouches.length; i++) {
            if (e.changedTouches[i].identifier === activeId) {
                update(e.changedTouches[i].clientX, e.changedTouches[i].clientY);
            }
        }
    }, { passive: false });

    const endTouch = (e) => {
        e.preventDefault();
        e.stopPropagation();
        for (let i = 0; i < e.changedTouches.length; i++) {
            if (e.changedTouches[i].identifier === activeId) {
                reset();
            }
        }
    };
    base.addEventListener('touchend', endTouch, { passive: false });
    base.addEventListener('touchcancel', endTouch, { passive: false });

    return base;
}

function showVirtualGamepad() {
    if (document.getElementById('gs-virtual-gamepad')) return;
    injectGamepadStyles();
    const container = document.querySelector('.video-container') || document.body;

    const root = document.createElement('div');
    root.id = 'gs-virtual-gamepad';

    // Left group: LT/LB + D-Pad + Left Stick
    const leftGroup = document.createElement('div');
    leftGroup.className = 'gs-pad-group';
    const leftShoulders = document.createElement('div');
    leftShoulders.className = 'gs-pad-row';
    leftShoulders.appendChild(makePadBtn('LT', 'gs-pad-shoulder', 0, 'lt'));
    leftShoulders.appendChild(makePadBtn('LB', 'gs-pad-shoulder', GS_BTN.LB));

    const dpad = document.createElement('div');
    dpad.className = 'gs-dpad';
    dpad.appendChild(document.createElement('div'));
    dpad.appendChild(makePadBtn('▲', '', GS_BTN.UP));
    dpad.appendChild(document.createElement('div'));
    dpad.appendChild(makePadBtn('◀', '', GS_BTN.LEFT));
    dpad.appendChild(document.createElement('div'));
    dpad.appendChild(makePadBtn('▶', '', GS_BTN.RIGHT));
    dpad.appendChild(document.createElement('div'));
    dpad.appendChild(makePadBtn('▼', '', GS_BTN.DOWN));
    dpad.appendChild(document.createElement('div'));

    leftGroup.appendChild(leftShoulders);
    leftGroup.appendChild(dpad);
    leftGroup.appendChild(makeStick(true));

    // Center group: Back / Home / Start
    const centerGroup = document.createElement('div');
    centerGroup.className = 'gs-pad-group';
    const centerRow = document.createElement('div');
    centerRow.className = 'gs-pad-row';
    centerRow.appendChild(makePadBtn('Back', 'gs-pad-small', GS_BTN.BACK));
    centerRow.appendChild(makePadBtn('Home', 'gs-pad-small', GS_BTN.HOME));
    centerRow.appendChild(makePadBtn('Start', 'gs-pad-small', GS_BTN.START));
    centerGroup.appendChild(centerRow);

    // Right group: RB/RT + ABXY + Right Stick
    const rightGroup = document.createElement('div');
    rightGroup.className = 'gs-pad-group';
    const rightShoulders = document.createElement('div');
    rightShoulders.className = 'gs-pad-row';
    rightShoulders.appendChild(makePadBtn('RB', 'gs-pad-shoulder', GS_BTN.RB));
    rightShoulders.appendChild(makePadBtn('RT', 'gs-pad-shoulder', 0, 'rt'));

    const abxy = document.createElement('div');
    abxy.className = 'gs-pad-abxy';
    abxy.appendChild(document.createElement('div'));
    abxy.appendChild(makePadBtn('Y', '', GS_BTN.Y));
    abxy.appendChild(document.createElement('div'));
    abxy.appendChild(makePadBtn('X', '', GS_BTN.X));
    abxy.appendChild(document.createElement('div'));
    abxy.appendChild(makePadBtn('B', '', GS_BTN.B));
    abxy.appendChild(document.createElement('div'));
    abxy.appendChild(makePadBtn('A', '', GS_BTN.A));
    abxy.appendChild(document.createElement('div'));

    rightGroup.appendChild(rightShoulders);
    rightGroup.appendChild(abxy);
    rightGroup.appendChild(makeStick(false));

    root.appendChild(leftGroup);
    root.appendChild(centerGroup);
    root.appendChild(rightGroup);
    container.appendChild(root);
}

function hideVirtualGamepad() {
    const el = document.getElementById('gs-virtual-gamepad');
    if (el) el.remove();
}

function pollPhysicalGamepads() {
    if (!gsGamepadEnabled) return;
    if (typeof navigator.getGamepads === 'function') {
        const pads = navigator.getGamepads();
        const pad = pads && pads[0];
        if (pad && pad.connected) {
            const btnMap = [
                [0, GS_BTN.A], [1, GS_BTN.B], [2, GS_BTN.X], [3, GS_BTN.Y],
                [4, GS_BTN.LB], [5, GS_BTN.RB],
                [8, GS_BTN.BACK], [9, GS_BTN.START],
                [10, GS_BTN.L3], [11, GS_BTN.R3],
                [12, GS_BTN.UP], [13, GS_BTN.DOWN], [14, GS_BTN.LEFT], [15, GS_BTN.RIGHT],
                [16, GS_BTN.HOME],
            ];
            let buttons = 0;
            for (const [idx, flag] of btnMap) {
                if (pad.buttons[idx] && pad.buttons[idx].pressed) {
                    buttons |= flag;
                }
            }
            const lt = pad.buttons[6] ? Math.round(pad.buttons[6].value * 255) : 0;
            const rt = pad.buttons[7] ? Math.round(pad.buttons[7].value * 255) : 0;
            const axis = (v) => (Math.abs(v || 0) < 0.08 ? 0 : Math.round(Math.max(-1, Math.min(1, v)) * 32767));
            const lx = axis(pad.axes[0]);
            const ly = axis(-(pad.axes[1] || 0));
            const rx = axis(pad.axes[2]);
            const ry = axis(-(pad.axes[3] || 0));

            const sig = `${buttons},${lt},${rt},${lx},${ly},${rx},${ry}`;
            if (sig !== gsLastPhysicalSig) {
                gsLastPhysicalSig = sig;
                gsPadState.buttons = buttons;
                gsPadState.lt = lt;
                gsPadState.rt = rt;
                gsPadState.lx = lx;
                gsPadState.ly = ly;
                gsPadState.rx = rx;
                gsPadState.ry = ry;
                sendGamepadState(false);
            }
        }
    }
    gsPhysicalRaf = requestAnimationFrame(pollPhysicalGamepads);
}

// =========================================================================
// 3. Toolbar Controls
// =========================================================================

initGameStreamTouchAndTrackpad();

addControlButton(
    'gamestreamTouchToggleBtn',
    'Touch',
    'M9 11.24V7.5C9 6.12 10.12 5 11.5 5S14 6.12 14 7.5v3.74c1.21-.81 2-2.18 2-3.74C16 5.01 13.99 3 11.5 3S7 5.01 7 7.5c0 1.56.79 2.93 2 3.74zm9.84 4.63l-4.54-2.26c-.17-.07-.35-.11-.54-.11H13v-6c0-.83-.67-1.5-1.5-1.5S10 6.67 10 7.5v10.74l-3.43-.72c-.08-.01-.15-.03-.24-.03-.31 0-.59.13-.79.33l-.79.8 4.94 4.94c.27.27.65.44 1.06.44h6.79c.75 0 1.33-.55 1.44-1.28l.75-5.27c.01-.07.02-.14.02-.2 0-.62-.38-1.16-.91-1.38z'
).addEventListener('click', () => {
    gsTouchEnabled = !gsTouchEnabled;
    const btn = document.getElementById('gamestreamTouchToggleBtn');
    if (btn) btn.classList.toggle('active', gsTouchEnabled);
    if (typeof showToast === 'function') {
        showToast(
            gsTouchEnabled
                ? (i18n.t('gamestream_touch_on') || '触控模式已开启')
                : (i18n.t('gamestream_trackpad_on') || '触摸板模式已开启'),
            1500
        );
    }
});

addControlButton(
    'gamestreamGamepadToggleBtn',
    'Gamepad',
    'M21 6H3c-1.1 0-2 .9-2 2v8c0 1.1.9 2 2 2h18c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2zm-10 7H8v3H6v-3H3v-2h3V8h2v3h3v2zm4.5 2c-.83 0-1.5-.67-1.5-1.5s.67-1.5 1.5-1.5 1.5.67 1.5 1.5-.67 1.5-1.5 1.5zm4-3c-.83 0-1.5-.67-1.5-1.5S18.67 9 19.5 9s1.5.67 1.5 1.5-.67 1.5-1.5 1.5z'
).addEventListener('click', () => {
    gsGamepadEnabled = !gsGamepadEnabled;
    const btn = document.getElementById('gamestreamGamepadToggleBtn');
    if (btn) btn.classList.toggle('active', gsGamepadEnabled);
    if (gsGamepadEnabled) {
        gsPadState.activeMask = 0x01;
        showVirtualGamepad();
        sendGamepadState(true);
        pollPhysicalGamepads();
    } else {
        if (gsPhysicalRaf) {
            cancelAnimationFrame(gsPhysicalRaf);
            gsPhysicalRaf = null;
        }
        hideVirtualGamepad();
        disconnectGamepad();
    }
});
