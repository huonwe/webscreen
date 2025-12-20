
const TYPE_SCROLL = 0x03; // scroll event

const videoElementScroll = document.getElementById('remoteVideo');
const SCROLL_SCALE = 30; // 调整此值以改变滚动灵敏度

// 使用 requestAnimationFrame 批量处理滚动事件
let pendingScroll = { x: 0, y: 0, hScroll: 0, vScroll: 0 };
let scrollRafScheduled = false;
let lastScrollCoords = null;

function sendPendingScroll() {
    if (pendingScroll.hScroll !== 0 || pendingScroll.vScroll !== 0) {
        const packet = createScrollPacket(
            pendingScroll.x, 
            pendingScroll.y, 
            pendingScroll.hScroll, 
            pendingScroll.vScroll
        );
        
        if (window.ws && window.ws.readyState === WebSocket.OPEN) {
            window.ws.send(packet);
        }
        
        // 重置累积的滚动量
        pendingScroll.hScroll = 0;
        pendingScroll.vScroll = 0;
    }
}

const handleWheel = (event) => {
    // 阻止默认的页面滚动行为
    event.preventDefault();

    const coords = getScreenCoordinates(event.clientX, event.clientY);
    if (!coords) return;

    // 缓存坐标位置，用于后续批量发送
    lastScrollCoords = coords;
    
    // 增加滚动敏感度

    // 累积滚动量
    pendingScroll.x = coords.x;
    pendingScroll.y = coords.y;
    pendingScroll.hScroll += Math.round(event.deltaX * SCROLL_SCALE);
    pendingScroll.vScroll += -Math.round(event.deltaY * SCROLL_SCALE);

    // 使用 requestAnimationFrame 批量发送，减少消息数量
    if (!scrollRafScheduled) {
        scrollRafScheduled = true;
        requestAnimationFrame(() => {
            scrollRafScheduled = false;
            sendPendingScroll();
        });
    }
};

videoElementScroll.addEventListener('wheel', handleWheel, { passive: false });


function createScrollPacket(x, y, hScroll, vScroll) {
    // Scroll Packet Structure (Custom for WebSocket, will be converted to Scrcpy format on server):
    // 0: Type (0x03)
    // 1-2: X (uint16)
    // 3-4: Y (uint16)
    // 5-6: hScroll (int16)
    // 7-8: vScroll (int16)
    // 9: Buttons (uint8)
    const buffer = new ArrayBuffer(10);
    const view = new DataView(buffer);
    view.setUint8(0, TYPE_SCROLL);
    view.setUint16(1, x);
    view.setUint16(3, y);
    view.setInt16(5, hScroll);
    view.setInt16(7, vScroll);
    view.setUint8(9, 0); // No buttons pressed
    // console.log("Created Scroll Packet:", {x, y, hScroll, vScroll});
    return buffer;
}