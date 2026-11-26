// =========================================================================
// PaddleOCR 模型与运行参数配置
// =========================================================================
const MODEL_CONFIG = {
    // ONNX Runtime Web ESM 入口（支持本地静态路径或 CDN）
    ortModuleUrl: 'https://cdn.jsdelivr.net/npm/onnxruntime-web@1.20.1/dist/ort.min.mjs',
    wasmBasePath: 'https://cdn.jsdelivr.net/npm/onnxruntime-web@1.20.1/dist/',

    // PaddleOCR (PP-OCRv4 Mobile) 模型与字典路径，请替换为你的实际部署路径
    detModelUrl: './ch_PP-OCRv4_det_infer.onnx',
    recModelUrl: './japan_PP-OCRv4_rec_infer.onnx', // 日语 Galgame 推荐 japan 模型；通用可选 ch_PP-OCRv4_rec_infer.onnx
    dictUrl: './japan_dict.txt',

    detThreshold: 0.3,
    recImgHeight: 48,
};

const TRANSLATE_CONFIG = {
    // 默认请求当前页面同 IP 的 5000 端口；若通过 HTTPS 域名访问，可改为反向代理路径（如 '/libretranslate/translate'）
    endpoint: `http://${self.location.hostname}:5000/translate`,
    sourceLang: 'ja',   // 日文 Galgame 建议固定为 'ja'（比 'auto' 对短句和人名更准）；通用可改为 'auto'
    targetLang: 'zh',   // LibreTranslate 中文目标语言代码为 'zh'
    apiKey: '',         // 若服务端未开启 API Key 验证保持为空即可
    timeoutMs: 8000,
};

// Galgame 打字机稳定与静态帧去重参数
const DIFF_SAMPLE_W = 64;
const DIFF_SAMPLE_H = 32;
const DIFF_THRESHOLD = 2.2; // 像素平均亮度变化阈值（0~255）

const diffCanvas = new OffscreenCanvas(DIFF_SAMPLE_W, DIFF_SAMPLE_H);
const diffCtx = diffCanvas.getContext('2d', { willReadFrequently: true });

const workCanvas = new OffscreenCanvas(1, 1);
const workCtx = workCanvas.getContext('2d', { willReadFrequently: true });

let prevSample = null;      // 上一次收到的帧缩略图（用于判断打字机是否仍在刷新）
let lastOcrSample = null;   // 上一次实际执行 OCR 的帧缩略图（用于跳过静态画面）
let lastRawText = '';

let detSession = null;
let recSession = null;
let charDict = null;
let initPromise = null;

// =========================================================================
// 1. 虚拟翻译接口（待后续阶段接入真实翻译 API）
// =========================================================================

const translationCache = new Map();

/**
 * 调用本机同 IP 5000 端口的 LibreTranslate API 完成翻译
 *
 * @param {string} text - PaddleOCR 识别并合并后的原文
 * @returns {Promise<string>} 翻译后的文本
 */
async function translateText(text) {
    const clean = (text || '').trim();
    if (!clean) return '';

    if (translationCache.has(clean)) {
        return translationCache.get(clean);
    }

    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), TRANSLATE_CONFIG.timeoutMs);

    try {
        const payload = {
            q: clean,
            source: TRANSLATE_CONFIG.sourceLang,
            target: TRANSLATE_CONFIG.targetLang,
            format: 'text',
        };
        if (TRANSLATE_CONFIG.apiKey) {
            payload.api_key = TRANSLATE_CONFIG.apiKey;
        }

        const resp = await fetch(TRANSLATE_CONFIG.endpoint, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(payload),
            signal: controller.signal,
        });

        if (!resp.ok) {
            const errBody = await resp.text().catch(() => '');
            throw new Error(`HTTP ${resp.status}: ${errBody || resp.statusText}`);
        }

        const data = await resp.json();
        const translated = (data && typeof data.translatedText === 'string')
            ? data.translatedText.trim()
            : '';

        if (translated) {
            translationCache.set(clean, translated);
            return translated;
        }
        return clean;
    } catch (err) {
        console.warn('[LibreTranslate] 翻译请求失败:', err);
        // 失败时不写入缓存，下次画面刷新可重试，同时在界面保留原文并提示错误
        return `${clean}\n[翻译失败: ${err.name === 'AbortError' ? '请求超时' : err.message}]`;
    } finally {
        clearTimeout(timer);
    }
}
// =========================================================================
// 2. 帧差检测（跳过静态台词 + 等待打字机打印完成）
// =========================================================================

