# 📺 Webscreen

<p align="center">
  <a href="https://github.com/huonwe/webscreen/releases"><img src="https://img.shields.io/github/v/release/huonwe/webscreen?style=flat-square&color=blue" alt="GitHub Release"></a>
  <a href="https://github.com/huonwe/webscreen/stargazers"><img src="https://img.shields.io/github/stars/huonwe/webscreen?style=flat-square&color=yellow" alt="GitHub Stars"></a>
  <a href="https://hub.docker.com/r/dukihiroi/webscreen"><img src="https://img.shields.io/docker/pulls/dukihiroi/webscreen?style=flat-square&logo=docker&label=Docker%20Pulls" alt="Docker Pulls"></a>
  <a href="https://github.com/huonwe/webscreen/commits/main"><img src="https://img.shields.io/github/last-commit/huonwe/webscreen?style=flat-square" alt="Last Commit"></a>
  <a href="https://www.gnu.org/licenses/agpl-3.0"><img src="https://img.shields.io/badge/License-AGPL_v3-purple?style=flat-square" alt="License: AGPL v3"></a>
</p>
<p align="center">
  <img src="https://img.shields.io/badge/Deployment-Single_Binary-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Single Binary">
  <img src="https://img.shields.io/badge/Arch-amd64%20%7C%20arm64-success?style=flat-square" alt="Architectures">
  <img src="https://img.shields.io/badge/Platform-Linux%20%7C%20Windows%20%7C%20macOS%20%7C%20Termux-lightgrey?style=flat-square" alt="Platforms">
  <img src="https://img.shields.io/badge/AI-WebSR%20%2B%20LLM%20OCR-ff69b4?style=flat-square" alt="AI Enhanced">
</p>

**Turn any browser into an AI-enhanced universal remote screen.**  
Stream **Android**, **GameStream (Sunshine/Wolf)**, **Linux Desktops**, and **RK3588 HDMI-in** from a single self-hosted Go binary — supercharged with client-side **WebSR Super Resolution (Anime4K models)** and **LLM OCR Translation**.

[简体中文](./README.zh.md) • [日本語](./README.jp.md)

---

<!-- 💡 建议在此处直接拖入两段 5-8 秒的实机录屏（GitHub 会自动渲染为原生嵌入式视频播放器）：
     1. 浏览器内开启/关闭 WebSR 超分前后的游戏画面锐度对比
     2. 框选外语游戏/直播画面，触发 LLM OCR 实时翻译弹窗 -->
<p align="center">
  <img src="./doc/assets/timer-difference.png" alt="Webscreen Showcase" width="85%">
</p>

## ⚡ Why Webscreen?

Most remote streaming tools force you to juggle separate apps, suffer blurry low-bitrate streams, or manually screenshot foreign text. **Webscreen unifies everything into one lightweight web interface:**

* 🔍 **Client-Side WebSR Super Resolution**: Upscale low-bitrate streams directly in your browser using WebGPU/WebGL shaders — get crisp text and sharp game graphics over bandwidth-limited networks without extra server GPU load.
* 🌐 **Instant LLM OCR Translation**: Playing untranslated games or watching foreign streams? Select any region on screen to extract and translate text on the fly via your preferred LLM endpoint.
* 🧩 **All-in-one Streaming Hub**: Manage physical Android phones, **Redroid** cloud phones, **Sunshine/Wolf** gaming rigs, headless **Linux** desktops, and **RK3588 HDMI-in** capture feeds from one unified dashboard.
* 🚀 **Zero Client Setup**: Runs natively on Linux, Windows, macOS, Docker, and **Android Termux** (`amd64` & `arm64`). Access everything via WebRTC or WebSocket from any modern browser — no client app required.

## 🎯 Built For

* **NAS & Homelab Enthusiasts**: Host Redroid cloud phones, Linux workspaces, and Wolf game streaming inside a single container on Unraid, TrueNAS, or fnOS.
* **Gamers & Stream Viewers**: Play foreign-language games or mobile titles from a desktop/tablet browser with real-time AI translation and visual upscaling.
* **SBC & Hardware Tinkerers**: Stream hardware-encoded H.264/H.265 video directly from OrangePi 5 Plus (`rk3588-hdmirx`) into any browser.

## ✨ Supported Backends & Capabilities

