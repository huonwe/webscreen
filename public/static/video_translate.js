/**
 * Video Real-time OCR & Translator
 * 实时屏幕 OCR 与翻译模块
 */

(function() {
    // --- 配置项 ---
    const CONFIG = {
        videoElementId: 'remoteVideo',
        ocrLanguage: 'jpn', // 源语言: 'eng' (英语), 'chi_sim' (简体中文), 'jpn' (日语)等
        interval: 3000,     // OCR 采样间隔 (毫秒)，太快会卡顿
        confidenceThreshold: 70, // 置信度阈值，低于此值的文字不显示
    };

    // --- 状态变量 ---
    let isTranslating = false;
    let worker = null;
    let timer = null;
    let overlayContainer = null;

    // --- 初始化 UI ---
    function initTranslatorUI() {
        const videoEl = document.getElementById(CONFIG.videoElementId);
        if (!videoEl) {
            console.warn("Translator: Video element not found.");
            return;
        }

        // 1. 创建覆盖层容器 (用于显示翻译文本)
        // 必须确保父容器是 relative 定位，以便 overlay 绝对定位
        const parent = videoEl.parentElement;
        if (getComputedStyle(parent).position === 'static') {
            parent.style.position = 'relative';
        }

        overlayContainer = document.createElement('div');
        overlayContainer.id = 'ocr-overlay-container';
        Object.assign(overlayContainer.style, {
            position: 'absolute',
            top: '0',
            left: '0',
            width: '100%',
            height: '100%',
            pointerEvents: 'none', // 让鼠标点击穿透，不影响操作视频
            zIndex: '20',
            overflow: 'hidden'
        });
        parent.appendChild(overlayContainer);

        // 2. 创建控制按钮
        const btn = document.createElement('button');
        btn.textContent = '🔍 开启实时翻译';
        Object.assign(btn.style, {
            position: 'absolute',
            top: '10px',
            right: '10px',
            zIndex: '30',
            padding: '8px 16px',
            backgroundColor: 'rgba(0, 0, 0, 0.6)',
            color: 'white',
            border: '1px solid rgba(255,255,255,0.3)',
            borderRadius: '4px',
            cursor: 'pointer',
            fontSize: '14px',
            backdropFilter: 'blur(4px)'
        });

        btn.onclick = () => toggleTranslation(btn);
        parent.appendChild(btn);
    }

    // --- 核心控制逻辑 ---
    async function toggleTranslation(btn) {
        if (isTranslating) {
            // 停止
            isTranslating = false;
            btn.textContent = '🔍 开启实时翻译';
            btn.style.backgroundColor = 'rgba(0, 0, 0, 0.6)';
            stopOCR();
            clearOverlay();
        } else {
            // 开启
            isTranslating = true;
            btn.textContent = '⏳ 初始化引擎...';
            btn.style.backgroundColor = 'rgba(0, 128, 0, 0.6)';
            
            try {
                await startOCR();
                btn.textContent = '🔴 停止翻译';
            } catch (e) {
                console.error(e);
                alert("OCR 引擎加载失败，请检查网络");
                isTranslating = false;
                btn.textContent = '🔍 开启实时翻译';
            }
        }
    }

    // --- OCR 引擎 ---
    async function startOCR() {
        if (!worker) {
            // 初始化 Tesseract Worker
            worker = await Tesseract.createWorker(CONFIG.ocrLanguage);
        }

        // 开始循环采样
        loopOCR();
    }

    function stopOCR() {
        if (timer) clearTimeout(timer);
        timer = null;
    }

    async function loopOCR() {
        if (!isTranslating) return;

        const videoEl = document.getElementById(CONFIG.videoElementId);
        
        // 1. 截图
        const canvas = document.createElement('canvas');
        canvas.width = videoEl.videoWidth; // 使用视频原始分辨率
        canvas.height = videoEl.videoHeight;
        const ctx = canvas.getContext('2d');
        ctx.drawImage(videoEl, 0, 0, canvas.width, canvas.height);

        // 2. 识别
        // console.log("OCR: Scanning...");
        try {
            const { data } = await worker.recognize(canvas);
            
            // 3. 渲染结果
            renderOverlay(data, videoEl);
        } catch (e) {
            console.error("OCR Error:", e);
        }

        // 4. 下一轮循环
        if (isTranslating) {
            timer = setTimeout(loopOCR, CONFIG.interval);
        }
    }

    // --- 渲染与翻译 ---
    async function renderOverlay(ocrData, videoEl) {
        clearOverlay();

        // 计算视频在屏幕上的缩放比例 (视频原始尺寸 vs 显示尺寸)
        // 这一步对于坐标对齐至关重要
        const scaleX = videoEl.offsetWidth / videoEl.videoWidth;
        const scaleY = videoEl.offsetHeight / videoEl.videoHeight;

        for (const word of ocrData.words) {
            if (word.confidence < CONFIG.confidenceThreshold) continue;
            if (word.text.trim().length < 2) continue; // 忽略太短的杂讯

            // 提取坐标
            const { x0, y0, x1, y1 } = word.bbox;
            
            // 翻译文本 (这里是一个 Mock 函数，实际需要对接翻译 API)
            const translatedText = await mockTranslate(word.text);

            // 创建文本框
            const div = document.createElement('div');
            div.textContent = translatedText;
            
            Object.assign(div.style, {
                position: 'absolute',
                left: `${x0 * scaleX}px`,
                top: `${y0 * scaleY}px`,
                width: `${(x1 - x0) * scaleX}px`,
                height: `${(y1 - y0) * scaleY}px`,
                backgroundColor: 'rgba(0, 0, 0, 0.7)',
                color: '#4ade80', // 绿色文字
                fontSize: `${(y1 - y0) * scaleX * 0.8}px`, // 根据文字高度自动调整字体
                lineHeight: '1',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                whiteSpace: 'nowrap',
                overflow: 'hidden',
                borderRadius: '2px',
                zIndex: '25',
                pointerEvents: 'none' // 再次确保穿透
            });

            overlayContainer.appendChild(div);
        }
    }

    function clearOverlay() {
        if (overlayContainer) {
            overlayContainer.innerHTML = '';
        }
    }

    /**
     * 模拟翻译函数
     * 实际项目中，你需要在这里调用 Google Translate / DeepL / 百度翻译 API
     */
    async function mockTranslate(text) {
        // 简单的演示逻辑：如果是英文，假装翻译一下
        // 在真实场景中，你会用 fetch 调用你的后端 API
        // const res = await fetch('/api/translate', { body: JSON.stringify({text}) });
        
        // 演示：
        // if (/^[a-zA-Z]+$/.test(text)) {
        //     return `[译]${text}`; 
        // }
        return text; // 如果不是纯英文，直接显示原文(OCR模式)
    }

    // --- 启动 ---
    // 等待 DOM 加载
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initTranslatorUI);
    } else {
        initTranslatorUI();
    }

})();