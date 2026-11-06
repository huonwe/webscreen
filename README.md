<div align="center">

# 📺 Webscreen

A self-hosted screen streaming web application for Android and Linux devices, based on WebRTC. 

[![GitHub release](https://img.shields.io/github/v/release/huonwe/webscreen?style=flat-square)](https://github.com/huonwe/webscreen/releases)
[![GitHub stars](https://img.shields.io/github/stars/huonwe/webscreen?style=flat-square)](https://github.com/huonwe/webscreen/stargazers)
[![Go Version](https://img.shields.io/github/go-mod/go-version/huonwe/webscreen?style=flat-square)](https://github.com/huonwe/webscreen)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg?style=flat-square)](https://www.gnu.org/licenses/agpl-3.0)

[简体中文](README.zh.md) • [日本語](README.jp.md)

<br/>

**[▶️ Watch Demo Video](https://youtu.be/6WtbwaIk2aY)**

</div>

![screenshot](doc/assets/screenshot.png)

## ✨ Features

Webscreen can run on **Android Termux**, **Linux**, **Windows**, and **MacOS** (both `amd64` and `arm64`). It might support more devices in the future!

### 📱 Android (via [scrcpy](https://github.com/Genymobile/scrcpy))
- Video, Audio, Control
- UHID Devices (Mouse, Keyboard, Gamepad)
- Clipboard Sync
- Touch (Multi-finger, pressure)
- H.264 / H.265 Hardware Encoding
- Multi-Connection

### 🐧 Linux (Xvfb / Xorg / Sway)
- Video, Control
- Touch
- H.264 / H.265
- GPU support (Xorg/Sway)

### 🛹 rk3588-hdmirx (OrangePi 5 Plus tested)
- HDMI-in video streaming
- H.264 / H.265 with RKMPP hardware encoding
- *Note: Audio is not supported yet.*


> **Note**: For Android apk (rooted) and FnOS fpk, you might see [youyoudezhuzhu/webscreen](https://github.com/youyoudezhuzhu/webscreen).

---

## 🛠️ Prerequisites

### Client Side
You need a modern web browser that supports WebRTC (H.264 High Profile, or H.265 Main Profile).

### Server / Device Side
For Android devices, please refer to the [scrcpy prerequisites](https://github.com/Genymobile/scrcpy/blob/master/README.md#prerequisites). 

For Linux, you need `adb`, `xvfb`, `ffmpeg`, `xfce4`, `sway`, and optionally `wf-recorder` in your `PATH`:
```bash
apt install adb

# If you want to stream the Linux display:
apt install xvfb ffmpeg xfce4 sway wf-recorder
```

#### rk3588 HDMI-in Requirements

* A vendor kernel with HDMI RX support (`rk3588-hdmirx` enabled).
* An FFmpeg build with RKMPP support (e.g., `/usr/lib/jellyfin-ffmpeg/ffmpeg`).
* Correct read/write permissions for MPP service: `sudo chmod 666 /dev/mpp_service`.
* System package 'v4l-utils'.

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

* Huge thanks to [@Enterpr1se0](https://github.com/Enterpr1se0) and [@rogniceguy-max](https://github.com/Enterpr1se0) for fixing code bugs!

* Huge thanks to [youyoudezhuzhu/webscreen](https://github.com/youyoudezhuzhu/webscreen) for fixing bugs and adding new features!

## 📄 License

This project is licensed under the **GNU Affero General Public License v3.0**.

```text
Webscreen, streaming your device in Web browser.
Copyright (C) 2026  Hiroi

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
```
