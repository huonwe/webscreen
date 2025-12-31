const translations = {
    en: {
        app_title: "WebScreen Console",
        refresh: "Refresh",
        connected_devices: "Connected Devices",
        manage_devices_subtitle: "Manage and stream your Android devices",
        wireless_pair: "Wireless Pair",
        connect_device: "Connect Device",
        scanning_devices: "Scanning devices...",
        no_devices: "No devices connected",
        connect_new_device: "Connect New Device",
        ip_address: "IP Address",
        port_default: "Port (Default 5555)",
        cancel: "Cancel",
        connect: "Connect",
        wireless_pair_title: "Wireless Pair",
        port_wireless: "Port (Wireless Debugging Port)",
        pair_code: "Pairing Code (6 digits)",
        pair: "Pair",
        configure_device: "Configure Device",
        configure_subtitle: "Set streaming parameters",
        avsync: "AV Sync",
        max_fps: "Max FPS",
        max_size: "Max Size",
        bitrate: "Bitrate (Mbps)",
        video_codec: "Video Codec",
        h264_compatible: "H.264 (Compatible)",
        h265_efficient: "H.265 (Efficient)",
        codec_options_placeholder: "e.g. i-frame-interval=10",
        audio_enable: "Enable Audio",
        new_display: "New Display",
        leave_empty_disable: "Leave empty to disable",
        save_settings: "Save Settings",
        refreshed_found: "Refreshed: Found {n} devices",
        enter_ip: "Please enter IP address",
        connected_success: "Connected successfully!",
        connection_failed: "Connection failed",
        fill_all_fields: "Please fill all fields",
        pair_success: "Pairing successful! You can connect now",
        pair_failed: "Pairing request failed",
        starting_stream: "Starting stream...",
        config_saved: "Configuration saved",
        default_config: "Default Config",
        start_stream: "Start Stream",
        device_manager: "Device Manager",
        reconnect: "Connect (Reconnect)",
        fullscreen: "Fullscreen",
        volume_up: "Volume +",
        volume_down: "Volume -",
        power: "Power",
        back: "Back",
        home: "Home",
        menu: "Menu",
        rotate: "Rotate",
        set_clipboard: "Set Clipboard (Browser -> Device)",
        mouse_mode: "UHID Mouse",
        keyboard_mode: "UHID Keyboard",
        gamepad_mode: "UHID Gamepad",
        video: "Video",
        display: "Display (Experimental)",
        config_device_title: "Configure {serial}",
        video_codec_options: "video_codec_options",
        error_from_server: "Error from server: {msg}",
        call_api_failed: "API request failed",

        unlock_now_verifying: "Verifying...",
        unlock_please_enter_pin: "Please enter the PIN to unlock",
        unlock_enter_6_digits: "Please enter 6-digit PIN code",
        unlock_temporarily_locked: "Temporarily Locked",
        unlock_too_many_attempts: "Too many failed attempts. Please try again in {seconds} seconds",
        unlock_locked_message: "Locked due to multiple failed attempts",
        unlock_wrong_password: "Wrong PIN, {leftTries} attempts remaining",
        unlock_forgot_password: "Forgot PIN?",
        unlock_contact_admin: "Please contact the administrator to reset",
        unlock_success: "Unlock Successful",
        unlock_welcome: "Welcome to Webscreen Management Console",
        unlock_network_error: "Network error, please try again",
        unlock_verify_success: "Verification successful",

        error_empty_sdp_answer: "Received empty SDP answer from server. WebRTC connection cannot be established.",
    },
    zh: {
        app_title: "WebScreen 控制台",
        refresh: "刷新",
        connected_devices: "已连接设备",
        manage_devices_subtitle: "管理和串流您的 Android 设备",
        wireless_pair: "无线配对",
        connect_device: "连接设备",
        scanning_devices: "正在扫描设备...",
        no_devices: "未连接设备",
        connect_new_device: "连接新设备",
        ip_address: "IP 地址",
        port_default: "端口 (默认 5555)",
        cancel: "取消",
        connect: "连接",
        wireless_pair_title: "无线配对",
        port_wireless: "端口 (无线调试端口)",
        pair_code: "配对码 (6位)",
        pair: "配对",
        configure_device: "配置设备",
        configure_subtitle: "设置流传输参数",
        avsync: "音画同步",
        max_fps: "最大 FPS",
        max_size: "最大尺寸",
        bitrate: "比特率 (Mbps)",
        video_codec: "视频编码",
        h264_compatible: "H.264 (兼容)",
        h265_efficient: "H.265 (高效)",
        codec_options_placeholder: "例如: i-frame-interval=10",
        audio_enable: "启用音频",
        new_display: "New Display",
        leave_empty_disable: "留空以禁用此选项",
        save_settings: "保存设置",
        refreshed_found: "已刷新: 发现 {n} 台设备",
        enter_ip: "请输入 IP 地址",
        connected_success: "连接成功!",
        connection_failed: "连接失败",
        fill_all_fields: "请填写所有字段",
        pair_success: "配对成功! 现在可以连接了",
        pair_failed: "配对请求失败",
        starting_stream: "正在启动串流...",
        config_saved: "配置已保存",
        default_config: "默认配置",
        start_stream: "开始串流",
        device_manager: "设备管理",
        reconnect: "连接(重新连接)",
        fullscreen: "全屏",
        volume_up: "音量+",
        volume_down: "音量-",
        power: "电源",
        back: "返回",
        home: "主页",
        menu: "菜单",
        rotate: "旋转",
        set_clipboard: "设置剪贴板 (Browser -> Device)",
        mouse_mode: "UHID鼠标",
        keyboard_mode: "UHID键盘",
        gamepad_mode: "UHID手柄",
        video: "视频",
        display: "显示 (实验性)",
        config_device_title: "配置 {serial}",
        video_codec_options: "video_codec_options",
        error_from_server: "服务器错误: {msg}",
        call_api_failed: "API请求失败",

        unlock_now_verifying: "正在验证...",
        unlock_please_enter_pin: "请输入 PIN 码以解锁",
        unlock_enter_6_digits: "请输入 6 位 PIN 码",
        unlock_temporarily_locked: "已暂时锁定",
        unlock_too_many_attempts: "多次尝试失败，请在 {seconds} 秒后重试",
        unlock_locked_message: "由于多次尝试失败，已被锁定",
        unlock_wrong_password: "密码错误，还剩 {leftTries} 次尝试",
        unlock_forgot_password: "忘记密码?",
        unlock_contact_admin: "请联系管理员重置",
        unlock_success: "解锁成功",
        unlock_welcome: "欢迎进入 Webscreen 管理控制台",
        unlock_network_error: "网络错误，请重试",
        unlock_verify_success: "验证成功",

        error_empty_sdp_answer: "从服务器收到空的 SDP 答案，无法建立 WebRTC 连接。",
    },
    ja: {
        app_title: "WebScreen コンソール",
        refresh: "更新",
        connected_devices: "接続済みデバイス",
        manage_devices_subtitle: "Androidデバイスの管理とストリーミング",
        wireless_pair: "ワイヤレスペアリング",
        connect_device: "デバイス接続",
        scanning_devices: "デバイスをスキャン中...",
        no_devices: "デバイスが接続されていません",
        connect_new_device: "新しいデバイスを接続",
        ip_address: "IPアドレス",
        port_default: "ポート (デフォルト 5555)",
        cancel: "キャンセル",
        connect: "接続",
        wireless_pair_title: "ワイヤレスペアリング",
        port_wireless: "ポート (ワイヤレスデバッグポート)",
        pair_code: "ペアリングコード (6桁)",
        pair: "ペアリング",
        configure_device: "デバイス設定",
        configure_subtitle: "ストリーミングパラメータの設定",
        avsync: "AV同期",
        max_fps: "最大 FPS",
        max_size: "最大サイズ",
        bitrate: "ビットレート (Mbps)",
        video_codec: "ビデオコーデック",
        h264_compatible: "H.264 (互換性)",
        h265_efficient: "H.265 (高効率)",
        codec_options_placeholder: "例: i-frame-interval=10",
        audio_enable: "オーディオを有効化",
        new_display: "新しいディスプレイ",
        leave_empty_disable: "無効にする場合は空欄",
        save_settings: "設定を保存",
        refreshed_found: "更新完了: {n} 台のデバイスが見つかりました",
        enter_ip: "IPアドレスを入力してください",
        connected_success: "接続成功!",
        connection_failed: "接続失敗",
        fill_all_fields: "すべてのフィールドを入力してください",
        pair_success: "ペアリング成功! 接続可能です",
        pair_failed: "ペアリング要求失敗",
        starting_stream: "ストリーミングを開始中...",
        config_saved: "設定を保存しました",
        default_config: "デフォルト設定",
        start_stream: "ストリーミング開始",
        device_manager: "デバイス管理",
        reconnect: "接続 (再接続)",
        fullscreen: "全画面",
        volume_up: "音量 +",
        volume_down: "音量 -",
        power: "電源",
        back: "戻る",
        home: "ホーム",
        menu: "メニュー",
        rotate: "回転",
        set_clipboard: "クリップボード設定 (Browser -> Device)",
        mouse_mode: "UHIDマウスモード",
        keyboard_mode: "UHIDキーボードモード",
        gamepad_mode: "UHIDゲームパッドモード",
        video: "ビデオ",
        display: "ディスプレイ (実験的)",
        config_device_title: "{serial} の設定",
        video_codec_options: "video_codec_options",
        error_from_server: "サーバーエラー: {msg}",
        call_api_failed: "APIリクエストに失敗しました",

        unlock_now_verifying: "検証中...",
        unlock_please_enter_pin: "ロック解除するにはPINを入力してください",
        unlock_enter_6_digits: "6桁のPINコードを入力してください",
        unlock_temporarily_locked: "一時的にロック中",
        unlock_too_many_attempts: "失敗の試み回数が多すぎます。{seconds} 秒後にもう一度お試しください",
        unlock_locked_message: "複数回の失敗により、ロックされています",
        unlock_wrong_password: "PINが間違っています。残り {leftTries} 回の試行があります",
        unlock_forgot_password: "PINを忘れた方",
        unlock_contact_admin: "管理者に連絡してリセットしてください",
        unlock_success: "ロック解除成功",
        unlock_welcome: "Webscreen 管理コンソールへようこそ",
        unlock_network_error: "ネットワークエラー。もう一度お試しください",
        unlock_verify_success: "検証成功",

        error_empty_sdp_answer: "サーバーから空のSDPアンサーが受信されました。WebRTC接続を確立できません。",
    }
};