function sampleLuma(bitmap) {
    diffCtx.drawImage(bitmap, 0, 0, DIFF_SAMPLE_W, DIFF_SAMPLE_H);
    const rgba = diffCtx.getImageData(0, 0, DIFF_SAMPLE_W, DIFF_SAMPLE_H).data;
    const luma = new Uint8Array(DIFF_SAMPLE_W * DIFF_SAMPLE_H);
    for (let i = 0, p = 0; i < rgba.length; i += 4, p++) {
        luma[p] = (rgba[i] * 77 + rgba[i + 1] * 150 + rgba[i + 2] * 29) >> 8;
    }
    return luma;
}

function calcMeanAbsDiff(a, b) {
    if (!a || !b || a.length !== b.length) return 255;
    let sum = 0;
    for (let i = 0; i < a.length; i++) {
        sum += Math.abs(a[i] - b[i]);
    }
    return sum / a.length;
}

// =========================================================================
// 3. PaddleOCR (PP-OCRv4 ONNX) 初始化与推理管线
// =========================================================================

async function ensurePaddleOCR() {
    if (detSession && recSession && charDict) return;
    if (initPromise) return initPromise;

    initPromise = (async () => {
        const ort = await import(MODEL_CONFIG.ortModuleUrl);
        ort.env.wasm.wasmPaths = MODEL_CONFIG.wasmBasePath;
        ort.env.wasm.numThreads = 1;

        const sessionOpts = {
            executionProviders: ['wasm'],
            graphOptimizationLevel: 'all',
        };

        const [det, rec, dictResp] = await Promise.all([
            ort.InferenceSession.create(MODEL_CONFIG.detModelUrl, sessionOpts),
            ort.InferenceSession.create(MODEL_CONFIG.recModelUrl, sessionOpts),
            fetch(MODEL_CONFIG.dictUrl),
        ]);

        if (!dictResp.ok) {
            throw new Error(`字典加载失败 (${dictResp.status}): ${MODEL_CONFIG.dictUrl}`);
        }
        const dictText = await dictResp.text();
        // PaddleOCR 字典：索引 0 为 CTC blank，1..N 对应字典每行字符，末尾追加空格
        const lines = dictText.replace(/\r\n/g, '\n').split('\n');
        if (lines[lines.length - 1] === '') lines.pop();
        charDict = ['blank', ...lines, ' '];

        detSession = det;
        recSession = rec;
        self.ort = ort;
    })().catch((err) => {
        initPromise = null;
        throw err;
    });

    return initPromise;
}

/**
 * 文本检测 (DBNet)：找出选区内的文本框列表，按从上到下、从左到右排序
 */
