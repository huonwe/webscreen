// global-settings.js
const GLOBAL_SETTINGS_KEY = 'global_settings';

const DEFAULT_SETTINGS = {
    transport_protocol: "webrtc",
    video_codec: "h264",
    audio_codec: "opus",
    user_language: "zh-CN",
    language_to_translate: "ja-JP",
    translate_llm_endpoint: "",
    translate_llm_secret: "",
    translate_llm_model: "",
    translate_llm_system_prompt: `你是一个专业的本地化翻译专家。请将用户输入的文本准确翻译成自然、贴合语境的用户指定语言。
严格遵守以下规则：
1. 只输出最终的译文，绝对不要输出任何解释、注释、拼音、原文对照或多余的标点符号。
2. 输入文本来自 OCR 识别，若存在明显的个别错别字、多余空格或识别噪点，请结合语境自动修正后再翻译。
3. 保留原文中的特殊符号（如「」、……、！？、♪ 等）。`,
    translate_llm_system_prompt_galgame: `你是一个专业的日文 Galgame（视觉小说）本地化翻译专家。请将用户输入的日文游戏文本（对话、心理描写或旁白）准确翻译成自然、贴合二次元语境的用户指定语言。
严格遵守以下规则：
1. 只输出最终的用户指定语言译文，绝对不要输出任何解释、注释、拼音、原文对照或多余的标点符号。
2. 保留角色语气、口癖和人物关系称谓（如「お兄ちゃん」译为“哥哥”，「先輩」译为“前辈”等），对话口语化，旁白细腻流畅。
3. 输入文本来自 OCR 识别，若存在明显的个别错别字、多余空格或识别噪点，请结合日文语境自动修正后再翻译。
4. 保留原文中的特殊符号（如「」、……、！？、♪ 等）。`,
    _device_config: {},
};

// 1. 私有内存缓存，不暴露到 window 上
let settingsCache = null;

function initSettings() {
    if (settingsCache) return settingsCache;
    try {
        const stored = localStorage.getItem(GLOBAL_SETTINGS_KEY);
        // 使用浅拷贝合并默认值，防止旧版缓存缺少新增字段
        settingsCache = stored
            ? { ...DEFAULT_SETTINGS, ...JSON.parse(stored) }
            : { ...DEFAULT_SETTINGS };
        if (!stored) {
            console.log('Initializing settings with default values');
            localStorage.setItem(GLOBAL_SETTINGS_KEY, JSON.stringify(settingsCache));
        }
    } catch (e) {
        console.error('Failed to load settings:', e);
        settingsCache = { ...DEFAULT_SETTINGS };
    }
    return settingsCache;
}

// 2. 高频读取走内存
export function getSetting(key) {
    const current = initSettings();
    return current[key];
}

// 3. 修改时同时更新内存和 localStorage
export function updateSetting(key, value) {
    const current = initSettings();
    current[key] = value;
    try {
        localStorage.setItem(GLOBAL_SETTINGS_KEY, JSON.stringify(current));
    } catch (e) {
        console.error('Failed to save settings:', e);
    }
}
