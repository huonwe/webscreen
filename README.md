# Webscreen

在浏览器里查看并操控 Android 手机画面 —— 自托管、基于 WebRTC，Android 侧基于 [scrcpy](https://github.com/Genymobile/scrcpy)。

[For Android apk (rooted) and FnOS, please see this fork](https://github.com/youyoudezhuzhu/webscreen)

## ℹ️ About

![screenshot](doc/assets/screenshot.png)

---

## ✨ 本分支新增的功能

### 1. 飞牛 OS（fnOS）原生应用 —— 让 NAS 变成投屏服务器

上游需要自己编译并在命令行里运行；本分支提供现成的 **`.fpk`**：装到飞牛 NAS 后，**NAS 本身就是 WebScreen 服务端**，通过 **USB 有线 ADB** 或 **Wi-Fi ADB** 管理多台 Android 设备。

- 手机端**完全不用安装任何软件**，只需打开 USB 调试（或无线调试）并授权
- 设备列表区分 USB / 无线、显示授权状态、支持自定义命名、USB 热插拔、多设备并存
- 桌面入口「WebScreen 控制台」，浏览器访问 `http://<NAS 的 IP>:8079`
- 详细说明：[`doc/fnos-fpk.md`](doc/fnos-fpk.md)

### 2. Android 版 —— 手机自己就是服务端（无需电脑 / adb / Termux）

rk3588 supports:
- HDMI-in video streaming
- H.264/H.265 with RKMPP hardware encoding
- Audio is not supported yet

Note: only OrangePi 5 Plus is tested

## Prerequisites

- 要求：arm64、Android 8+、已 root
- 详细说明：[`doc/android-apk.md`](doc/android-apk.md)

### 3. 受控端黑屏（隐私保护）

侧边栏一键让被控手机**屏幕熄灭但不锁屏** —— 设备保持唤醒，串流与触控全部照常，适合「需要遮挡画面、但还要继续操作」的场景。

- 只关闭背光，**不按电源键**，因此**不会锁屏**
- 关闭前记住原亮度，恢复时写回；离开页面自动恢复，不会把设备留在黑屏
- 按钮带**可见状态反馈**：成功或失败原因直接显示在页面上

### 4. 侧边栏虚拟键盘

点侧边栏键盘图标弹出屏幕键盘，直接向受控设备注入按键（走已有的 UHID 通道）。

- 5 行全键盘；`Ctrl` / `Shift` / `Alt` 点击锁定（适配触屏，无需长按）
- 面板可通过顶部把手拖动，隐藏时自动释放所有按键，避免卡键
- **可用于在锁屏界面输入密码**

### 5. 稳定性与体验改进

- 修复 Android 15 / 16 上「点启动就闪退」（ROM 的前台服务校验 + 自杀式 `pkill`）
- 修复 `failed to ensure agent: EOF`（根因是 scrcpy 会话失败路径漏清理，会污染后续连接）
- App 界面状态自动校正，不再需要手动「停止 → 启动」
- 前端资源禁缓存，升级后无需强制刷新浏览器

---

## 该下载哪个文件

| 你的情况 | 下载这个 | 装在哪里 | 手机端需要做什么 |
|---|---|---|---|
| 有**飞牛 NAS（fnOS）**，想让 NAS 管理手机 | `webscreen_<版本>.fpk` | **只装 NAS** | **不用装任何软件**，只需打开 USB 调试或无线调试并授权 |
| 手机已 **root**，没有 NAS / 不想用 NAS | `WebScreen-<版本>.apk` | **只装手机** | 不用 NAS、不用 adb、不用 Termux、不用电脑 |

> **无需同时安装两个文件** —— 两种方案得到的是同一个东西：一个在浏览器里操作手机画面的页面。

### 方案 A：飞牛 NAS —— 安装 `.fpk`

NAS 装上后就是 WebScreen 服务端，通过 USB 有线 ADB 或 Wi-Fi ADB 连接手机；局域网内任意浏览器访问 `http://<NAS 的 IP>:8079` 进入控制台。
安装 / 升级 / 端口 / 已知限制：[`doc/fnos-fpk.md`](doc/fnos-fpk.md)

### 方案 B：已 root 的 Android 手机 —— 安装 `.apk`

```
已 root 手机 → 安装 APK → 授予 root → 点「启动」
             → 局域网浏览器访问 http://<手机 IP>:8079
```

完整说明（含功能限制）：[`doc/android-apk.md`](doc/android-apk.md)

---

## 完整功能

Android（基于 [scrcpy](https://github.com/Genymobile/scrcpy)）：

- 视频 / 音频 / 控制
- UHID 虚拟设备：鼠标、键盘、手柄
- 剪贴板同步
- 多指触控（带压力感应）
- H.264 / H.265
- 多路连接
- 受控端黑屏（见上）

Linux（Xvfb / Xorg / Sway）：视频、控制、触控、H.264/H.265、GPU（Xorg/Sway）

## 其他平台（上游用法）

同一份代码也可运行在 Termux、Linux、Windows、macOS（amd64 / arm64），或使用 Docker：

For rk3588 HDMI-in, all of the following are required:

- A vendor kernel with HDMI RX support (rk3588-hdmirx enabled)
- An FFmpeg build with RKMPP support, such as `/usr/lib/jellyfin-ffmpeg/ffmpeg`
- Correct read/write permissions for `/dev/mpp_service`

The HDMI-in adapter currently supports video only. Audio capture is not supported.

```bash
wget https://raw.githubusercontent.com/youyoudezhuzhu/webscreen/refs/heads/main/docker-compose.yml
docker compose up -d
```

默认端口 `8079`，可用 `-port 8080` 指定；支持 6 位 PIN（`-pin 555555`，默认为空）。
客户端需要支持 WebRTC（H.264 High Profile 或 H.265 Main Profile）的浏览器。

## 常见问题

- **自定义 Android 设备上 MediaCodec 0x80001001 崩溃**（硬编码 H.264 High Profile 所致）：把 `video_codec_options` 设为 `profile=1`

## 许可证

AGPL-3.0 —— 见 [LICENSE](LICENSE)。
