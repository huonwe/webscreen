const translations = {
    en: {
        // App & Navigation
        app_title: "WebScreen Console",
        refresh: "Refresh",
        close: "Close",
        cancel: "Cancel",
        confirm: "OK",
        save_settings: "Save Settings",
        clear_local_storage: "Clear local storage",
        confirm_clear_local_storage: "Clear all local storage for this site, including global settings, device configurations, ignored devices, and language preference? The page will reload.",
        clear_local_storage_failed: "Could not clear local storage",

        // Device List & Status
        connected_devices: "Connected Devices",
        manage_devices_subtitle: "Manage and stream your devices",
        scanning_devices: "Scanning devices...",
        no_devices: "No devices connected",
        refreshed_found: "Refreshed: Found {n} devices",
        toggle_ignored_devices: "Show/Hide ignored devices",
        ignore_device: "Ignore Device",
        unignore_device: "Unignore Device",
        device_ignored: "Device ignored",
        device_unignored: "Device unignored",
        rename_device: "Edit Display Name",
        display_name: "Display Name",
        enter_display_name: "Enter device display name",
        device_renamed: "Display name updated",
        update_failed: "Failed to update display name",
        device_manager: "Device Manager",
        usb_devices: "USB Devices",
        wireless_devices: "Wireless Devices",
        other_devices: "Other Devices",
        no_wireless_devices: "No wireless devices yet: pair with \"Wireless Pair\", or use \"Connect Device\" with ip:port.",
        refresh_usb_auth: "Refresh & request authorization",
        no_usb_devices: "No USB device detected: connect the phone with a data cable and enable USB debugging.",
        remove_device: "Remove from list",
        confirm_remove_device: "Remove \"{name}\" from the device list? It will show up again when reconnected.",
        device_removed: "Removed from the list",
        rescanning_usb: "Refreshing USB devices and requesting authorization...",
        rescan_done: "Authorization requested again, please confirm on the phone",
        add_wireless_device: "Add Wireless Device",
        disconnect: "Disconnect",
        reconnect: "Reconnect",
        device_events: "Device Events",
        no_events: "No device events yet",
        device_disconnected: "Disconnected",
        reconnecting: "Reconnecting...",
        connecting: "Connecting...",
        wireless_added: "Wireless device connected",
        waiting_authorization: "Waiting for authorization",
        device_unavailable: "Device unavailable",
        usb_unauthorized_hint: "Waiting for USB debugging authorization: check the phone/tablet screen and tap \"Allow USB debugging\".",

        // Streaming & Remote Config
        start_stream: "Start Stream",
        starting_stream: "Starting stream...",
        join_stream: "Join Stream",
        joining_stream: "Joining stream...",
        streaming_active: "Streaming",
        remote_config: "Remote Config",
        remote_config_readonly_subtitle: "Active stream remote configuration (read-only)",
        runtime_parameters: "Runtime Parameters",

        // Pairing & Connection Modals
        connect_device: "Connect Device",
        connect_new_device: "Connect New Device",
        ip_address: "IP Address",
        port_default: "Port (Default 5555)",
        connect: "Connect",
        enter_ip: "Please enter IP address",
        connected_success: "Connected successfully!",
        connection_failed: "Connection failed",
        wireless_pair: "Wireless Pair",
        wireless_pair_title: "Wireless Pair",
        port_wireless: "Port (Wireless Debugging Port)",
        pair_code: "Pairing Code (6 digits)",
        pair: "Pair",
        fill_all_fields: "Please fill all fields",
        pair_success: "Pairing successful! You can connect now",
        pair_failed: "Pairing request failed",
        gamestream_pair: "GameStream Pair",
        gamestream_pair_title: "GameStream Pairing",
        gamestream_scanning: "Discovering GameStream hosts...",
        gamestream_no_devices: "No GameStream / Sunshine hosts found on the network",
        gamestream_pin: "Pairing PIN (4 digits)",
        gamestream_challenge: "Pair Challenge",
        gamestream_paired_badge: "Paired",
        gamestream_unpaired_badge: "Unpaired",

        // Global & Device Settings
        global_settings: "Global Settings",
        global_settings_subtitle: "Configure default universal stream parameters and system options",
        universal_stream_defaults: "Universal Stream Defaults",
        general_and_translation_settings: "General & Translation Settings",
        configure_device: "Configure Device",
        config_device_title: "Configure {serial}",
        configure_subtitle: "Set per-device streaming parameters (leave universal fields empty to inherit global settings)",
        universal_device_overrides: "Universal Parameters (Leave empty to inherit global)",
        driver_specific_settings: "Device Driver Parameters",
        inherit_global: "Inherit global setting",
        enabled: "Enabled (true)",
        disabled: "Disabled (false)",
        select_auto_default: "-- Default / Auto --",
        no_configurable_options: "No configurable options available",
        config_saved: "Configuration saved",
        default_config: "Default Config",
        error_no_device_config: "No device configuration found. Please set up the device configuration in the console.",

        // Global Setting Keys
        transport_protocol: "Transport Protocol",
        protocol_transport: "Transport Protocol",
        video_codec: "Video Codec",
        audio_codec: "Audio Codec",
        user_language: "User Language",
        language_to_translate: "Target Translation Language",
        translate_llm_endpoint: "Translation LLM Endpoint",
        translate_llm_secret: "Translation LLM API Key",
        translate_llm_model: "Translation LLM Model",
        translate_llm_system_prompt: "Translation System Prompt",
        translate_llm_system_prompt_galgame: "Galgame Translation System Prompt",

        // Stream & Driver Parameters
        audio: "Audio",
        video: "Video",
        control: "Control",
        display: "Display (Experimental)",
        video_encoder: "Video Encoder",
        video_bit_rate: "Video Bitrate",
        bitrate: "Bitrate (Mbps)",
        no_video_codec_options: "Disable video codec options",
        video_codec_options: "Video Codec Options",
        use_video_codec_options: "Send video_codec_options",
        codec_options_placeholder: "e.g. i-frame-interval=10",
        av_sync: "AV Sync",
        use_local_timestamp: "Use Local Timestamp",
        max_fps: "Max FPS",
        frame_rate: "Frame Rate",
        resolution: "Resolution",
        max_size: "Max Size",
        h264: "H.264",
        h265: "H.265",
        av1: "AV1",
        opus: "Opus",
        audio_enable: "Enable Audio",
        new_display: "New Display",
        leave_empty_disable: "Leave empty to disable",
        backend: "Backend",

        // Stream Viewer Controls
        fullscreen: "Fullscreen",
        volume_up: "Volume +",
        volume_down: "Volume -",
        power: "Power",
        back: "Back",
        home: "Home",
        menu: "Menu",
        rotate: "Rotate",
        set_clipboard: "Set Clipboard (Browser -> Device)",
        uhid_mouse: "UHID Mouse",
        uhid_keyboard: "UHID Keyboard",
        screen_off: "Screen OFF (privacy)",
        uhid_gamepad: "UHID Gamepad",
        gamestream_touch_on: "Touch mode enabled",
        gamestream_touch_off: "Touch mode disabled",
        gamestream_trackpad_on: "Trackpad mode enabled",

        // Errors & Auth
        error_from_server: "Error from server: {msg}",
        call_api_failed: "API request failed",
        error_empty_sdp_answer: "Received empty SDP answer from server. WebRTC connection cannot be established.",
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
        unlock_welcome: "Welcome to WebScreen Management Console",
        unlock_network_error: "Network error, please try again",
        unlock_verify_success: "Verification successful",
    },
    zh: {
        // 应用与导航
        app_title: "WebScreen 控制台",
        refresh: "刷新",
        close: "关闭",
        cancel: "取消",
        confirm: "确定",
        save_settings: "保存设置",
        clear_local_storage: "清空本地存储",
        confirm_clear_local_storage: "确定清空此网站的所有本地存储（包括全局设置、设备配置、忽略列表和语言偏好）？页面将重新加载。",
        clear_local_storage_failed: "无法清空本地存储",

        // 设备列表与状态
        connected_devices: "已连接设备",
        manage_devices_subtitle: "管理和串流您的设备",
        scanning_devices: "正在扫描设备...",
        no_devices: "未连接设备",
        refreshed_found: "已刷新：发现 {n} 台设备",
        toggle_ignored_devices: "显示/隐藏已忽略设备",
        ignore_device: "忽略设备",
        unignore_device: "取消忽略",
        device_ignored: "设备已忽略",
        device_unignored: "设备已取消忽略",
        rename_device: "修改显示名称",
        display_name: "显示名称",
        enter_display_name: "输入设备显示名称",
        device_renamed: "设备名称已更新",
        update_failed: "更新显示名称失败",
        device_manager: "设备管理",
        usb_devices: "USB 设备",
        wireless_devices: "无线设备",
        other_devices: "其他设备",
        no_wireless_devices: "暂无无线设备：用上方「无线配对」配对，或用「连接设备」填写 ip:端口 接入。",
        refresh_usb_auth: "刷新并请求授权",
        no_usb_devices: "未检测到 USB 设备：请用数据线连接手机，并在手机上开启 USB 调试。",
        remove_device: "从列表移除",
        confirm_remove_device: "从设备列表移除「{name}」？设备重新接入时会再次出现。",
        device_removed: "已从列表移除",
        rescanning_usb: "正在刷新 USB 设备并请求授权…",
        rescan_done: "已重新请求授权，请在手机上确认",
        add_wireless_device: "添加无线设备",
        disconnect: "断开",
        reconnect: "重连",
        device_events: "设备事件",
        no_events: "暂无设备事件",
        device_disconnected: "已断开",
        reconnecting: "正在重连…",
        connecting: "正在连接…",
        wireless_added: "无线设备已连接",
        waiting_authorization: "等待授权",
        device_unavailable: "设备不可用",
        usb_unauthorized_hint: "设备等待 USB 调试授权：请查看手机/平板屏幕，点击“允许 USB 调试”。",

        // 串流与远端配置
        start_stream: "开始串流",
        starting_stream: "正在启动串流...",
        join_stream: "加入串流",
        joining_stream: "正在加入串流...",
        streaming_active: "串流中",
        remote_config: "远端配置",
        remote_config_readonly_subtitle: "当前活动串流的远端配置参数（只读）",
        runtime_parameters: "运行时参数",

        // 配对与连接弹窗
        connect_device: "连接设备",
        connect_new_device: "连接新设备",
        ip_address: "IP 地址",
        port_default: "端口 (默认 5555)",
        connect: "连接",
        enter_ip: "请输入 IP 地址",
        connected_success: "连接成功！",
        connection_failed: "连接失败",
        wireless_pair: "无线配对",
        wireless_pair_title: "无线配对",
        port_wireless: "端口 (无线调试端口)",
        pair_code: "配对码 (6位)",
        pair: "配对",
        fill_all_fields: "请填写所有字段",
        pair_success: "配对成功！现在可以连接了",
        pair_failed: "配对请求失败",
        gamestream_pair: "GameStream 配对",
        gamestream_pair_title: "GameStream 设备配对",
        gamestream_scanning: "正在发现局域网 GameStream 设备...",
        gamestream_no_devices: "未发现网络中的 GameStream / Sunshine 服务端",
        gamestream_pin: "配对 PIN 码 (4位)",
        gamestream_challenge: "发起配对 Challenge",
        gamestream_paired_badge: "已配对",
        gamestream_unpaired_badge: "未配对",

        // 全局与单设备配置
        global_settings: "全局设置",
        global_settings_subtitle: "配置默认通用串流参数与系统全局选项",
        universal_stream_defaults: "通用串流默认参数",
        general_and_translation_settings: "全局与翻译选项",
        configure_device: "配置设备",
        config_device_title: "配置 {serial}",
        configure_subtitle: "设置单设备串流参数（通用项留空则继承全局设置）",
        universal_device_overrides: "通用参数（留空继承全局）",
        driver_specific_settings: "设备驱动参数",
        inherit_global: "继承全局设置",
        enabled: "启用 (true)",
        disabled: "禁用 (false)",
        select_auto_default: "-- 默认 / 自动 --",
        no_configurable_options: "暂无可配置项",
        config_saved: "配置已保存",
        default_config: "默认配置",
        error_no_device_config: "未找到设备配置，请在控制台设置设备配置。",

        // 全局设置字段
        transport_protocol: "传输协议",
        protocol_transport: "传输协议",
        video_codec: "视频编码",
        audio_codec: "音频编码",
        user_language: "用户语言",
        language_to_translate: "目标翻译语言",
        translate_llm_endpoint: "翻译 LLM 接口地址",
        translate_llm_secret: "翻译 LLM 密钥",
        translate_llm_model: "翻译 LLM 模型",
        translate_llm_system_prompt: "翻译系统提示词",
        translate_llm_system_prompt_galgame: "Galgame 翻译系统提示词",

        // 串流与驱动参数
        audio: "音频",
        video: "视频",
        control: "控制",
        display: "显示 (实验性)",
        video_encoder: "视频编码器",
        video_bit_rate: "视频比特率",
        bitrate: "比特率 (Mbps)",
        no_video_codec_options: "禁用视频编解码器选项",
        video_codec_options: "视频编解码器选项",
        use_video_codec_options: "携带 video_codec_options 参数",
        codec_options_placeholder: "例如: i-frame-interval=10",
        av_sync: "音画同步",
        use_local_timestamp: "使用本地时间戳",
        max_fps: "最大 FPS",
        frame_rate: "帧率",
        resolution: "分辨率",
        max_size: "最大尺寸",
        h264: "H.264",
        h265: "H.265",
        av1: "AV1",
        opus: "Opus",
        audio_enable: "启用音频",
        new_display: "新显示器",
        leave_empty_disable: "留空以禁用此选项",
        backend: "后端",

        // 串流界面工具栏
        fullscreen: "全屏",
        volume_up: "音量+",
        volume_down: "音量-",
        power: "电源",
        back: "返回",
        home: "主页",
        menu: "菜单",
        rotate: "旋转",
        set_clipboard: "设置剪贴板 (浏览器 -> 设备)",
        uhid_mouse: "UHID 鼠标",
        uhid_keyboard: "UHID 键盘",
        screen_off: "息屏（隐私保护）",
        uhid_gamepad: "UHID 手柄",
        gamestream_touch_on: "触控模式已开启",
        gamestream_touch_off: "触控模式已关闭",
        gamestream_trackpad_on: "触摸板模式已开启",

        // 错误与解锁页
        error_from_server: "服务器错误：{msg}",
        call_api_failed: "API 请求失败",
        error_empty_sdp_answer: "从服务器收到空的 SDP 答案，无法建立 WebRTC 连接。",
        unlock_now_verifying: "正在验证...",
        unlock_please_enter_pin: "请输入 PIN 码以解锁",
        unlock_enter_6_digits: "请输入 6 位 PIN 码",
        unlock_temporarily_locked: "已暂时锁定",
        unlock_too_many_attempts: "多次尝试失败，请在 {seconds} 秒后重试",
        unlock_locked_message: "由于多次尝试失败，已被锁定",
        unlock_wrong_password: "密码错误，还剩 {leftTries} 次尝试",
        unlock_forgot_password: "忘记密码？",
        unlock_contact_admin: "请联系管理员重置",
        unlock_success: "解锁成功",
        unlock_welcome: "欢迎进入 WebScreen 管理控制台",
        unlock_network_error: "网络错误，请重试",
        unlock_verify_success: "验证成功",
    },
    ja: {
        // アプリ・ナビゲーション
        app_title: "WebScreen コンソール",
        refresh: "更新",
        close: "閉じる",
        cancel: "キャンセル",
        confirm: "OK",
        save_settings: "設定を保存",
        clear_local_storage: "ローカルストレージを消去",
        confirm_clear_local_storage: "グローバル設定、デバイス設定、非表示リスト、言語設定を含むこのサイトのすべてのローカルストレージを消去しますか？ページが再読み込みされます。",
        clear_local_storage_failed: "ローカルストレージを消去できませんでした",

        // デバイス一覧・ステータス
        connected_devices: "接続済みデバイス",
        manage_devices_subtitle: "デバイスの管理とストリーミング",
        scanning_devices: "デバイスをスキャン中...",
        no_devices: "デバイスが接続されていません",
        refreshed_found: "更新完了：{n} 台のデバイスが見つかりました",
        toggle_ignored_devices: "非表示デバイスの表示切替",
        ignore_device: "デバイスを非表示",
        unignore_device: "非表示を解除",
        device_ignored: "デバイスを非表示にしました",
        device_unignored: "非表示を解除しました",
        rename_device: "表示名を変更",
        display_name: "表示名",
        enter_display_name: "デバイスの表示名を入力",
        device_renamed: "表示名を更新しました",
        update_failed: "表示名の更新に失敗しました",
        device_manager: "デバイス管理",
        usb_devices: "USB デバイス",
        wireless_devices: "ワイヤレスデバイス",
        other_devices: "その他のデバイス",
        no_wireless_devices: "ワイヤレスデバイスはありません。「ワイヤレスペアリング」または「デバイス接続」で ip:ポート を入力してください。",
        refresh_usb_auth: "更新して認証を要求",
        no_usb_devices: "USB デバイスが見つかりません。データケーブルで接続し、USB デバッグを有効にしてください。",
        remove_device: "リストから削除",
        confirm_remove_device: "「{name}」をリストから削除しますか？再接続すると再び表示されます。",
        device_removed: "リストから削除しました",
        rescanning_usb: "USB デバイスを更新し、認証を要求しています…",
        rescan_done: "認証を再要求しました。端末で確認してください",
        add_wireless_device: "ワイヤレスデバイスを追加",
        disconnect: "切断",
        reconnect: "再接続",
        device_events: "デバイスイベント",
        no_events: "デバイスイベントはありません",
        device_disconnected: "切断しました",
        reconnecting: "再接続中…",
        connecting: "接続中…",
        wireless_added: "ワイヤレスデバイスを接続しました",
        waiting_authorization: "認証待ち",
        device_unavailable: "デバイス利用不可",
        usb_unauthorized_hint: "USB デバッグの認証待ちです。端末の画面で「USB デバッグを許可」をタップしてください。",

        // ストリーミング・リモート設定
        start_stream: "ストリーミング開始",
        starting_stream: "ストリーミングを開始中...",
        join_stream: "ストリーミングに参加",
        joining_stream: "ストリーミングに参加中...",
        streaming_active: "配信中",
        remote_config: "リモート設定",
        remote_config_readonly_subtitle: "現在配信中のリモート設定パラメータ（読み取り専用）",
        runtime_parameters: "ランタイムパラメータ",

        // ペアリング・接続モーダル
        connect_device: "デバイス接続",
        connect_new_device: "新しいデバイスを接続",
        ip_address: "IPアドレス",
        port_default: "ポート (デフォルト 5555)",
        connect: "接続",
        enter_ip: "IPアドレスを入力してください",
        connected_success: "接続成功！",
        connection_failed: "接続失敗",
        wireless_pair: "ワイヤレスペアリング",
        wireless_pair_title: "ワイヤレスペアリング",
        port_wireless: "ポート (ワイヤレスデバッグポート)",
        pair_code: "ペアリングコード (6桁)",
        pair: "ペアリング",
        fill_all_fields: "すべてのフィールドを入力してください",
        pair_success: "ペアリング成功！接続可能です",
        pair_failed: "ペアリング要求失敗",
        gamestream_pair: "GameStream ペアリング",
        gamestream_pair_title: "GameStream デバイスペアリング",
        gamestream_scanning: "LAN内の GameStream ホストを検索中...",
        gamestream_no_devices: "ネットワーク上に GameStream / Sunshine ホストが見つかりません",
        gamestream_pin: "ペアリング PIN (4桁)",
        gamestream_challenge: "ペアリング Challenge 送信",
        gamestream_paired_badge: "ペアリング済",
        gamestream_unpaired_badge: "未ペアリング",

        // グローバル・デバイス設定
        global_settings: "グローバル設定",
        global_settings_subtitle: "デフォルトの共通配信パラメータとシステム設定",
        universal_stream_defaults: "共通ストリーミング既定値",
        general_and_translation_settings: "全般・翻訳設定",
        configure_device: "デバイス設定",
        config_device_title: "{serial} の設定",
        configure_subtitle: "個別デバイスの配信設定（共通項目を空欄にするとグローバル設定を継承します）",
        universal_device_overrides: "共通パラメータ（空欄でグローバル設定を継承）",
        driver_specific_settings: "デバイスドライバー設定",
        inherit_global: "グローバル設定を継承",
        enabled: "有効 (true)",
        disabled: "無効 (false)",
        select_auto_default: "-- デフォルト / 自動 --",
        no_configurable_options: "設定可能な項目がありません",
        config_saved: "設定を保存しました",
        default_config: "デフォルト設定",
        error_no_device_config: "デバイス設定が見つかりません。コンソールでデバイス設定を行ってください。",

        // グローバル設定キー
        transport_protocol: "転送プロトコル",
        protocol_transport: "転送プロトコル",
        video_codec: "ビデオコーデック",
        audio_codec: "オーディオコーデック",
        user_language: "表示言語",
        language_to_translate: "翻訳先言語",
        translate_llm_endpoint: "翻訳 LLM エンドポイント",
        translate_llm_secret: "翻訳 LLM API キー",
        translate_llm_model: "翻訳 LLM モデル",
        translate_llm_system_prompt: "翻訳システムプロンプト",
        translate_llm_system_prompt_galgame: "Galgame 翻訳システムプロンプト",

        // ストリーミング・ドライバー項目
        audio: "オーディオ",
        video: "ビデオ",
        control: "コントロール",
        display: "ディスプレイ (実験的)",
        video_encoder: "ビデオエンコーダ",
        video_bit_rate: "ビデオビットレート",
        bitrate: "ビットレート (Mbps)",
        no_video_codec_options: "ビデオコーデックオプションを無効化",
        video_codec_options: "ビデオコーデックオプション",
        use_video_codec_options: "video_codec_options を送信",
        codec_options_placeholder: "例: i-frame-interval=10",
        av_sync: "AV同期",
        use_local_timestamp: "ローカルタイムスタンプを使用",
        max_fps: "最大 FPS",
        frame_rate: "フレームレート",
        resolution: "解像度",
        max_size: "最大サイズ",
        h264: "H.264",
        h265: "H.265",
        av1: "AV1",
        opus: "Opus",
        audio_enable: "オーディオを有効化",
        new_display: "新しいディスプレイ",
        leave_empty_disable: "無効にする場合は空欄",
        backend: "バックエンド",

        // ビューアー操作コントロール
        fullscreen: "全画面",
        volume_up: "音量 +",
        volume_down: "音量 -",
        power: "電源",
        back: "戻る",
        home: "ホーム",
        menu: "メニュー",
        rotate: "回転",
        set_clipboard: "クリップボード送信 (ブラウザ -> デバイス)",
        uhid_mouse: "UHID マウスモード",
        uhid_keyboard: "UHID キーボードモード",
        screen_off: "画面オフ（プライバシー保護）",
        uhid_gamepad: "UHID ゲームパッドモード",
        gamestream_touch_on: "タッチモードを有効にしました",
        gamestream_touch_off: "タッチモードを無効にしました",
        gamestream_trackpad_on: "トラックパッドモードを有効にしました",

        // エラー・ロック解除画面
        error_from_server: "サーバーエラー：{msg}",
        call_api_failed: "APIリクエストに失敗しました",
        error_empty_sdp_answer: "サーバーから空のSDPアンサーが受信されました。WebRTC接続を確立できません。",
        unlock_now_verifying: "検証中...",
        unlock_please_enter_pin: "ロック解除するにはPINを入力してください",
        unlock_enter_6_digits: "6桁のPINコードを入力してください",
        unlock_temporarily_locked: "一時的にロック中",
        unlock_too_many_attempts: "失敗の試み回数が多すぎます。{seconds} 秒後にもう一度お試しください",
        unlock_locked_message: "複数回の失敗によりロックされています",
        unlock_wrong_password: "PINが間違っています。残り {leftTries} 回の試行があります",
        unlock_forgot_password: "PINを忘れた方",
        unlock_contact_admin: "管理者に連絡してリセットしてください",
        unlock_success: "ロック解除成功",
        unlock_welcome: "WebScreen 管理コンソールへようこそ",
        unlock_network_error: "ネットワークエラー。もう一度お試しください",
        unlock_verify_success: "検証成功",
    }
};

