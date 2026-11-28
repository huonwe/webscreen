import { getRemoteCanvas } from '../screen.js';

// 返回 0 ~ 65535 的归一化坐标，彻底无视 canvas 分辨率和 CSS 缩放
export function NormalizeCoords(clientX, clientY) {
    const rect = getRemoteCanvas().getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) return null;

    // 情况 A：如果你已经通过 CSS/JS 把 <canvas> 标签本身按视频比例缩放居中（黑边在 canvas 外面）
    // 那么 canvas 内部没有黑边，直接用 rect 算即可：
    const relX = clientX - rect.left;
    const relY = clientY - rect.top;
    if (relX < 0 || relY < 0 || relX > rect.width || relY > rect.height) return null;

    return {
        x: Math.max(0, Math.min(65535, Math.round((relX / rect.width) * 65535))),
        y: Math.max(0, Math.min(65535, Math.round((relY / rect.height) * 65535))),
    };
}