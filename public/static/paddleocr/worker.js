// =========================================================================
// PaddleOCR 模型与运行参数配置
// =========================================================================
const OCR_MODEL_SETTINGS = {
    // ONNX Runtime Web ESM 入口（支持本地静态路径或 CDN）
    ortModuleUrl: 'https://cdn.jsdelivr.net/npm/onnxruntime-web@1.31.1/dist/ort.min.mjs',
    wasmBasePath: 'https://cdn.jsdelivr.net/npm/onnxruntime-web@1.31.1/dist/',

    // PaddleOCR (PP-OCRv4 Mobile) 模型与字典路径，请替换为你的实际部署路径
    detModelUrl: 'https://huggingface.co/PaddlePaddle/PP-OCRv6_small_det_onnx/resolve/main/inference.onnx',
    recModelUrl: 'https://huggingface.co/PaddlePaddle/PP-OCRv6_small_rec_onnx/resolve/main/inference.onnx',
    dictUrl: 'https://raw.githubusercontent.com/PaddlePaddle/PaddleOCR/refs/heads/main/ppocr/utils/dict/ppocrv6_dict.txt',
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

const MODEL_CACHE_DB = 'gs-paddleocr-model-cache';
const MODEL_CACHE_STORE = 'files';

/**
 * 从 IndexedDB 读取缓存文件；若未命中则从网络下载并写入缓存
 */
function openModelCacheDB() {
    return new Promise((resolve, reject) => {
        const req = indexedDB.open(MODEL_CACHE_DB, 1);
        req.onupgradeneeded = () => {
            const db = req.result;
            if (!db.objectStoreNames.contains(MODEL_CACHE_STORE)) {
                db.createObjectStore(MODEL_CACHE_STORE);
            }
        };
        req.onsuccess = () => resolve(req.result);
        req.onerror = () => reject(req.error);
    });
}

async function fetchWithIDBCache(url, responseType = 'arraybuffer') {
    const cacheKey = `${responseType}:${url}`;
    let db = null;

    try {
        db = await openModelCacheDB();
        const cached = await new Promise((resolve, reject) => {
            const tx = db.transaction(MODEL_CACHE_STORE, 'readonly');
            const req = tx.objectStore(MODEL_CACHE_STORE).get(cacheKey);
            req.onsuccess = () => resolve(req.result);
            req.onerror = () => reject(req.error);
        });
        if (cached !== undefined) {
            console.log('[OCR Cache] Hit local cache:', url);
            return cached;
        }
    } catch (e) {
        console.warn('[OCR Cache] Failed to read IndexedDB, falling back to direct download:', e);
    }

    console.log('[OCR Cache] First time downloading model/dictionary:', url);
    const resp = await fetch(url);
    if (!resp.ok) {
        throw new Error(`下载失败 (${resp.status}): ${url}`);
    }

    const data = responseType === 'text'
        ? await resp.text()
        : await resp.arrayBuffer();

    if (db) {
        try {
            await new Promise((resolve, reject) => {
                const tx = db.transaction(MODEL_CACHE_STORE, 'readwrite');
                const req = tx.objectStore(MODEL_CACHE_STORE).put(data, cacheKey);
                req.onsuccess = () => resolve();
                req.onerror = () => reject(req.error);
            });
        } catch (e) {
            console.warn('[OCR Cache] Failed to write to IndexedDB:', e);
        }
    }

    return data;
}

// =========================================================================
// 1. 翻译接口）
// =========================================================================

/**
 * 根据 endpoint URL 后缀识别 API 协议类型
 * @param {string} endpoint
 * @returns {'ollama' | 'openai' | 'claude'}
 */
function detectApiProvider(endpoint) {
    try {
        // 去除末尾斜杠和查询参数后匹配路径后缀
        const pathname = new URL(endpoint, self.location?.origin || 'http://localhost')
            .pathname.replace(/\/+$/, '');

        if (pathname.endsWith('/messages')) return 'claude';
        if (pathname.endsWith('/chat/completions')) return 'openai';
        if (pathname.endsWith('/api/chat')) return 'ollama';
    } catch (_) {
        // URL 解析异常时的兜底字符串匹配
        const cleanUrl = (endpoint || '').split('?')[0].replace(/\/+$/, '');
        if (cleanUrl.endsWith('/messages')) return 'claude';
        if (cleanUrl.endsWith('/chat/completions')) return 'openai';
        if (cleanUrl.endsWith('/api/chat')) return 'ollama';
    }
    // 默认按最通用的 OpenAI 兼容格式处理
    return 'openai';
}

const translationCache = new Map();

/**
 * 调用大模型 API 完成文本翻译（自动适配 Ollama / OpenAI / Claude）
 *
 * @param {string} text - PaddleOCR 识别并合并后的原文
 * @returns {Promise<string>} 翻译后的文本
 */
async function translateText(text) {
    const clean = (text || '').trim();
    if (!clean) return '';

    console.log('Translating text:', clean);
    if (translationCache.has(clean)) {
        return translationCache.get(clean);
    }

    const provider = detectApiProvider(TRANSLATE_SETTINGS.llm_endpoint);
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), LLM_TIMEOUT_MS);

    const userPrompt = `Translate to ${TRANSLATE_SETTINGS.user_language}：\n${clean}`;
    const systemPrompt = TRANSLATE_SETTINGS.llm_system_prompt || '';
    const secret = (TRANSLATE_SETTINGS.llm_secret || '').trim();

    try {
        const headers = {
            'Content-Type': 'application/json',
        };
        let payload;

        if (provider === 'claude') {
            // 1. Anthropic Claude API (/v1/messages)
            if (secret) {
                headers['x-api-key'] = secret;
            }
            headers['anthropic-version'] = '2023-06-01';
            // 允许浏览器 / Web Worker 直接跨域调用 Anthropic 官方接口
            headers['anthropic-dangerous-direct-browser-access'] = 'true';

            payload = {
                model: TRANSLATE_SETTINGS.llm_model,
                max_tokens: 256,
                temperature: 0.2, // Claude 建议仅传 temperature 或 top_p 其一，避免部分模型报错
                ...(systemPrompt ? { system: systemPrompt } : {}),
                messages: [
                    { role: 'user', content: userPrompt },
                ],
            };
        } else if (provider === 'ollama') {
            // 2. Ollama 原生 API (/api/chat)
            if (secret) {
                headers['Authorization'] = `Bearer ${secret}`;
            }
            payload = {
                model: TRANSLATE_SETTINGS.llm_model,
                stream: false,
                keep_alive: LLM_KEEP_ALIVE,
                messages: [
                    ...(systemPrompt ? [{ role: 'system', content: systemPrompt }] : []),
                    { role: 'user', content: userPrompt },
                ],
                options: {
                    temperature: 0.2,
                    top_p: 0.7,
                    num_predict: 256,
                },
            };
        } else {
            // 3. OpenAI 兼容 API (/v1/chat/completions)
            if (secret) {
                headers['Authorization'] = `Bearer ${secret}`;
            }
            payload = {
                model: TRANSLATE_SETTINGS.llm_model,
                stream: false,
                temperature: 0.2,
                top_p: 0.7,
                max_tokens: 256,
                messages: [
                    ...(systemPrompt ? [{ role: 'system', content: systemPrompt }] : []),
                    { role: 'user', content: userPrompt },
                ],
            };
        }

        const resp = await fetch(TRANSLATE_SETTINGS.llm_endpoint, {
            method: 'POST',
            headers,
            body: JSON.stringify(payload),
            signal: controller.signal,
        });

        if (!resp.ok) {
            const errBody = await resp.text().catch(() => '');
            throw new Error(`HTTP ${resp.status}: ${errBody || resp.statusText}`);
        }

        const data = await resp.json();

        // 按协议提取模型输出文本
        let rawOutput = '';
        if (provider === 'claude') {
            rawOutput = Array.isArray(data?.content)
                ? data.content.filter((b) => b.type === 'text').map((b) => b.text).join('')
                : '';
        } else if (provider === 'ollama') {
            rawOutput = data?.message?.content || '';
        } else {
            rawOutput = data?.choices?.[0]?.message?.content || '';
        }

        // 清理首尾空白及可能残留的引号
        let translated = rawOutput.trim().replace(/^["“]|["”]$/g, '');

        if (translated) {
            translationCache.set(clean, translated);
            return translated;
        }
        return clean;
    } catch (err) {
        console.warn(`[LLM:${provider}] Failed:`, err);
        return `[Failed: ${err.name === 'AbortError' ? 'timeout' : err.message}]`;
    } finally {
        clearTimeout(timer);
    }
}

