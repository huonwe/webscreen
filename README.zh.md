# Webscreen

## ℹ️ 关于

[观看演示](https://youtu.be/6WtbwaIk2aY)

Webscreen 是一个基于 WebRTC 的自托管屏幕流式传输 Web 应用程序，适用于 Android 和 Linux 设备。
![screenshot](doc/assets/screenshot.png)

它可以在以下平台上运行：

- Android Termux
- Linux
- Windows
- MacOS

支持 `amd64` 和 `arm64` 架构。

Android 支持 ([scrcpy](https://github.com/Genymobile/scrcpy))：

- 视频、音频、控制
- UHID 设备（鼠标、键盘、手柄）
- 剪贴板同步
- 多指触控、压力感应
- H.264/H.265
- 多连接
- 可能更多...

Linux 支持 (xvfb)：

- 视频、控制

## 前提条件

对于设备端，请参考 [scrcpy](https://github.com/Genymobile/scrcpy/blob/master/README.md#prerequisites)

对于服务端，最好先确保 PATH 中包含 `adb` 以及 `xvfb, ffmpeg, xfce4 (如果需要此功能，可选)`。

```bash
# Termux
pkg install android-tools
# 克隆仓库并构建
git clone https://github.com/huonwe/webscreen.git
cd webscreen
go build -o sdriver/xvfb/bin/capturer_xvfb ./capturer
go build -ldflags "-checklinkname=0"

# Debian
apt install adb
# 如果你想流式传输 xvfb 显示
apt install xvfb ffmpeg xfce4
# 然后你可以直接使用预构建的二进制文件
```

**对于客户端，你需要一个支持 WebRTC (H.264 High Profile, 或 H.265 Main Profile) 的 Web 浏览器。**

## 使用方法

下载最新的 [发布版本](https://github.com/huonwe/webscreen/releases)，执行程序。默认端口是 `8079`，但你可以通过 `-port 8080` 指定。还需要 6 位 PIN 码（默认为 '123456'）。命令示例：`./webscreen -port 8080 -pin 555555`
然后打开你喜欢的浏览器并访问 `<你的 ip>:<你的端口>`

或者你可以自己构建。通常，你只需运行 `go build` 即可构建。但如果你想在 `Termux` 上自己构建，你需要运行 `go build -ldflags "-checklinkname=0"`。

你也可以使用 docker：

```bash
wget https://raw.githubusercontent.com/huonwe/webscreen/refs/heads/main/docker-compose.yml

docker compose up -d
```

由于 UDP 流量和设备连接的原因，推荐使用 `host` 网络模式。

你可能需要先在 [无线调试](https://developer.android.com/studio/debug/dev-options#enable) 中配对 Android 设备。支持 `使用配对码配对设备`。配对完成后，点击 `Connect` 按钮并输入必要信息。

开始流式传输后，你可能需要手动稍微改变一下屏幕场景，以获取屏幕画面。你可以简单地点击音量按钮来实现。

### 其他

[Redroid 快速入门](https://github.com/huonwe/webscreen/blob/main/doc/quick-start-redroid.md)

## 已知问题

- Xvfb 在 docker 和 termux 中无法工作

## 许可证

```LICENSE
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
