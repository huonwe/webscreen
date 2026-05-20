# Change: device-transport-refactor

## 状态

draft

## 用户确认的方向

- 目标是让 webscreen 在不破坏现有 USB / 远程 ADB 桥 / `webscreen-root-agent` 路径前提下，支持同时管理多台桥、多台设备。
- 改造拆为 Phase 0（接口抽象 + 并发清算 + 横切安全）和 Phase 1（多桥注册表）。Phase 2 完全沿用 `webscreen-root-agent` OpenSpec，不在本变更内引入新协议。
- root-agent transport **不实现 ADBTransport**。它没有任意 shell / 任意 push / pair / connect，把它塞进 ADBTransport 会破坏安全边界。三分接口为：`DeviceProvider`、`ADBTransport`、`ScrcpyTransport`。
- Phase 1 的 bridge token **不保护裸 socat**。socat 兼容期必须显式标记 `insecure`。安全责任在**桥机一侧**：桥机 socat `bind=Tailscale IP` + 桥机宿主防火墙 + Tailscale ACL。webscreen 自己的 HTTP listener bind 和桥安全无关，不能用 webscreen 端 bind 来"保护" socat。真正 HMAC 必须等桥侧 shim/agent 验证后才成立。
- bridge 健康检查不用 `OpenLocalAbstract` —— scrcpy-server 未启动时它无意义。健康探针必须包含 TCP connect、`adb -H -P devices -l`（ADB host 协议），并支持可选 per-device `shell echo alive`。
- `DeviceRef` 必须**版本化**编码：格式 `v1.<base64url(json)>`，decode 失败时按 legacy 本地 adb serial 迁移。display name / model / status 和 stable key 必须分开，避免 UI 名称变化影响配置绑定。
- 旧 `WEBSCREEN_REMOTE_ADB_*` 环境变量保留为 bootstrap 语法糖（启动时自动注册一条 bridge），不再作为运行时单点。
- 本变更不动 WebRTC、不动编码器选择、不重写 streamAgent 主链路、不下沉媒体到桥。

## 背景

当前 ADB 调用面散落在 12 处，跨 4 个文件，且存在 2 种调用风格（一种走 `utils.GetADBPath()`，一种硬编码字符串 `"adb"`）：

| 文件 | 行 | 调用 | 用途 |
|---|---|---|---|
| `webservice/android/connect.go` | 11 | `ExecADB(args)` | 通用 adb 入口（无 ctx） |
| `webservice/android/connect.go` | 23 | `GetDevices` → `adb devices` | 设备列表 |
| `webservice/android/connect.go` | 65 | `ConnectDevice` → `adb connect ADDR` | tcpip 连接 |
| `webservice/android/connect.go` | 82 | `PairDevice` → `adb pair ADDR CODE` | 无线调试配对 |
| `sdriver/scrcpy/adbutils.go` | 15 | `ExecADB(ctx, args)` | scrcpy 内部 adb（带 ctx）— 与上面重复定义 |
| `sdriver/scrcpy/adbutils.go` | 75 | `GetConnectedDevices` → `adb devices` | scrcpy 设备探测 |
| `sdriver/scrcpy/adbutils.go` | 101 | `ConnectDevice` | 与 `webservice/android` 重复 |
| `sdriver/scrcpy/adbutils.go` | 118 | `PairDevice` | 与 `webservice/android` 重复 |
| `sdriver/scrcpy/adb.go` | 122 | `exec.CommandContext(ctx, "adb", ...)` | encoder 探测，**硬编码字符串，绕过 GetADBPath** |
| `sdriver/scrcpy/adb.go` | 159 | `exec.CommandContext(ctx, "adb", ...)` | opus 探测，**同上** |

结果：

1. webscreen 进程级只能绑定一台桥（`remote_adb.go:14-36` 读取 `WEBSCREEN_REMOTE_ADB_*` 单例 env）。
2. encoder 探测的 `"adb"` 硬编码绕过 `GetADBPath`，远程 adb 切桥时**列表来自桥机、encoder 探测打到本地 adb**，路由错位（你指出的第 2 点）。
3. scrcpy driver 三处写死共享资源：
   - `driver.go:26` `SCRCPY_SERVER_LOCAL_PATH = "./scrcpy-server"` —— 多 session 并发时 `os.WriteFile`（`driver.go:100`）和 `os.Remove`（`driver.go:132`）会互相覆盖/删除。
   - `driver.go:28` `SCRCPY_PROXY_PORT_DEFAULT = "27183"` —— 多 session reverse 模式抢同一 TCP 端口。
   - `driver.go:86` `scid: "00000000"` —— 抽象 socket 名 `scrcpy_00000000` 多 session 撞名。`adbutils.go:27 GenerateSCID()` 已经实现但**无人调用**（dead code）。
