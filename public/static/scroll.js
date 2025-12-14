
const videoElementScroll = document.getElementById('remoteVideo');

// 节流函数，避免发送过于频繁的滚动事件
const handleWheel = throttle((event) => {
    // 阻止默认的页面滚动行为
    event.preventDefault();

    const coords = getScreenCoordinates(event);
    if (!coords) return;

    // Scrcpy 协议中，滚动值通常是 +1/-1 或更大的整数
    // event.deltaY > 0 表示向下滚动，对应 Scrcpy 的负值 (通常)
    // 但 Scrcpy 的 ScrollEvent 定义：
    // hScroll: horizontal scroll amount (-1: left, 1: right)
    // vScroll: vertical scroll amount (-1: down, 1: up)
    
    // 浏览器 WheelEvent:
    // deltaY > 0: 向下滚动 (用户手指向上滑)
    // deltaY < 0: 向上滚动 (用户手指向下滑)

    // 转换逻辑:
    // deltaY > 0 (Down) -> vScroll = -1
    // deltaY < 0 (Up)   -> vScroll = 1

    // 增加滚动敏感度
    const SCROLL_SCALE = 30; // 调整此值以改变滚动灵敏度

    let hScroll = 0;
    let vScroll = 0;

    hScroll = Math.round(event.deltaX * SCROLL_SCALE);
    vScroll = -Math.round(event.deltaY * SCROLL_SCALE);

    if (hScroll === 0 && vScroll === 0) return;

    // 发送滚动包
    // 注意：我们需要当前的鼠标位置 (x, y)
    const packet = createScrollPacket(coords.x, coords.y, hScroll, vScroll);
    // console.log(`Sending scroll event at (${coords.x}, ${coords.y}): hScroll=${hScroll}, vScroll=${vScroll}`);
    
    if (window.ws && window.ws.readyState === WebSocket.OPEN) {
        window.ws.send(packet);
    }

}, 16); // 16ms 节流

videoElementScroll.addEventListener('wheel', handleWheel, { passive: false });
