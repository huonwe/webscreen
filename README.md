# Webscreen

## ℹ️ About

Webscreen is a self-hosted screen streaming web application for Android devices, based on WebRTC and [scrcpy](https://github.com/Genymobile/scrcpy).

![screenshot](doc/assets/screenshot.png)

It supports:
- Video, Audio, Control
- UHID Devices (Mouse, Keyboard, Gamepad)
- Clipboard Sync
- Maybe more...

It can run on:
- Android Termux
- Linux
- Windows
- MacOS

at both `amd64` and `arm64`

## Prerequisites
Please refer to [scrcpy](https://github.com/Genymobile/scrcpy/blob/master/README.md#prerequisites)

Additionally, a [modified scrcpy-server](https://github.com/huonwe/scrcpy-0x63) is used.

## Usage
Download the latest [release](https://github.com/huonwe/webscreen/releases), then open your favorite browser and visit `<your ip>:8079`

Or you can build by yourself. Normally, you can build simply by `go build`. But if you want to build by yourself on `Termux`, you need to run `go build -ldflags "-checklinkname=0"`.

You can also use docker:
```bash
docker run -d \
  --name webscreen \
  -p 8079:8079 \
  dukihiroi/webscreen:latest
```


You might need to pair Android device first. `Pair device with pairing code` is supported. Once you finished pairing, type `Connect` button and enter necessary information.

Please notice that the ports in `pair` and `connect` are different.
![sample](doc/assets/wireless_debugging.png)