async function detectTextBoxes(imgData, width, height) {
    const padW = Math.ceil(width / 32) * 32;
    const padH = Math.ceil(height / 32) * 32;
    const planeSize = padW * padH;
    const tensorData = new Float32Array(3 * planeSize);

    const mean = [0.485, 0.456, 0.406];
    const std = [0.229, 0.224, 0.225];
    const src = imgData.data;

    for (let y = 0; y < height; y++) {
        for (let x = 0; x < width; x++) {
            const srcIdx = (y * width + x) * 4;
            const dstIdx = y * padW + x;
            tensorData[dstIdx] = (src[srcIdx] / 255 - mean[0]) / std[0];
            tensorData[planeSize + dstIdx] = (src[srcIdx + 1] / 255 - mean[1]) / std[1];
            tensorData[2 * planeSize + dstIdx] = (src[srcIdx + 2] / 255 - mean[2]) / std[2];
        }
    }

    const inputTensor = new self.ort.Tensor('float32', tensorData, [1, 3, padH, padW]);
    const outMap = await detSession.run({ [detSession.inputNames[0]]: inputTensor });
    const probMap = outMap[detSession.outputNames[0]].data;

    // 连通域提取文本框
    const visited = new Uint8Array(width * height);
    const boxes = [];
    const stackX = new Int32Array(width * height);
    const stackY = new Int32Array(width * height);

    for (let y = 0; y < height; y++) {
        for (let x = 0; x < width; x++) {
            const idx = y * width + x;
            if (visited[idx] || probMap[y * padW + x] < MODEL_CONFIG.detThreshold) continue;

            let minX = x, maxX = x, minY = y, maxY = y;
            let top = 0;
            stackX[top] = x;
            stackY[top] = y;
            visited[idx] = 1;
            top++;

            while (top > 0) {
                top--;
                const cx = stackX[top];
                const cy = stackY[top];
                if (cx < minX) minX = cx;
                if (cx > maxX) maxX = cx;
                if (cy < minY) minY = cy;
                if (cy > maxY) maxY = cy;

                const neighbors = [
                    [cx - 1, cy], [cx + 1, cy],
                    [cx, cy - 1], [cx, cy + 1],
                ];
                for (let k = 0; k < 4; k++) {
                    const nx = neighbors[k][0];
                    const ny = neighbors[k][1];
                    if (nx < 0 || ny < 0 || nx >= width || ny >= height) continue;
                    const nIdx = ny * width + nx;
                    if (!visited[nIdx] && probMap[ny * padW + nx] >= MODEL_CONFIG.detThreshold) {
                        visited[nIdx] = 1;
                        stackX[top] = nx;
                        stackY[top] = ny;
                        top++;
                    }
                }
            }

            const bw = maxX - minX + 1;
            const bh = maxY - minY + 1;
            if (bw < 6 || bh < 6) continue;

            // 适度外扩包围盒，避免裁掉文字边缘笔画
            const padX = Math.round(bh * 0.45);
            const padY = Math.round(bh * 0.35);
            const x1 = Math.max(0, minX - padX);
            const y1 = Math.max(0, minY - padY);
            const x2 = Math.min(width, maxX + padX);
            const y2 = Math.min(height, maxY + padY);

            boxes.push({ x: x1, y: y1, w: x2 - x1, h: y2 - y1 });
        }
    }

    // Galgame 对话按行从上到下排序，同一行按从左到右排序
    boxes.sort((a, b) => {
        const lineTol = Math.min(a.h, b.h) * 0.5;
        if (Math.abs(a.y - b.y) > lineTol) {
            return a.y - b.y;
        }
        return a.x - b.x;
    });

    return boxes;
}

/**
 * 文本行识别 (SVTR / CRNN + CTC Greedy Decode)
 */
async function recognizeBoxText(sourceCanvas, box) {
    const targetH = MODEL_CONFIG.recImgHeight;
    const targetW = Math.max(targetH, Math.round(box.w * (targetH / box.h)));

    const lineCanvas = new OffscreenCanvas(targetW, targetH);
    const lineCtx = lineCanvas.getContext('2d');
    lineCtx.drawImage(sourceCanvas, box.x, box.y, box.w, box.h, 0, 0, targetW, targetH);

    const rgba = lineCtx.getImageData(0, 0, targetW, targetH).data;
    const planeSize = targetW * targetH;
    const tensorData = new Float32Array(3 * planeSize);

    for (let i = 0, p = 0; i < rgba.length; i += 4, p++) {
        // PP-OCR 识别模型标准归一化: (val / 255 - 0.5) / 0.5
        tensorData[p] = rgba[i + 2] / 127.5 - 1.0;               // B
        tensorData[planeSize + p] = rgba[i + 1] / 127.5 - 1.0;   // G
        tensorData[2 * planeSize + p] = rgba[i] / 127.5 - 1.0;   // R
    }

    const inputTensor = new self.ort.Tensor('float32', tensorData, [1, 3, targetH, targetW]);
    const outMap = await recSession.run({ [recSession.inputNames[0]]: inputTensor });
    const output = outMap[recSession.outputNames[0]];

    const seqLen = output.dims[1];
    const numClasses = output.dims[2];
    const logits = output.data;

    let text = '';
    let prevIdx = 0;
    for (let t = 0; t < seqLen; t++) {
        const offset = t * numClasses;
        let maxIdx = 0;
        let maxVal = logits[offset];
        for (let c = 1; c < numClasses; c++) {
            if (logits[offset + c] > maxVal) {
                maxVal = logits[offset + c];
                maxIdx = c;
            }
        }
        if (maxIdx !== 0 && maxIdx !== prevIdx && maxIdx < charDict.length) {
            text += charDict[maxIdx];
        }
        prevIdx = maxIdx;
    }

    return text.trim();
}

