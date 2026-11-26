// =========================================================================
// PaddleOCR 模型与运行参数配置
// =========================================================================
const OCR_MODEL_SETTINGS = {
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

let TRANSLATE_SETTINGS = {
    // Ollama 原生对话接口地址
    user_language: 'zh-CN',
    llm_endpoint: '',
    llm_secret: '',
    llm_model: '',
    // 专用于日文 Galgame + OCR 文本的翻译提示词
    llm_system_prompt: `Translate given text`,
};


const LLM_KEEP_ALIVE = '30m';
const LLM_TIMEOUT_MS = 15000; // 15 秒超时

// =========================================================================
// 1. 虚拟翻译接口（待后续阶段接入真实翻译 API）
// =========================================================================

const translationCache = new Map();

/**
 * 调用局域网 Ollama API 完成日文 Galgame 文本翻译
 *
 * @param {string} text - PaddleOCR 识别并合并后的原文
 * @returns {Promise<string>} 翻译后的文本
 */
async function translateText(text) {
    // 移除换行符并清理首尾空白
    // .replace(/[\r\n]/g, '')
    const clean = (text || '').trim();
    if (!clean) return '';
    // console.log('llm system prompt:', TRANSLATE_SETTINGS.llm_system_prompt);
    // console.log('llm endpoint:', TRANSLATE_SETTINGS.llm_endpoint);
    // console.log('llm secret:', TRANSLATE_SETTINGS.llm_secret);
    // console.log('llm model:', TRANSLATE_SETTINGS.llm_model);
    // console.log('user language:', TRANSLATE_SETTINGS.user_language);
    console.log('Translating text:', clean);
    if (translationCache.has(clean)) {
        return translationCache.get(clean);
    }

    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), LLM_TIMEOUT_MS); // 15 秒超时

    try {
        const payload = {
            model: TRANSLATE_SETTINGS.llm_model,
            stream: false, // 关闭流式输出，一次性拿完整结果
            keep_alive: LLM_KEEP_ALIVE,
            messages: [
                {
                    role: 'system',
                    content: TRANSLATE_SETTINGS.llm_system_prompt,
                },
                {
                    role: 'user',
                    content: `Translate to ${TRANSLATE_SETTINGS.user_language}：\n${clean}`,
                },
            ],
            options: {
                temperature: 0.2, // 低温度防止 3B 小模型自由发挥或胡言乱语
                top_p: 0.7,
                num_predict: 256, // 限制最大输出 token，防止死循环复读
            },
        };

        const resp = await fetch(TRANSLATE_SETTINGS.llm_endpoint, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${TRANSLATE_SETTINGS.llm_secret}`,
            },
            body: JSON.stringify(payload),
            signal: controller.signal,
        });

        if (!resp.ok) {
            const errBody = await resp.text().catch(() => '');
            throw new Error(`HTTP ${resp.status}: ${errBody || resp.statusText}`);
        }

        const data = await resp.json();
        // 提取 Ollama /api/chat 的返回内容，并去掉可能残留的首尾引号
        let translated = (data?.message?.content || '').trim();
        translated = translated.replace(/^["“]|["”]$/g, '');

        if (translated) {
            translationCache.set(clean, translated);
            return translated;
        }
        return clean;
    } catch (err) {
        console.warn('[Ollama] Failed:', err);
        return `[Failed: ${err.name === 'AbortError' ? 'timeout' : err.message}]`;
    } finally {
        clearTimeout(timer);
    }
}
// =========================================================================
// 2. 高灵敏度局部网格帧差检测 + 上一次结果缓存
// =========================================================================
const DIFF_SAMPLE_W = 128;
const DIFF_SAMPLE_H = 64;
const BLOCK_SIZE = 8;               // 将采样图划分为 8x8 局部网格检测变化
const BLOCK_DIFF_THRESHOLD = 5;   // 单个网格内平均亮度变化阈值（越小越灵敏，推荐 2.5~5.0）
const STABLE_DURATION_MS = 700;     // 画面停止变化后等待 700ms 触发（可按需改为 1000）

const diffCanvas = new OffscreenCanvas(DIFF_SAMPLE_W, DIFF_SAMPLE_H);
const diffCtx = diffCanvas.getContext('2d', { willReadFrequently: true });

const workCanvas = new OffscreenCanvas(1, 1);
const workCtx = workCanvas.getContext('2d', { willReadFrequently: true });

let prevSample = null;          // 上一帧缩略图
let lastMotionTime = 0;         // 最近一次检测到画面变化的时间戳
let hasPendingChange = false;   // 标记是否有待处理的画面变化

// 显式保存上一次的原文与翻译结果
let lastOriginalText = '';
let lastTranslatedText = '';

let detSession = null;
let recSession = null;
let charDict = null;
let initPromise = null;

function sampleLuma(bitmap) {
    diffCtx.drawImage(bitmap, 0, 0, DIFF_SAMPLE_W, DIFF_SAMPLE_H);
    const rgba = diffCtx.getImageData(0, 0, DIFF_SAMPLE_W, DIFF_SAMPLE_H).data;
    const luma = new Uint8Array(DIFF_SAMPLE_W * DIFF_SAMPLE_H);
    for (let i = 0, p = 0; i < rgba.length; i += 4, p++) {
        luma[p] = (rgba[i] * 77 + rgba[i + 1] * 150 + rgba[i + 2] * 29) >> 8;
    }
    return luma;
}

/**
 * 计算局部 8x8 网格的最大平均差异
 * 只要选区内任意一小块区域出现新文字，就能返回较大的差值，不会被大片背景稀释
 */
function calcMaxBlockDiff(a, b) {
    if (!a || !b || a.length !== b.length) return 255;
    let maxBlockDiff = 0;
    const pixelsPerBlock = BLOCK_SIZE * BLOCK_SIZE;

    for (let by = 0; by < DIFF_SAMPLE_H; by += BLOCK_SIZE) {
        for (let bx = 0; bx < DIFF_SAMPLE_W; bx += BLOCK_SIZE) {
            let sum = 0;
            for (let y = 0; y < BLOCK_SIZE; y++) {
                const rowOffset = (by + y) * DIFF_SAMPLE_W + bx;
                for (let x = 0; x < BLOCK_SIZE; x++) {
                    sum += Math.abs(a[rowOffset + x] - b[rowOffset + x]);
                }
            }
            const blockMean = sum / pixelsPerBlock;
            if (blockMean > maxBlockDiff) {
                maxBlockDiff = blockMean;
            }
        }
    }
    return maxBlockDiff;
}
// =========================================================================
// 3. PaddleOCR (PP-OCRv4 ONNX) 初始化与推理管线
// =========================================================================

async function ensurePaddleOCR() {
    if (detSession && recSession && charDict) return;
    if (initPromise) return initPromise;

    initPromise = (async () => {
        const ort = await import(OCR_MODEL_SETTINGS.ortModuleUrl);
        ort.env.wasm.wasmPaths = OCR_MODEL_SETTINGS.wasmBasePath;
        ort.env.wasm.numThreads = 1;

        const sessionOpts = {
            executionProviders: ['wasm'],
            graphOptimizationLevel: 'all',
        };

        const [det, rec, dictResp] = await Promise.all([
            ort.InferenceSession.create(OCR_MODEL_SETTINGS.detModelUrl, sessionOpts),
            ort.InferenceSession.create(OCR_MODEL_SETTINGS.recModelUrl, sessionOpts),
            fetch(OCR_MODEL_SETTINGS.dictUrl),
        ]);

        if (!dictResp.ok) {
            throw new Error(`Failed to fetch dictionary (${dictResp.status}): ${OCR_MODEL_SETTINGS.dictUrl}`);
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
            if (visited[idx] || probMap[y * padW + x] < OCR_MODEL_SETTINGS.detThreshold) continue;

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
                    if (!visited[nIdx] && probMap[ny * padW + nx] >= OCR_MODEL_SETTINGS.detThreshold) {
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
    const targetH = OCR_MODEL_SETTINGS.recImgHeight;
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
    if (msg.type === 'UPDATE_SETTINGS' && msg.settings) {
        console.log('Updating translate settings', msg);
        if (msg.settings) {
            TRANSLATE_SETTINGS.llm_endpoint = msg.settings.translate_llm_endpoint;
            TRANSLATE_SETTINGS.llm_prompt = msg.settings.translate_llm_prompt;
            TRANSLATE_SETTINGS.llm_secret = msg.settings.translate_llm_secret;
            TRANSLATE_SETTINGS.llm_model = msg.settings.translate_llm_model;
            TRANSLATE_SETTINGS.user_language = msg.settings.user_language;
            TRANSLATE_SETTINGS.llm_system_prompt = msg.settings.translate_llm_system_prompt;
        }
        return;
    }

    if (msg.type === 'RESET_DIFF') {
        prevSample = null;
        lastMotionTime = 0;
        hasPendingChange = false;
        lastOriginalText = '';
        lastTranslatedText = '';
        return;
    }

    if (msg.type !== 'OCR_FRAME' || !msg.bitmap) return;
    const bitmap = msg.bitmap;
    const now = performance.now();

    try {
        const curSample = sampleLuma(bitmap);

        // 0. 首帧初始化：记录基准并进入等待稳定状态
        if (!prevSample) {
            prevSample = curSample;
            lastMotionTime = now;
            hasPendingChange = true;
            self.postMessage({ type: 'SKIPPED', reason: 'init_wait' });
            return;
        }

        // 计算局部最大网格变化
        const maxDiff = calcMaxBlockDiff(curSample, prevSample);
        prevSample = curSample;

        // 1. 任意网格仍在变化（逐字打印 / 翻页中）：重置静止计时器并标记有新变化
        if (maxDiff >= BLOCK_DIFF_THRESHOLD) {
            lastMotionTime = now;
            hasPendingChange = true;
            self.postMessage({ type: 'SKIPPED', reason: 'animating' });
            return;
        }

        // 2. 画面静止且没有待处理的新变化：直接跳过
        if (!hasPendingChange) {
            self.postMessage({ type: 'SKIPPED', reason: 'idle' });
            return;
        }

        // 3. 画面已停止变化，但持续静止时间未达到设定时长：继续等待
        if (now - lastMotionTime < STABLE_DURATION_MS) {
            self.postMessage({ type: 'SKIPPED', reason: 'waiting_stable' });
            return;
        }

        // 4. 满足“变化后持续静止”条件：消耗标记并执行 OCR
        hasPendingChange = false;

        const rawText = await runPaddleOCR(bitmap);
        if (!rawText) {
            lastOriginalText = '';
            lastTranslatedText = '';
            self.postMessage({ type: 'RESULT', rawText: '', translatedText: '' });
            return;
        }

        // 5. 命中上一次原文缓存：直接返回上一次保存的译文
        if (rawText === lastOriginalText && lastTranslatedText) {
            self.postMessage({
                type: 'RESULT',
                rawText,
                translatedText: lastTranslatedText,
            });
            return;
        }

        // 6. 命中历史 Map 缓存（如回看历史记录或重复选项）
        if (translationCache.has(rawText)) {
            const cached = translationCache.get(rawText);
            lastOriginalText = rawText;
            lastTranslatedText = cached;
            self.postMessage({
                type: 'RESULT',
                rawText,
                translatedText: cached,
            });
            console.log('Cache hit for rawText:', rawText, 'cached translation:', cached);
            return;
        }

        // 7. 调用 Ollama 翻译并更新上一次原文/译文缓存
        const translatedText = await translateText(rawText);
        lastOriginalText = rawText;
        lastTranslatedText = translatedText;

        self.postMessage({ type: 'RESULT', rawText, translatedText });
    } catch (err) {
        console.error('OCR Worker encountered an error', err);
        self.postMessage({
            type: 'ERROR',
            error: err instanceof Error ? err.message : String(err),
        });
    } finally {
        bitmap.close();
    }
};