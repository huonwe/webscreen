# Webscreen

## ℹ️ 概要

[デモを見る](https://youtu.be/6WtbwaIk2aY)

Webscreen は、WebRTC をベースにした Android および Linux デバイス向けのセルフホスト型画面ストリーミングウェブアプリケーションです。
![screenshot](doc/assets/screenshot.png)

動作環境：

- Android Termux
- Linux
- Windows
- MacOS

（`amd64` および `arm64` をサポート）

Android の機能 ([scrcpy](https://github.com/Genymobile/scrcpy)):

- ビデオ、オーディオ、制御
- UHID デバイス（マウス、キーボード、ゲームパッド）
- クリップボード同期
- マルチタッチ、筆圧
- H.264/H.265
- マルチ接続
- その他...

Linux の機能 (xvfb):

- ビデオ、制御

## 前提条件

デバイス側については、[scrcpy](https://github.com/Genymobile/scrcpy/blob/master/README.md#prerequisites) を参照してください。

サーバー側については、事前に PATH に `adb` と `xvfb, ffmpeg, xfce4 (この機能が必要な場合、オプション)` をインストールしておくことをお勧めします。

```bash
# Termux の場合
pkg install android-tools
# リポジトリをクローンしてビルド
git clone https://github.com/huonwe/webscreen.git
cd webscreen
go build -o sdriver/xvfb/bin/capturer_xvfb ./capturer
go build -ldflags "-checklinkname=0"

# Debian の場合
apt install adb
# xvfb ディスプレイをストリーミングしたい場合
apt install xvfb ffmpeg xfce4
# その場合、ビルド済みのバイナリを直接使用できます
```

**クライアント側には、WebRTC (H.264 High Profile, または H.265 Main Profile) をサポートするウェブブラウザが必要です。**

## 使用方法

最新の [リリース](https://github.com/huonwe/webscreen/releases) をダウンロードし、プログラムを実行してください。デフォルトのポートは `8079` ですが、 `-port 8080` で指定できます。6桁の PIN コードも必要です（デフォルトは '123456'）。コマンド例：`./webscreen -port 8080 -pin 555555`
その後、お好みのブラウザを開き、`<あなたの IP>:<あなたのポート>` にアクセスしてください。

または、自分でビルドすることもできます。通常は `go build` を実行するだけでビルドできます。ただし、`Termux` 上でビルドする場合は、 `go build -ldflags "-checklinkname=0"` を実行する必要があります。

Docker も使用できます：

```bash
wget https://raw.githubusercontent.com/huonwe/webscreen/refs/heads/main/docker-compose.yml

docker compose up -d
```

UDP トラフィックとデバイス接続のため、`host` ネットワークモードの使用が推奨されます。

Android デバイスを [ワイヤレスデバッグ](https://developer.android.com/studio/debug/dev-options#enable) でペアリングする必要がある場合があります。`ペアリングコードによるデバイスのペアリング` がサポートされています。ペアリングが完了したら、`Connect` ボタンをクリックし、必要な情報を入力してください。

ストリーミング開始後、画面を取得するために画面上のシーンを手動で少し変更する必要がある場合があります。音量ボタンをクリックするだけで十分です。

### その他

[Redroid クイックスタート](https://github.com/huonwe/webscreen/blob/main/doc/quick-start-redroid.md)

## 既知の問題

- Docker および Termux では Xvfb は動作しません

## ライセンス

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
