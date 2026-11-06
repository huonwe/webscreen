<div align="center">

# 📺 Webscreen

WebRTC をベースにした、Android および Linux デバイス向けのセルフホスト型画面ストリーミング Web アプリケーションです。

[![GitHub release](https://img.shields.io/github/v/release/huonwe/webscreen?style=flat-square)](https://github.com/huonwe/webscreen/releases)
[![GitHub stars](https://img.shields.io/github/stars/huonwe/webscreen?style=flat-square)](https://github.com/huonwe/webscreen/stargazers)
[![Go Version](https://img.shields.io/github/go-mod/go-version/huonwe/webscreen?style=flat-square)](https://github.com/huonwe/webscreen)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg?style=flat-square)](https://www.gnu.org/licenses/agpl-3.0)

[English](README.md) • [简体中文](README.zh.md)

<br/>

**[▶️ デモ動画を見る](https://youtu.be/6WtbwaIk2aY)**

</div>

![スクリーンショット](doc/assets/screenshot.png)

## ✨ 機能

Webscreen は **Android Termux**、**Linux**、**Windows**、**MacOS** で動作します（`amd64` と `arm64` の両方に対応）。将来的にはさらに多くのデバイスに対応する可能性があります！

### 📱 Android（[scrcpy](https://github.com/Genymobile/scrcpy) 経由）
- 映像、音声、操作
- UHID デバイス（マウス、キーボード、ゲームパッド）
- クリップボード同期
- タッチ（マルチタッチ、圧力）
- H.264 / H.265 ハードウェアエンコード
- 複数接続

### 🐧 Linux（Xvfb / Xorg / Sway）
- 映像、操作
- タッチ
- H.264 / H.265
- GPU 対応（Xorg/Sway）

### 🛹 rk3588-hdmirx（OrangePi 5 Plus でテスト済み）
- HDMI 入力映像のストリーミング
- RKMPP ハードウェアエンコードによる H.264 / H.265
- *注：音声にはまだ対応していません。*

> **注**：Android apk（root 化済み）および FnOS については、[youyoudezhuzhu/webscreen](https://github.com/youyoudezhuzhu/webscreen) を参照してください。

---

## 🛠️ 前提条件

### クライアント側
WebRTC（H.264 High Profile または H.265 Main Profile）に対応した最新の Web ブラウザが必要です。

### サーバー / デバイス側
Android デバイスについては、[scrcpy の前提条件](https://github.com/Genymobile/scrcpy/blob/master/README.md#prerequisites)を参照してください。

Linux では、`adb`、`xvfb`、`ffmpeg`、`xfce4`、`sway`、および任意で `wf-recorder` を `PATH` に配置する必要があります。
```bash
apt install adb

# Linux の画面をストリーミングする場合：
apt install xvfb ffmpeg xfce4 sway wf-recorder
```

#### rk3588 HDMI 入力の要件

* HDMI RX に対応したベンダーカーネル（`rk3588-hdmirx` が有効であること）。
* RKMPP に対応した FFmpeg ビルド（例：`/usr/lib/jellyfin-ffmpeg/ffmpeg`）。
* MPP サービスに正しい読み書き権限があること：`sudo chmod 666 /dev/mpp_service`。

---

## 🚀 使い方

### オプション 1：ビルド済みバイナリを使用

[Releases](https://github.com/huonwe/webscreen/releases) ページから最新のバイナリをダウンロードしてください。デフォルトのポートは `8079` です。また、6 桁の PIN が必要です（省略した場合はパスワードなしになります）。

```bash
./webscreen -host 0.0.0.0 -port 8080 -pin 555555
```

ブラウザを開き、`http://<your-ip>:8080` にアクセスしてください。

### オプション 2：ソースからビルド

通常は `make` を実行するだけでビルドできます。

> **Termux について**：Termux 上でネイティブにビルドする場合は、`go build -ldflags "-checklinkname=0"` を実行する必要があります。

```bash
git clone https://github.com/huonwe/webscreen.git
cd webscreen
make
```

### オプション 3：Docker

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

デスクトップ組み込み版では、最初にサンプル環境変数ファイルをコピーしてください：`cp .env.sample .env`

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

> 💡 **ヒント：** WebRTC の UDP トラフィックと USB デバイス接続の安定性のため、`host` ネットワークモードを強く推奨します。

#### Android のペアリング

まず[ワイヤレスデバッグ](https://developer.android.com/studio/debug/dev-options#enable)で Android デバイスをペアリングする必要がある場合があります。「ペアリングコードでデバイスをペア設定」に対応しています。ペアリング後、Web UI の **Connect** ボタンをクリックしてください。
*注：ストリーム開始後に画面が黒い場合は、物理音量ボタンを押すなどして UI を変更すると、フレームが強制的に更新されます。*

---

## 📖 ドキュメント

* [Redroid クイックスタート](https://www.google.com/search?q=doc/quick-start-redroid.md)
* [開発者向け](https://www.google.com/search?q=doc/dev)

---

## ❓ FAQ

**Q：カスタム Android デバイスで `MediaCodec 0x80001001 Exception` が発生してクラッシュします。**

* 通常、H.264 High Profile（`profile=8`）が原因です。
* **対処法**：`video_codec_options` で `profile=1` を設定してください。

## 🤝 謝辞

* コードのバグを修正してくださった [@Enterpr1se0](https://github.com/Enterpr1se0) と [@rogniceguy-max](https://github.com/Enterpr1se0) に心より感謝します！

* 不具合を修正し、新機能を追加してくださった [youyoudezhuzhu/webscreen](https://github.com/youyoudezhuzhu/webscreen) に心より感謝します！

## 📄 ライセンス

このプロジェクトは **GNU Affero General Public License v3.0** の下でライセンスされています。

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