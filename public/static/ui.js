/**
 * 显示顶部 Toast 提示消息
 * @param {string} message 消息文本
 * @param {number} duration 持续时间(ms)
 */
function showToast(message, duration = 3000) {
    let toast = document.getElementById('webcpy-toast');
    
    if (!toast) {
        toast = document.createElement('div');
        toast.id = 'webcpy-toast';
        
        // 注入样式
        Object.assign(toast.style, {
            position: 'fixed',
            top: '-60px', // 初始位置在屏幕外
            left: '50%',
            transform: 'translateX(-50%)',
            backgroundColor: 'rgba(33, 33, 33, 0.95)',
            color: '#ffffff',
            padding: '12px 24px',
            borderRadius: '8px',
            boxShadow: '0 4px 12px rgba(0,0,0,0.2)',
            zIndex: '10000',
            fontSize: '14px',
            fontWeight: '500',
            pointerEvents: 'none',
            transition: 'top 0.4s cubic-bezier(0.22, 1, 0.36, 1), opacity 0.4s ease',
            opacity: '0',
            whiteSpace: 'nowrap',
            maxWidth: '90%',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            display: 'flex',
            alignItems: 'center',
            gap: '8px'
        });
        
        document.body.appendChild(toast);
    }

    toast.textContent = message;
    
    // 显示动画
    requestAnimationFrame(() => {
        toast.style.top = '24px'; // 下滑出现
        toast.style.opacity = '1';
    });

    // 清除旧定时器
    if (toast.timeoutId) {
        clearTimeout(toast.timeoutId);
    }

    // 自动隐藏
    toast.timeoutId = setTimeout(() => {
        toast.style.top = '-60px'; // 上滑消失
        toast.style.opacity = '0';
    }, duration);
}