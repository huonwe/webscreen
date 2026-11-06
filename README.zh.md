<div align="center">

# 📺 Webscreen

一个基于 WebRTC、面向 Android 和 Linux 设备的自托管屏幕串流 Web 应用。

[![GitHub release](https://img.shields.io/github/v/release/huonwe/webscreen?style=flat-square)](https://github.com/huonwe/webscreen/releases)
[![GitHub stars](https://img.shields.io/github/stars/huonwe/webscreen?style=flat-square)](https://github.com/huonwe/webscreen/stargazers)
[![Go Version](https://img.shields.io/github/go-mod/go-version/huonwe/webscreen?style=flat-square)](https://github.com/huonwe/webscreen)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg?style=flat-square)](https://www.gnu.org/licenses/agpl-3.0)

[English](README.md) • [日本語](README.jp.md)

<br/>

**[▶️ 观看演示视频](https://youtu.be/6WtbwaIk2aY)**

</div>

![截图](doc/assets/screenshot.png)

## ✨ 功能

Webscreen 可以运行在 **Android Termux**、**Linux**、**Windows** 和 **MacOS** 上（支持 `amd64` 和 `arm64`）。未来可能会支持更多设备！

### 📱 Android（通过 [scrcpy](https://github.com/Genymobile/scrcpy)）
- 视频、音频、控制
- UHID 设备（鼠标、键盘、游戏手柄）
- 剪贴板同步
- 触控（多指、压力感应）
- H.264 / H.265 硬件编码
- 多连接

### 🐧 Linux（Xvfb / Xorg / Sway）
- 视频、控制
- 触控
- H.264 / H.265
- GPU 支持（Xorg/Sway）

### 🛹 rk3588-hdmirx（已在 OrangePi 5 Plus 上测试）
- HDMI 输入视频串流
- 使用 RKMPP 硬件编码的 H.264 / H.265
- *注意：暂不支持音频。*

> **注意**：对于 Android apk（已 root）和 FnOS，你可以查看 [youyoudezhuzhu/webscreen](https://github.com/youyoudezhuzhu/webscreen)。

---

## 🛠️ 前置要求

### 客户端
你需要使用支持 WebRTC 的现代 Web 浏览器（H.264 High Profile 或 H.265 Main Profile）。

### 服务端 / 设备端
对于 Android 设备，请参考 [scrcpy 的前置要求](https://github.com/Genymobile/scrcpy/blob/master/README.md#prerequisites)。

对于 Linux，你需要将 `adb`、`xvfb`、`ffmpeg`、`xfce4`、`sway` 以及可选的 `wf-recorder` 放入 `PATH`：
```bash
apt install adb

# 如果你希望串流 Linux 桌面：
apt install xvfb ffmpeg xfce4 sway wf-recorder
```

#### rk3588 HDMI 输入要求

* 支持 HDMI RX 的厂商内核（已启用 `rk3588-hdmirx`）。
* 支持 RKMPP 的 FFmpeg 构建版本（例如 `/usr/lib/jellyfin-ffmpeg/ffmpeg`）。
* MPP 服务具有正确的读写权限：`sudo chmod 666 /dev/mpp_service`。

---

## 🚀 使用方法

### 选项 1：使用预编译二进制文件

从 [Releases](https://github.com/huonwe/webscreen/releases) 页面下载最新的二进制文件。默认端口为 `8079`，同时需要一个 6 位 PIN（省略时默认为无密码）。

```bash
./webscreen -host 0.0.0.0 -port 8080 -pin 555555
```

然后打开浏览器访问：`http://<your-ip>:8080`

### 选项 2：从源代码构建

通常直接运行 `make` 即可完成构建。

> **Termux 注意**：如果在 Termux 上原生构建，必须运行 `go build -ldflags "-checklinkname=0"`。

```bash
git clone https://github.com/huonwe/webscreen.git
cd webscreen
make
```

### 选项 3：Docker

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

对于桌面嵌入版，请先复制示例环境变量文件：`cp .env.sample .env`

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

> 💡 **提示：**由于 WebRTC UDP 流量和 USB 设备连接稳定性的原因，强烈建议使用 `host` 网络模式。

#### Android 配对

你可能需要先通过[无线调试](https://developer.android.com/studio/debug/dev-options#enable)配对 Android 设备。支持“使用配对码配对设备”。配对完成后，点击 Web UI 中的 **Connect** 按钮。
*注意：启动串流后，如果屏幕是黑的，只需触发一次 UI 变化（例如按下实体音量键）即可强制更新画面。*

---

## 📖 文档

* [使用 Redroid 快速开始](https://www.google.com/search?q=doc/quick-start-redroid.md)
* [开发者文档](https://www.google.com/search?q=doc/dev)

---

## ❓ 常见问题

**问：在定制 Android 设备上崩溃：`MediaCodec 0x80001001 Exception`？**

* 这通常是由 H.264 High Profile（`profile=8`）导致的。
* **解决方法**：在 `video_codec_options` 中设置 `profile=1`。

## 🤝 致谢

* 非常感谢 [@Enterpr1se0](https://github.com/Enterpr1se0) 和 [@rogniceguy-max](https://github.com/Enterpr1se0) 修复代码错误！

* 非常感谢 [youyoudezhuzhu/webscreen](https://github.com/youyoudezhuzhu/webscreen) 修复连接问题并添加新功能！

## 📄 许可证

本项目采用 **GNU Affero General Public License v3.0** 授权。

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