| Backend | Video & Audio | Input & Control | Extras |
| :--- | :--- | :--- | :--- |
| **Android (`scrcpy`)** | Low-latency H.264 / H.265 + Audio | Multi-touch (pressure), UHID Mouse / Keyboard / Gamepad | Wireless ADB pairing, Clipboard sync, Redroid ready |
| **GameStream (`Sunshine` / `Wolf`)** | Low-latency H.264 / H.265 + Audio | Mouse, Keyboard, Touch, Gamepad | Full desktop & cloud gaming support |
| **Linux (`Xvfb` / `Xorg` / `Sway`)** | H.264 / H.265 (GPU accelerated on Xorg/Sway) | Mouse, Keyboard, Touch (`uinput` / `seatd`) | Headless virtual desktop |
| **RK3588 (`rk3588-hdmirx`)** | HDMI-in via RKMPP hardware encoding | View-only | Tested on OrangePi 5 Plus Armbian vendor kernel |


## 🛠️ Prerequisites

### Client Side
You need a modern web browser that supports WebRTC (H.264 High Profile, or H.265 Main Profile).

### Server / Device Side
For Android devices, please refer to the [scrcpy prerequisites](https://github.com/Genymobile/scrcpy/blob/master/README.md#prerequisites). 

For Windows, Linux, MacOS, and Termux, you need `adb` (or `android-tools` in some distributions, which provides `adb`) in your PATH at least to stream Android devices. If the program didn't found `adb`, it will try to download it. Optionally, if you want to use Linux headless virtual desktop, make sure `ffmpeg` `xvfb`, `xfce4`, `sway`, and `wf-recorder` are in your `PATH`:
```bash
sudo apt install adb

# If you want to stream the Linux display:
sudo apt install xvfb ffmpeg xfce4 sway wf-recorder seatd
```

> **Note**: If you want to use Xorg, you might need to run this program as root.

> **Note**: If you want to use Sway, `wf-recorder` is used.

> **Note**: All of them need `uinput` and `seatd` and correct permissions if you want to have keyboard and mouse passthrough.



#### rk3588 HDMI-in Requirements

* A vendor kernel with HDMI RX support (`rk3588-hdmirx` enabled).
* An FFmpeg build with RKMPP support (e.g., `/usr/lib/jellyfin-ffmpeg/ffmpeg`).
* Correct read/write permissions for MPP service: `sudo chmod 666 /dev/mpp_service`.
* System package `v4l-utils`.

---

## 🚀 Usage

### Option 1: Use Pre-built Binary

Download the latest binary from the [Releases](https://github.com/huonwe/webscreen/releases) page.

Then,
```bash
mkdir ./data
curl -fsSL https://raw.githubusercontent.com/huonwe/webscreen/main/config.yml -o ./data/config.yml
# You might need to change some settings here.
```

```bash
./webscreen -D ./data
```

Then open your browser and visit: `http://<your-ip>:8081`

### Option 2: Build from Source

Normally, you can build simply by running `make`.

> **Termux Note**: If building natively on Termux, you must run `go build -ldflags "-checklinkname=0"`.

```bash
git clone https://github.com/huonwe/webscreen.git
cd webscreen
make
```

### Option 3: Docker

```yaml
services:
  webscreen:
    image: dukihiroi/webscreen:latest
    container_name: webscreen
    network_mode: host
    restart: unless-stopped
    volumes:
      - /dev/bus/usb:/dev/bus/usb
      - ./data:/webscreen/data
    privileged: true

```


> 💡 **Tip:** `host` network mode is highly recommended due to WebRTC UDP traffic and USB device connection stability.

#### Android Pairing

You might need to pair your Android device via [Wireless Debugging](https://developer.android.com/studio/debug/dev-options#enable) first. "Pair device with pairing code" is supported. Once paired, click the **Connect** button in the web UI.
*Note: After starting the stream, if the screen is black, simply trigger a UI change (e.g., press the physical volume button) to force a frame update.*

---

## 📖 Documentation

* [For Developers](doc/dev)

---

## ❓ FAQ

**Q: Crash: `MediaCodec 0x80001001 Exception` on custom Android devices?**

* This is usually due to the H.264 High Profile (`profile=8`).
* **Fix**: Set `profile=1` in `video_codec_options`.

---

## 🤝 Acknowledgements

* Huge thanks to [@Enterpr1se0](https://github.com/Enterpr1se0) and [@rogniceguy-max](https://github.com/rogniceguy-max) for fixing connection bugs!

* Huge thanks to [youyoudezhuzhu/webscreen](https://github.com/youyoudezhuzhu/webscreen) for fixing agent unused sdriver resource release bugs and adding android adb manager!

* [websr](https://github.com/sb2702/websr) for use of super resolution

## 📄 License

This project is licensed under the **GNU Affero General Public License v3.0**.