// =========================================================================
// 2. 抗光标干扰的局部网格帧差检测 + 上一次结果缓存
// =========================================================================
const DIFF_SAMPLE_W = 128;
const DIFF_SAMPLE_H = 64;
const BLOCK_SIZE = 8;               // 将采样图划分为 8x8 局部网格检测变化
const BLOCK_DIFF_THRESHOLD = 3;   // 单个网格内平均亮度变化阈值
const CURSOR_MAX_BLOCKS = 3;        // 仅 1~3 个网格变化时视为等待光标旋转/闪烁，忽略干扰
const STABLE_DURATION_MS = 700;     // 文本停止变化后等待 700ms 触发
const MAX_WAIT_MS = 1500;           // 兜底：若背景持续有动态特效，最长等待 1.5s 强制识别

const diffCanvas = new OffscreenCanvas(DIFF_SAMPLE_W, DIFF_SAMPLE_H);
const diffCtx = diffCanvas.getContext('2d', { willReadFrequently: true });

const workCanvas = new OffscreenCanvas(1, 1);
const workCtx = workCanvas.getContext('2d', { willReadFrequently: true });

let prevSample = null;              // 上一帧缩略图
let lastOcrSample = null;           // 上一次实际执行 OCR 时的画面基准
let lastMotionTime = 0;             // 最近一次检测到有效文本变化的时间戳
let pendingStartTime = 0;           // 本轮变化开始的时间戳（用于兜底计时）
let hasPendingChange = false;       // 标记是否有待处理的画面变化

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
 * 统计亮度变化超过阈值的 8x8 网格数量
 */