4. `utils.GetADBPath()`（`utils/adb.go:18-44`）找不到 adb 时会下载到 `cwd`，是进程级副作用，进一步把 webscreen 跟单一 adb 绑定。
5. WebSocket 接受任意 Origin（`webservice/handleScreen.go:15-19` `CheckOrigin: return true`）。
6. WebRTC subscriber key 是字符串拼接 `DeviceType_DeviceID_DeviceIP_DevicePort`（`handleScreen.go:51`），同时 14 处前后端代码依赖 `device_id` 字段作为主键。
7. 当前没有桥健康检查；远程 adb-server 在 Tailnet 上裸暴露完整 shell 权限。

## 目标

### Phase 0 — 抽象与清算（无新功能）

- **接口三分**：在 `sdriver/scrcpy/transport.go` 新增三个接口，互不继承：
  - `DeviceProvider`：`Devices(ctx) ([]DeviceDescriptor, error)`。所有列表来源（local adb / remote adb / root-agent / future）都实现这个。
  - `ADBTransport`：扩展 `DeviceProvider`，提供 `Shell` / `Push` / `Reverse` / `ReverseRemove` / `OpenLocalAbstract` / `Connect` / `Pair`。**只有** local/remote adb 实现。
  - `ScrcpyTransport`：`StartScrcpySession(ctx, opts) (*ScrcpySession, error)`，返回的 session 内含 `Video / Audio / Control net.Conn`。Local/remote adb 通过共享 helper 把 `ADBTransport` 包成 `ScrcpyTransport`；root-agent 直接实现 `ScrcpyTransport`。
- **DeviceRef v1**：所有跨层主键改成 `DeviceRef{Transport, BridgeID, Serial, AgentID}`，对外编码 `v1.<base64url(json)>`。Display name / model / status 单独字段。Legacy `device_id`（裸 adb serial）在 decode 失败时按 `v1` 补齐 `{Transport:"local_adb", Serial:legacy}` 迁移，前端启动时一次性 upgrade localStorage。
- **scrcpy driver 并发安全**：
  - `scid` 改为 `GenerateSCID()` 输出，每个 driver 实例独立。
  - scrcpy-server 落盘改为 `os.MkdirTemp("", "scrcpy-srv-*")` per-driver，`Close()` 清理。
  - reverse 模式（仅 local adb 走）通过 `net.Listen("tcp", "127.0.0.1:0")` 拿动态端口；remote-adb / root-agent 强制 direct-socket。
- **adb 调用收口**：删除 `sdriver/scrcpy/adb.go:122/159` 的硬编码 `"adb"`；删除 `sdriver/scrcpy/adbutils.go` 中与 `webservice/android` 重复的 `ConnectDevice/PairDevice/GetConnectedDevices`，统一走 transport。
- **横切安全**：
  - `handleScreen.go:15` `CheckOrigin` 从 `return true` 改为读取配置白名单，默认拒绝跨 origin。
  - transport 每次调用强制 per-call timeout，默认 5s，超时计入 bridge 状态。
  - 每次 adb 调用写一行 audit 日志：`time, bridge_id, serial, cmd_summary, duration, result`。token / pin 等敏感数据不进日志。

### Phase 1 — 多桥注册表

- `config/bridges.json` 持久化：`[{id, name, transport, host, port, security_mode, token?, enabled, last_status}]`。
- `security_mode` 取值：
  - `insecure_socat` —— 兼容旧桥；webscreen 启动时打印警告，UI 标红。
  - `shim_hmac` —— 桥侧跑我们的 shim 校验 HMAC（Phase 1 之后）。
  - `none` —— local adb，不适用 token。
- 新增 API：`GET /api/bridge/list`、`POST /api/bridge/add`、`POST /api/bridge/{id}/test`、`DELETE /api/bridge/{id}`。
- 桥健康检查（必须三段式）：
  1. TCP connect to `host:port`，3s 超时。
  2. `adb -H host -P port devices -l`，解析输出。
  3. 可选 per-device `adb -H host -P port -s SERIAL shell echo alive`，仅 UI 主动点 "deep test" 时执行。
- `GET /api/device/list` 并发遍历所有 enabled bridge（fan-out + per-bridge 3s 超时），返回 `DeviceRef.Encode()` 列表。
- 前端：桥管理页面、设备按桥分组、insecure 桥红标、deep test 按钮。

