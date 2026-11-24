<div align="center">

# 📺 Webscreen

A self-hosted screen streaming web application for Android and Linux devices, based on WebRTC. 

[![GitHub release](https://img.shields.io/github/v/release/huonwe/webscreen?style=flat-square)](https://github.com/huonwe/webscreen/releases)
[![GitHub stars](https://img.shields.io/github/stars/huonwe/webscreen?style=flat-square)](https://github.com/huonwe/webscreen/stargazers)
[![Go Version](https://img.shields.io/github/go-mod/go-version/huonwe/webscreen?style=flat-square)](https://github.com/huonwe/webscreen)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg?style=flat-square)](https://www.gnu.org/licenses/agpl-3.0)

[简体中文](README.zh.md) • [日本語](README.jp.md)

<br/>

**[▶️ Watch Demo Video (Android)](https://youtu.be/6WtbwaIk2aY)**
**[▶️ Watch Demo Video (Orangepi5Plus HDMI-in)](https://youtu.be/XMpHG3clOMU)**


</div>

![screenshot](doc/assets/screenshot.png)

## ✨ Features

Webscreen can run on **Android Termux**, **Linux**, **Windows**, and **MacOS** (both `amd64` and `arm64`). It might support more devices in the future!

### 📱 Android (via [scrcpy](https://github.com/Genymobile/scrcpy))
- Video, Audio, Control
- UHID Devices (Mouse, Keyboard, Gamepad)
- Clipboard Sync (HTTPS required)
- Touch (Multi-finger, pressure)
- H.264 / H.265 Hardware Encoding
- Multi-Connection

> **Note**: For Android apk (rooted) and FnOS fpk, you might see [youyoudezhuzhu/webscreen](https://github.com/youyoudezhuzhu/webscreen).


### 🐧 Linux (Xvfb / Xorg / Sway)
- Video, Control
- Touch
- H.264 / H.265
- GPU support (Xorg/Sway)

### 🛹 rk3588-hdmirx (OrangePi 5 Plus Armbian vendor kernel tested)
- HDMI-in video streaming
- H.264 / H.265 with RKMPP hardware encoding
- *Note: Audio is not supported yet.*

### Universal
- Super Resolution (HTTPS required)

---

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

Download the latest binary from the [Releases](https://github.com/huonwe/webscreen/releases) page. The default port is `8079`. A 6-digit PIN is also required (defaults to no password if omitted).

```bash
./webscreen -host 0.0.0.0 -port 8080 -pin 555555
```

Then open your browser and visit: `http://<your-ip>:8080`

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
    privileged: true
    environment:
      - GIN_MODE=release
      - PORT=8081
      - PIN=123456

```

For the desktop embedded version,copy the sample env file first: `cp .env.sample .env`

```yaml
services:
  webscreen-full:
    user: appuser
    image: dukihiroi/webscreen-full:latest
    container_name: webscreen-full
    network_mode: host
    restart: unless-stopped
    volumes:
      - /dev/bus/usb:/dev/bus/usb
      - /dev/input:/dev/input
      - /run/udev:/run/udev:ro
    devices:
      - /dev/uinput:/dev/uinput
      - /dev/dri:/dev/dri
    group_add:
      - ${UINPUT_GID}
    privileged: true
    environment:
      - GIN_MODE=release
      - PORT=8081
      - PIN=123456

```

> 💡 **Tip:** `host` network mode is highly recommended due to WebRTC UDP traffic and USB device connection stability.

#### Android Pairing

You might need to pair your Android device via [Wireless Debugging](https://developer.android.com/studio/debug/dev-options#enable) first. "Pair device with pairing code" is supported. Once paired, click the **Connect** button in the web UI.
*Note: After starting the stream, if the screen is black, simply trigger a UI change (e.g., press the physical volume button) to force a frame update.*

---

## 📖 Documentation

* [Quick Start with Redroid](https://www.google.com/search?q=doc/quick-start-redroid.md)
* [For Developers](https://www.google.com/search?q=doc/dev)

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