function countChangedBlocks(a, b) {
    if (!a || !b || a.length !== b.length) return 999;
    let changedBlocks = 0;
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
            if (sum / pixelsPerBlock > BLOCK_DIFF_THRESHOLD) {
                changedBlocks++;
            }
        }
    }
    return changedBlocks;
}
// =========================================================================
// 3. PaddleOCR (PP-OCRv4 ONNX) 初始化与推理管线
// =========================================================================

async function ensurePaddleOCR() {
    if (detSession && recSession && charDict) return;
    if (initPromise) return initPromise;

    initPromise = (async () => {
        self.postMessage({
            type: 'ERROR',
            error: 'Now loading OCR model (first run will download about 15MB from Hugging Face, please wait)...',
        });
        const ort = await import(OCR_MODEL_SETTINGS.ortModuleUrl);
        ort.env.wasm.wasmPaths = OCR_MODEL_SETTINGS.wasmBasePath;
        ort.env.wasm.numThreads = 1;

        const sessionOpts = {
            executionProviders: ['wasm'],
            graphOptimizationLevel: 'all',
        };

        // 并行读取/下载检测模型、识别模型和字典
        const [detBuffer, recBuffer, dictText] = await Promise.all([
            fetchWithIDBCache(OCR_MODEL_SETTINGS.detModelUrl, 'arraybuffer'),
            fetchWithIDBCache(OCR_MODEL_SETTINGS.recModelUrl, 'arraybuffer'),
            fetchWithIDBCache(OCR_MODEL_SETTINGS.dictUrl, 'text'),
        ]);

        const [det, rec] = await Promise.all([
            ort.InferenceSession.create(detBuffer, sessionOpts),
            ort.InferenceSession.create(recBuffer, sessionOpts),
        ]);

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

            const DX = [-1, 1, 0, 0];
            const DY = [0, 0, -1, 1];
            const detThreshold = OCR_MODEL_SETTINGS.detThreshold;
            while (top > 0) {
                top--;
                const cx = stackX[top];
                const cy = stackY[top];
                if (cx < minX) minX = cx;
                if (cx > maxX) maxX = cx;
                if (cy < minY) minY = cy;
                if (cy > maxY) maxY = cy;

                for (let k = 0; k < 4; k++) {
                    const nx = cx + DX[k];
                    const ny = cy + DY[k];
                    if (nx < 0 || ny < 0 || nx >= width || ny >= height) continue;
                    const nIdx = ny * width + nx;
                    if (!visited[nIdx] && probMap[ny * padW + nx] >= detThreshold) {
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
            const padY = Math.round(bh * 0.25);
            const x1 = Math.max(0, minX - padX);
            const y1 = Math.max(0, minY - padY);
            const x2 = Math.min(width, maxX + padX);
            const y2 = Math.min(height, maxY + padY);

            boxes.push({ x: x1, y: y1, w: x2 - x1, h: y2 - y1 });
        }
    }

    // Galgame 对话按行从上到下排序，同一行按从左到右排序
    boxes.sort((a, b) => {
        const aCy = a.y + a.h * 0.5;
        const bCy = b.y + b.h * 0.5;
        const lineTol = Math.max(a.h, b.h) * 0.5;
        if (Math.abs(aCy - bCy) > lineTol) {
            return aCy - bCy;
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
        TRANSLATE_SETTINGS.llm_endpoint = msg.settings.translate_llm_endpoint;
        TRANSLATE_SETTINGS.llm_prompt = msg.settings.translate_llm_prompt;
        TRANSLATE_SETTINGS.llm_secret = msg.settings.translate_llm_secret;
        TRANSLATE_SETTINGS.llm_model = msg.settings.translate_llm_model;
        TRANSLATE_SETTINGS.user_language = msg.settings.user_language;
        TRANSLATE_SETTINGS.llm_system_prompt = msg.settings.translate_llm_system_prompt;
        return;
    }

    if (msg.type === 'RESET_DIFF') {
        prevSample = null;
        lastOcrSample = null;
        lastMotionTime = 0;
        pendingStartTime = 0;
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
            pendingStartTime = now;
            hasPendingChange = true;
            self.postMessage({ type: 'SKIPPED', reason: 'init_wait' });
            return;
        }

        // 计算发生明显变化的网格数量
        const motionBlocks = countChangedBlocks(curSample, prevSample);
        prevSample = curSample;

        // 1. 超过 CURSOR_MAX_BLOCKS 个网格仍在变化（打字 / 翻页中）：重置静止计时器
        if (motionBlocks > CURSOR_MAX_BLOCKS) {
            if (!hasPendingChange) {
                hasPendingChange = true;
                pendingStartTime = now;
            }
            lastMotionTime = now;

            // 若未超过最大兜底等待时间，则继续等待动画结束；超时则放行强行识别一次
            if (now - pendingStartTime < MAX_WAIT_MS) {
                self.postMessage({ type: 'SKIPPED', reason: 'animating' });
                return;
            }
        }

        // 2. 画面静止（或仅光标在转）且没有待处理的新变化：直接跳过
        if (!hasPendingChange) {
            self.postMessage({ type: 'SKIPPED', reason: 'idle' });
            return;
        }

        // 3. 文本已停止变化，但持续静止时间未达到设定时长：继续等待
        if (now - lastMotionTime < STABLE_DURATION_MS && now - pendingStartTime < MAX_WAIT_MS) {
            self.postMessage({ type: 'SKIPPED', reason: 'waiting_stable' });
            return;
        }

        // 4. 满足触发条件：消耗标记，并与上一次执行 OCR 的画面对比去重
        hasPendingChange = false;
        pendingStartTime = 0;

        if (lastOcrSample && countChangedBlocks(curSample, lastOcrSample) <= CURSOR_MAX_BLOCKS) {
            self.postMessage({ type: 'SKIPPED', reason: 'idle' });
            return;
        }
        lastOcrSample = curSample;

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