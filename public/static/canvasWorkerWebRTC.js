// 注意参数是 e，数据在 e.data 中
self.onmessage = async function (e) {
  const { readable, canvas } = e.data;
  const ctx = canvas.getContext('2d');
  const reader = readable.getReader();

  // 在 Worker 里放心跑 while 循环读取视频帧，完全不阻塞主线程！
  while (true) {
    const { done, value: frame } = await reader.read();
    if (done) break;

    // 同步 canvas 宽高与视频帧一致
    if (canvas.width !== frame.displayWidth || canvas.height !== frame.displayHeight) {
      canvas.width = frame.displayWidth;
      canvas.height = frame.displayHeight;
    }

    // 把视频帧画到 OffscreenCanvas 上（页面上的 canvas 会自动同步显示）
    ctx.drawImage(frame, 0, 0);

    // ⚠️ 极其重要：用完必须手动关闭 frame，否则瞬间内存泄漏卡死！
    frame.close();
  }
};