## 非目标

- **不实现 root-agent transport**。该 transport 在 `webscreen-root-agent` Phase 2/4 完成后，注册成虚拟 bridge（`bridge_id=agent-<agent_id>`，`security_mode=shim_hmac`），仅挂 `DeviceProvider + ScrcpyTransport`，不挂 `ADBTransport`。本变更只确保接口拆分能容纳它。
- 不实现 Phase 3 桥侧 agent；socat 兼容期靠 Tailscale ACL，不引入新协议。
- 不下沉 WebRTC 到桥/agent；本变更不动 `webRTCManager.go` 主链路（仅替换 subscriber key 类型）。
- 不动 `streamAgent`、`linuxRecorder`、编码器选择。
- 不重写 `scrcpy-server`。
- 不引入多租户、多用户 ACL；PIN 仍为全局单一鉴权。
- 不做 Linux driver 的 transport 抽象（独立改造，不在本变更范围）。

## 安全边界

- WebSocket `CheckOrigin` 默认 same-origin；额外允许列表必须显式配置。
- `insecure_socat` bridge 的**安全责任在桥机一侧，不在 webscreen**。webscreen 只对桥做出方向 dial，不为桥监听任何端口；webscreen 的 HTTP listener bind（`-host 0.0.0.0` vs Tailscale IP）和桥安全完全无关。真正要限制的对象：
  - 桥机 socat 必须 `bind=100.x.y.z`（Tailscale 接口 IP），禁止 `bind=0.0.0.0`。
  - 桥机宿主防火墙仅允许 Tailscale 网卡入站到 socat 端口。
  - Tailscale ACL 限制只有 webscreen 主机的 tailnet identity 能访问该端口。
  - webscreen 在桥被标 `insecure_socat` 时强提示 UI 红标 + 启动日志 warning，但提示文案必须说 "webscreen 端无法替代桥侧 ACL"。
- bridge token 仅在 `shim_hmac` 模式下传输；明文 token 不进 audit 日志、不进 process listing、不进 `config/bridges.json` 明文存储（用 `crypto/rand` 生成 + DPAPI/keychain/file-mode 0600 选其一）。
- transport 调用 audit 日志默认开启，落 `webscreen-audit.log`；条目格式固定、不含敏感字段。
- DeviceRef 编码层不接受不在 enum 内的 `Transport`；decode 失败回退为 legacy 时打 warn 日志，不静默通过。
- scrcpy-server 临时目录 `os.MkdirTemp` 默认 0700；`Close()` 必须删除临时目录。
- `utils.GetADBPath()` 自动下载行为在本变更内**收敛**：仅 `LocalADBTransport` 显式调用，且写入路径改为 `os.UserCacheDir()/webscreen/adb/`，不再写 cwd。

## 验收

- `grep -rn "exec.Command.*adb\"" sdriver/ webservice/ utils/` 在业务代码内为空（utils 内允许 `LocalADBTransport` 私有实现）。
- 单 webscreen 进程同时跑两条 scrcpy session（同桥不同设备 / 不同桥），无文件 / 端口 / scid 冲突，关闭后无残留临时目录。
- `WEBSCREEN_REMOTE_ADB_*` 拆为 bootstrap 语法糖，仅在启动时调用 `bridge_registry.Register(...)`；运行时无任何代码再读这 5 个 env。
- 两台 bridge 同时注册（一台 mac socat，一台 windows local adb），`GET /api/device/list` 返回两台设备并各自标 bridge_id。
- `POST /api/bridge/{id}/test` 对一个 unauthorized 设备返回 `device_count=1, devices=[{serial, status:"unauthorized"}]`，对一个 fail bridge 返回 `tcp_ok=false`，对一个空桥返回 `tcp_ok=true, adb_ok=true, device_count=0`。
- 前端旧 `device_id` localStorage 在首次启动后被 upgrade 成 `DeviceRef v1`，且老页面书签链接（含旧 device_id 参数）仍可解析。
- DeviceRef decode 失败的请求返回 `400 invalid_device_ref`，不会静默落到默认设备。
- WebSocket 跨 origin 请求被拒绝并返回 `403 origin_not_allowed`。
- audit 日志包含 bridge_id 切换、health probe、连接事件；不含 token / PIN。
- 全部 Phase 1 OpenSpec fixture（见 `fixtures/`）有对应 Go 单元测试通过。
- 旧 ADB 路径（USB 直连 / 现有单桥 socat）regression 通过。