class I18n {
    constructor() {
        const saved = localStorage.getItem('webscreen_lang');
        const browserLang = (navigator.language || 'en').slice(0, 2);
        this.lang = ['en', 'zh', 'ja'].includes(saved)
            ? saved
            : (['en', 'zh', 'ja'].includes(browserLang) ? browserLang : 'en');
    }

    setLang(lang) {
        if (!translations[lang]) return;
        this.lang = lang;
        localStorage.setItem('webscreen_lang', lang);
        this.apply();
    }

    t(key, params = {}) {
        let str = translations[this.lang]?.[key] || translations['en']?.[key] || key;
        if (params && typeof params === 'object') {
            Object.keys(params).forEach((k) => {
                str = str.replace(new RegExp(`\\{${k}\\}`, 'g'), String(params[k]));
            });
        }
        return str;
    }

    apply() {
        document.documentElement.lang = this.lang === 'zh' ? 'zh-CN' : this.lang;

        document.querySelectorAll('[data-i18n]').forEach((el) => {
            const key = el.getAttribute('data-i18n');
            if (el.tagName === 'INPUT' && el.hasAttribute('placeholder')) {
                el.setAttribute('placeholder', this.t(key));
            } else {
                el.textContent = this.t(key);
            }
        });

        document.querySelectorAll('[data-i18n-placeholder]').forEach((el) => {
            const key = el.getAttribute('data-i18n-placeholder');
            el.setAttribute('placeholder', this.t(key));
        });

        document.querySelectorAll('[data-i18n-title]').forEach((el) => {
            const key = el.getAttribute('data-i18n-title');
            el.setAttribute('title', this.t(key));
        });

        document.querySelectorAll('.lang-btn').forEach((btn) => {
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

export const i18n = new I18n();
export default i18n;
window.i18n = i18n; // 保留全局引用以兼容未改造的旧脚本

if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => i18n.apply());
} else {
    i18n.apply();
}