class I18n {
    constructor() {
        this.lang = localStorage.getItem('webscreen_lang') || navigator.language.slice(0, 2);
        if (!['en', 'zh', 'ja'].includes(this.lang)) {
            this.lang = 'en';
        }
        // Initial apply will be called manually or on DOMContentLoaded
    }

    setLang(lang) {
        if (!translations[lang]) return;
        this.lang = lang;
        localStorage.setItem('webscreen_lang', lang);
        this.apply();
    }

    t(key, params = {}) {
        let str = translations[this.lang]?.[key] || translations['en']?.[key] || key;
        Object.keys(params).forEach(k => {
            str = str.replace(`{${k}}`, params[k]);
        });
        return str;
    }

    apply() {
        document.querySelectorAll('[data-i18n]').forEach(el => {
            const key = el.getAttribute('data-i18n');
            if (el.tagName === 'INPUT' && el.getAttribute('placeholder')) {
                 el.setAttribute('placeholder', this.t(key));
            } else {
                 el.textContent = this.t(key);
            }
        });
        
        document.querySelectorAll('[data-i18n-title]').forEach(el => {
            const key = el.getAttribute('data-i18n-title');
            el.setAttribute('title', this.t(key));
        });

        // Update active state of language buttons if they exist
        document.querySelectorAll('.lang-btn').forEach(btn => {
            if (btn.dataset.lang === this.lang) {
                btn.classList.add('active', 'text-[var(--md-sys-color-primary)]', 'font-bold');
                btn.classList.remove('text-gray-400');
            } else {
                btn.classList.remove('active', 'text-[var(--md-sys-color-primary)]', 'font-bold');
                btn.classList.add('text-gray-400');
            }
        });
    }
}

const i18n = new I18n();

// Auto-apply on load
document.addEventListener('DOMContentLoaded', () => {
    i18n.apply();
});