async function runPaddleOCR(bitmap) {
    await ensurePaddleOCR();

    const w = bitmap.width;
    const h = bitmap.height;
    workCanvas.width = w;
    workCanvas.height = h;
    workCtx.drawImage(bitmap, 0, 0, w, h);
    const imgData = workCtx.getImageData(0, 0, w, h);

    const boxes = await detectTextBoxes(imgData, w, h);
    if (boxes.length === 0) return '';

    const lines = [];
    for (const box of boxes) {
        const lineText = await recognizeBoxText(workCanvas, box);
        if (lineText) {
            lines.push(lineText);
        }
    }

    // 合并多行台词（若首行为较短的角色名，保留换行以区分说话人与正文）
    if (lines.length >= 2 && lines[0].length <= 12) {
        return `${lines[0]}\n${lines.slice(1).join('')}`;
    }
    return lines.join('\n');
}

// =========================================================================
// 4. Worker 消息入口
// =========================================================================

self.onmessage = async (e) => {
    const msg = e.data || {};

    if (msg.type === 'RESET_DIFF') {
        prevSample = null;
        lastOcrSample = null;
        lastRawText = '';
        return;
    }

    if (msg.type !== 'OCR_FRAME' || !msg.bitmap) return;
    const bitmap = msg.bitmap;

    try {
        const curSample = sampleLuma(bitmap);

        // 1. 若与上一次已识别画面几乎无变化，说明台词静止，直接跳过
        if (calcMeanAbsDiff(curSample, lastOcrSample) < DIFF_THRESHOLD) {
            self.postMessage({ type: 'SKIPPED', reason: 'unchanged' });
            return;
        }

        // 2. 若与上一采样帧仍在剧烈变化，说明 Galgame 正在逐字打字输出，等下一帧稳定再识别
        if (prevSample && calcMeanAbsDiff(curSample, prevSample) >= DIFF_THRESHOLD) {
            prevSample = curSample;
            self.postMessage({ type: 'SKIPPED', reason: 'animating' });
            return;
        }

        prevSample = curSample;
        lastOcrSample = curSample;

        // 3. 执行 PaddleOCR 识别
        const rawText = await runPaddleOCR(bitmap);
        if (!rawText) {
            lastRawText = '';
            self.postMessage({ type: 'RESULT', rawText: '', translatedText: '' });
            return;
        }

        if (rawText === lastRawText && translationCache.has(rawText)) {
            self.postMessage({
                type: 'RESULT',
                rawText,
                translatedText: translationCache.get(rawText),
            });
            return;
        }

        lastRawText = rawText;

        // 4. 调用虚拟翻译接口
        const translatedText = await translateText(rawText);
        self.postMessage({ type: 'RESULT', rawText, translatedText });
    } catch (err) {
        console.error("OCR Worker encountered an error", err);
        self.postMessage({
            type: 'ERROR',
            error: err instanceof Error ? err.message : String(err),
        });
        
    } finally {
        // 务必关闭 ImageBitmap 释放显存
        bitmap.close();
    }
};