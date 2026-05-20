# Design: device-transport-refactor

## 变更类型

接口与并发清算 + 多桥注册表，跨多个 package，必须分 Phase 落地。

设计范围允许覆盖：

```text
sdriver/
sdriver/scrcpy/
webservice/
webservice/android/
public/static/
utils/
openspec/changes/device-transport-refactor/
```

V1 不改：

```text
streamAgent/
linuxRecorder/
sdriver/linux/
sdriver/sunshine/
sdriver/dummy/
sdriver/comm/
public/screen.html (仅 device_id 读取处会变；HTML 结构不动)
main.go (除新增 bootstrap 调用)
```

后续实现必须按 Phase 0a → 0b → 0c → 0d → 1a → 1b → 1c → 1d 顺序落 PR，每个 PR 单独 review。Phase 边界不得越级合并。

## 设计门禁（来自用户）

1. **RootAgentTransport 不实现 ADBTransport**。接口三分，root-agent 只能拿到 `DeviceProvider + ScrcpyTransport`。
2. **socat 兼容期不靠 token**。`insecure_socat` 必须显式标记。安全责任在**桥机一侧**：桥机 socat `bind=Tailscale IP`、宿主防火墙、Tailscale ACL。webscreen 端只是显示警告，不能替代桥侧 ACL。真 HMAC 等 shim/agent。
3. **健康检查不用 OpenLocalAbstract**。三段式：TCP / `adb -H -P devices -l` / 可选 per-device shell。
4. **DeviceRef 必须版本化**。`v1.<base64url(json)>` 格式，decode 失败时按 legacy 本地 adb serial 迁移；display name 与 stable key 分离。

任何阶段实现违反上述四点必须停下来重新提案，不得绕道。

## 接口设计

### DeviceProvider

```go
// sdriver/scrcpy/transport.go

type DeviceDescriptor struct {
    Ref       DeviceRef       // 主键
    Display   string          // UI 显示名（model 或 user-rename）
    Model     string          // 设备 model（来自 adb -l）
    Status    DeviceStatus    // connected / unauthorized / offline / unknown
    Transport TransportID     // 冗余字段方便前端分组；权威值在 Ref
    Bridge    BridgeID        // 同上
    LastSeen  time.Time
}

type DeviceStatus string

const (
    DeviceConnected    DeviceStatus = "connected"
    DeviceUnauthorized DeviceStatus = "unauthorized"
    DeviceOffline      DeviceStatus = "offline"
    DeviceUnknown      DeviceStatus = "unknown"
)

type DeviceProvider interface {
    Devices(ctx context.Context) ([]DeviceDescriptor, error)
}
```

### ADBTransport

```go
type ADBTransport interface {
    DeviceProvider

    Shell(ctx context.Context, serial string, argv ...string) ([]byte, error)
    Push(ctx context.Context, serial, src, dst string) error
    Reverse(ctx context.Context, serial, remote, local string) error
    ReverseRemove(ctx context.Context, serial, remote string) error
    OpenLocalAbstract(ctx context.Context, serial, socketName string) (net.Conn, error)
    Connect(ctx context.Context, addr string) error // 仅支持 tcpip 模式时
    Pair(ctx context.Context, addr, code string) error

    Capabilities() ADBTransportCaps
    Bridge() BridgeID
}

type ADBTransportCaps struct {
    CanReverse        bool // remote adb 上不可用
    CanTCPIPConnect   bool // local adb / 某些远端可
    CanPair           bool // Android 11+ 无线调试
    SupportsDirectAbs bool // remote adb-server 可，local 需要 forward
}
```

### ScrcpyTransport

```go
type ScrcpyTransport interface {
    DeviceProvider

    StartScrcpySession(ctx context.Context, ref DeviceRef, opts ScrcpyOptions) (*ScrcpySession, error)
}

type ScrcpySession struct {
    SCID              string
    Video             net.Conn // raw bytes, scrcpy 原协议序列
    Audio             net.Conn // opts.Audio=false 时为 nil
    Control           net.Conn // opts.Control=false 时为 nil
    NeedsForwardDummy bool     // 见下表；true 时调用方必须先从首条非空 stream 读 1 字节 dummy
    Close             func() error
}

type ScrcpyOptions struct {
    SCID          string  // 由调用方注入；空时 transport 拒绝并报错（堵死历史 "00000000"）
    Video         bool
    Audio         bool
    Control       bool
    TunnelForward bool    // 决定隧道模式；transport 实现按下表强制
    Codec         string  // "h264" | "h265" | ...
    MaxSize       int
    MaxFPS        int
    BitrateKbps   int
    // 其余与现有 ScrcpyDriver options map 一一对应
}
```

#### Dummy byte 状态对照表

scrcpy 的 forward dummy byte 仅在 **scrcpy-server 主动等待客户端连接** 的模式下出现 —— 这一字节由 scrcpy-server 在 accept 后立刻发出，用于通知 "real client attached"。reverse 模式（device 主动连出到 PC 的 listener）下 PC 侧只 accept，不会收到 dummy。

| Transport / 模式 | 当前代码位置 | TunnelForward | NeedsForwardDummy |
|---|---|---|---|
| LocalADB reverse（device 连出） | `driver.go:352` | `false` | **false** |
| LocalADB direct（PC 通过 `adb forward` 连入 localabstract） | 当前无此路径 | `true` | **true** |
| RemoteADB direct（PC 通过 adb-server `localabstract:` 连入） | `driver.go:458-461` | `true` | **true** |
| RootAgent（agent 主动 dial PC，scrcpy-server 在 device 侧 listen） | `agent_transport.go:159-163` | `true` | **true** |

Phase 0b 提取 `gatherScrcpyConns` helper 时**必须**按 `session.NeedsForwardDummy` 字段决定是否调用 `io.ReadFull(firstConn, dummy[:1])`。错配会直接把 device meta 的第一个字节读掉，造成解析错位。

`ScrcpyTransport` 的两个内置 adapter：

- `adbScrcpyTransport{adb ADBTransport}`：用 `adb.Push` + `adb.Shell` 启动 scrcpy-server；按 `Capabilities().CanReverse` 决定走 reverse 或 direct-abs；返回的 `net.Conn` 来自 `adb.OpenLocalAbstract` 或本地 listener Accept。
- `rootAgentScrcpyTransport`：由 `webscreen-root-agent` 提供，把 control JSON + 三条 raw stream attach 直接映射成 `ScrcpySession`。**不**实现 ADBTransport。

`adbScrcpyTransport` 是包内 unexported helper，只接受 `ADBTransport` 入参。这样根本不会有"root-agent 包装成 adb"这种调用路径出现。

### 为什么三分

| 能力 | local adb | remote adb-server | root-agent |
|---|---|---|---|
| 列设备 | ✓ | ✓ | ✓（agent hello 时上报） |
| 任意 shell | ✓ | ✓ | ✗（设计上拒绝） |
| 任意 push | ✓ | ✓ | ✗（仅 allowlist 文件推送，受协议约束） |
| pair / connect | ✓ | ✓（如果 server 跑得允许） | ✗ |
| 启动 scrcpy + 拿三条 raw stream | ✓ | ✓ | ✓ |

把后两者并到一个接口里，意味着调用方必须运行时 `if transport.HasCapability(...)`，错误路径会扩散；接口三分时调用方在编译期就拿不到非法能力。

## DeviceRef v1

### 字段

```go
type DeviceRef struct {
    Transport TransportID // "local_adb" | "remote_adb" | "root_agent"
    BridgeID  BridgeID    // "" for local_adb; "<id>" otherwise
    Serial    string      // adb serial, 仅 *_adb transport 有意义
    AgentID   string      // root_agent 专用
}

type TransportID string

const (
    TransportLocalADB  TransportID = "local_adb"
    TransportRemoteADB TransportID = "remote_adb"
    TransportRootAgent TransportID = "root_agent"
)
```

### 编码

```text
v1.<base64url-no-pad(json-bytes)>
```

JSON 必须按字段名 ascii 排序：`{"agent_id":"","bridge_id":"...","serial":"...","transport":"..."}`。多余字段 reject。

例：

```text
v1.eyJhZ2VudF9pZCI6IiIsImJyaWRnZV9pZCI6Im1hYy1jb3JhbCIsInNlcmlhbCI6IjVmOWU3OTQ3IiwidHJhbnNwb3J0IjoicmVtb3RlX2FkYiJ9
```

decode 算法：

1. 输入若以 `v1.` 开头 → base64url-no-pad decode → JSON unmarshal → 字段校验。
   - Transport 必须在 enum 内。
   - `local_adb` 时 BridgeID 必须空且 AgentID 必须空。
   - `remote_adb` 时 BridgeID 必须非空且 AgentID 必须空。
   - `root_agent` 时 AgentID 必须非空（BridgeID 可为虚拟 `agent-<AgentID>`）。
2. 输入不以 `v1.` 开头 → 视作 legacy 裸 adb serial → 返回 `DeviceRef{Transport: TransportLocalADB, Serial: input}` 并打 warn 日志。
3. 编码格式不识别（如 `v2.`）→ 返回 `ErrUnknownDeviceRefVersion`，不静默 fallback。

### 显示名与主键分离

| 字段 | 是否参与等价 / 路由 | 是否可改 |
|---|---|---|
| DeviceRef | 是 | 否（改了等于换设备） |
| Display | 否 | 是（用户 rename） |
| Model | 否 | 仅来自 adb，不可手工改 |
| Status | 否 | 探针写入 |

WebRTC subscriber key、`/screen/:id` path 参数、前端 localStorage 全部用 `DeviceRef.Encode()`。

### 老 `device_id` 迁移

- 后端：`handleScreen.go:51` 的 `deviceIdentifier` 改成 `ref.Encode()`。如果接收到的字符串不以 `v1.` 开头，跑 legacy fallback 并 warn。
- 前端：当前真实存储是单对象 `localStorage.webscreen_device_configs = { [serial]: {...} }` + 数组 `localStorage.webscreen_ignored_devices = [serial, ...]`（见 `public/static/console.js:6-7`）。迁移方式不动结构 key：
  - 给 `webscreen_device_configs` 每个 entry 增加 `_ref: "v1.xxx"` 字段，老 serial key 保留 30 天。
  - 并行新建 `webscreen_ignored_device_refs: ["v1.xxx", ...]`，老数组保留 30 天。
  - 写 `webscreen_storage_schema_version = "2"` 标记已迁移；幂等。
- 老链接（`/screen/5f9e7947`）：路由层判定不是 `v1.` 前缀时按 `local_adb` 解析，**返回 200 OK 正常进入 stream**。本变更不做 301 跳转 —— 老书签必须继续可用，301 留到 device_id 字段最终下线那次变更再加。

## scrcpy driver 并发清算

### 现状

```go
// sdriver/scrcpy/driver.go
const (
    SCRCPY_SERVER_LOCAL_PATH  = "./scrcpy-server"   // L26
    SCRCPY_PROXY_PORT_DEFAULT = "27183"             // L28
)

func New(config map[string]string) (*ScrcpyDriver, error) {
    da := &ScrcpyDriver{
        scid: "00000000",                            // L86
    }
    ...
    err = os.WriteFile(SCRCPY_SERVER_LOCAL_PATH, data, 0755)  // L100
    localPort := SCRCPY_PROXY_PORT_DEFAULT                    // L106
    ...
}
```

`adbutils.go:27 GenerateSCID()` 已实现但无人调用。

### 改造

```go
type ScrcpyDriver struct {
    scid     string  // 8 byte 随机 hex
    tmpDir   string  // os.MkdirTemp("", "scrcpy-srv-*")
    proxyLn  net.Listener // reverse 模式才有，端口 = ln.Addr().Port
    ...
}

func New(transport ScrcpyTransport, ref DeviceRef, config map[string]string) (*ScrcpyDriver, error) {
    da := &ScrcpyDriver{
        scid: GenerateSCID(),
    }
    var err error
    da.tmpDir, err = os.MkdirTemp("", "scrcpy-srv-*")
    if err != nil { return nil, err }

    // ...写 scrcpy-server 到 da.tmpDir/scrcpy-server
    // ...由 transport 决定 reverse vs direct

    return da, nil
}

func (d *ScrcpyDriver) Close() error {
    if d.proxyLn != nil { d.proxyLn.Close() }
    if d.tmpDir != "" { os.RemoveAll(d.tmpDir) }
    // ...
}
```

reverse 模式动态端口：

```go
ln, err := net.Listen("tcp", "127.0.0.1:0")
if err != nil { return nil, err }
port := ln.Addr().(*net.TCPAddr).Port
abs := fmt.Sprintf("localabstract:scrcpy_%s", d.scid)
if err := adb.Reverse(ctx, ref.Serial, abs, fmt.Sprintf("tcp:%d", port)); err != nil { ... }
d.proxyLn = ln
```

direct-abs 模式（remote adb 直接拿 conn）：

```go
videoConn, err := adb.OpenLocalAbstract(ctx, ref.Serial, fmt.Sprintf("scrcpy_%s", d.scid))
// audio / control 同上
```

`ScrcpySession.Close` 必须：

1. 关闭三条 raw stream
2. （reverse 模式）`adb.ReverseRemove(..., abs)`
3. 关闭 `proxyLn`
4. `os.RemoveAll(tmpDir)`
5. shell `pkill -f scrcpy-server.*scid=<scid>` —— 只 kill 自己的 scid，不要 `pkill app_process` 误杀别的 session

## 多桥注册表

### Bridge 数据模型

```go
type BridgeID string

type Bridge struct {
    ID           BridgeID      // 用户起名 / 自动生成（不可改）
    Name         string        // UI 显示，可改
    Transport    TransportID   // local_adb / remote_adb
    Host         string        // remote_adb 必填
    Port         int           // remote_adb 必填
    SecurityMode SecurityMode  // insecure_socat / shim_hmac / none
    TokenRef     string        // 指向 secret store 的 key，不存明文
    Enabled      bool
    Notes        string        // 用户备注
    LastStatus   BridgeStatus  // 探针结果
    LastSeen     time.Time
}

type SecurityMode string

const (
    SecurityNone          SecurityMode = "none"           // local adb
    SecurityInsecureSocat SecurityMode = "insecure_socat" // 兼容期
    SecurityShimHMAC      SecurityMode = "shim_hmac"      // 桥侧 shim 验签
)

type BridgeStatus struct {
    TCPOk       bool
    ADBOk       bool
    DeviceCount int
    LatencyMs   int
    LastError   string  // 已 redact
    CheckedAt   time.Time
}
```

### 持久化

`config/bridges.json`：

```json
{
  "version": 1,
  "bridges": [
    {
      "id": "local",
      "name": "本机 ADB",
      "transport": "local_adb",
      "security_mode": "none",
      "enabled": true
    },
    {
      "id": "mac-coral",
      "name": "Mac 桥 (NE2210)",
      "transport": "remote_adb",
      "host": "100.85.28.3",
      "port": 15037,
      "security_mode": "insecure_socat",
      "enabled": true,
      "notes": "socat 中转，无鉴权，依赖 Tailscale ACL"
    }
  ]
}
```

文件权限：0600。token（如未来 shim_hmac 用）单独存入 OS secret store（Windows DPAPI / macOS Keychain / Linux libsecret），失败回退 0600 文件。

### Bootstrap 兼容

`main.go` 启动时：

```go
if v := os.Getenv("WEBSCREEN_REMOTE_ADB"); v == "1" {
    host := envOr("WEBSCREEN_REMOTE_ADB_HOST", "ANDROID_ADB_SERVER_ADDRESS")
    port := envOr("WEBSCREEN_REMOTE_ADB_PORT", "ANDROID_ADB_SERVER_PORT")
    registry.RegisterEphemeral(Bridge{
        ID:           "bootstrap-env",
        Transport:    TransportRemoteADB,
        Host:         host,
        Port:         atoi(port),
        SecurityMode: SecurityInsecureSocat,
        Enabled:      true,
    })
    log.Println("[bridge] WEBSCREEN_REMOTE_ADB env detected, registered ephemeral bridge `bootstrap-env`. Persist via /api/bridge/add for permanent setup.")
}
```

ephemeral bridge 不写盘，进程退出消失。这让 launcher / 现有脚本完全无感。

### API

| Method | Path | Body | 说明 |
|---|---|---|---|
| GET | `/api/bridge/list` | — | 返回所有 bridge + 最近 status |
| POST | `/api/bridge/add` | `{name, transport, host?, port?, security_mode, token?, enabled}` | 持久化新桥；id 后端生成 |
| POST | `/api/bridge/{id}/test` | `{deep: bool}` | 三段式健康检查，`deep=true` 跑 per-device shell echo |
| POST | `/api/bridge/{id}/toggle` | `{enabled: bool}` | 启停 |
| DELETE | `/api/bridge/{id}` | — | 删除桥；如果是 `bootstrap-env` 拒绝（用户应改环境变量） |

所有 API 走现有 PIN 中间件。新增 `CheckOrigin` 白名单检查（管理 API 同样适用）。

### 健康检查实现（按 transport 分派）

不同 transport 的可探针面不一样，必须显式分派；不允许"默认 TCP probe"逻辑套到没有 host:port 的 transport 上。

```go
func (b *Bridge) Probe(ctx context.Context, deep bool) BridgeStatus {
    s := BridgeStatus{CheckedAt: time.Now()}
    switch b.Transport {
    case TransportLocalADB:
        return b.probeLocalADB(ctx, s)
    case TransportRemoteADB:
        return b.probeRemoteADB(ctx, s, deep)
    case TransportRootAgent:
        return b.probeRootAgent(ctx, s)
    default:
        s.LastError = "unknown transport"
        return s
    }
}
```

#### probeLocalADB

local_adb 没有 host:port，TCP 段跳过：

```go
func (b *Bridge) probeLocalADB(ctx context.Context, s BridgeStatus) BridgeStatus {
    s.TCPOk = true // N/A，标记 true 避免 UI 误显示掉线
    devices, err := b.Transport().Devices(ctx)
    if err != nil {
        s.LastError = redact(err.Error())
        return s
    }
    s.ADBOk = true
    s.DeviceCount = len(devices)
    s.LatencyMs = int(time.Since(s.CheckedAt).Milliseconds())
    return s
}
```

#### probeRemoteADB（三段式）

```go
func (b *Bridge) probeRemoteADB(ctx context.Context, s BridgeStatus, deep bool) BridgeStatus {
    // 1. TCP
    dialCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
    defer cancel()
    conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", net.JoinHostPort(b.Host, strconv.Itoa(b.Port)))
    if err != nil {
        s.LastError = redact(err.Error())
        return s
    }
    conn.Close()
    s.TCPOk = true

    // 2. ADB host protocol via CLI: adb -H host -P port devices -l
    devices, err := b.Transport().Devices(ctx)
    if err != nil {
        s.LastError = redact(err.Error())
        return s
    }
    s.ADBOk = true
    s.DeviceCount = len(devices)

    // 3. Deep: per-device shell echo
    if deep {
        adb := b.Transport().(ADBTransport)
        for _, d := range devices {
            if d.Status != DeviceConnected { continue }
            if _, err := adb.Shell(ctx, d.Ref.Serial, "echo", "alive"); err != nil {
                s.LastError = fmt.Sprintf("device %s shell failed: %s", d.Ref.Serial, redact(err.Error()))
                return s
            }
        }
    }

    s.LatencyMs = int(time.Since(s.CheckedAt).Milliseconds())
    return s
}
```

#### probeRootAgent（in-memory only）

root-agent 是被动 listener：agent 主动连接 webscreen，session 状态保存在内存里的 `agentSessionRegistry`。健康检查不做出方向 dial，也不跑 adb：

```go
func (b *Bridge) probeRootAgent(ctx context.Context, s BridgeStatus) BridgeStatus {
    sess, ok := agentSessions.GetByBridgeID(b.ID)
    if !ok {
        s.LastError = "agent session not active"
        return s
    }
    if time.Since(sess.LastPong) > 30*time.Second {
        s.LastError = "heartbeat stale"
        return s
    }
    s.TCPOk = true
    s.ADBOk = true // 语义上"transport 可用"，避免 UI 误判
    s.DeviceCount = 1 // root-agent 一个 session 一台 device
    s.LatencyMs = int(time.Since(sess.LastPong).Milliseconds())
    return s
}
```

`OpenLocalAbstract` **不出现在任何 Probe 路径**。

## 横切安全

### CheckOrigin 白名单

```go
// webservice/handleScreen.go
var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return wm.config.IsAllowedOrigin(r.Header.Get("Origin"), r.Host)
    },
}
```

config：

```yaml
allowed_origins:
  - same-host    # 默认：Origin host == Host header
  - http://localhost:8081
  - http://100.x.y.z:8081
```

### Audit 日志

```go
// sdriver/scrcpy/audit.go
type AuditEntry struct {
    Time     time.Time
    BridgeID BridgeID
    Serial   string
    Action   string   // "shell" | "push" | "reverse" | "open_localabstract" | "connect" | "pair"
    Summary  string   // redacted command summary, no full args
    Duration time.Duration
    Result   string   // "ok" | error class
}
```

token / PIN / 文件内容 / shell stdout 不进 audit。

### transport 调用 timeout

每个 ADBTransport 方法外层 wrapper 强制 `context.WithTimeout`，默认 5s，可在 bridge config 覆盖。超时计入 BridgeStatus 的 `LastError` 并打 metric。

### adb 可执行注入策略（明确二分）

Phase 0a **不**重新实现 ADB sync protocol；CLI 类操作一律走标准 adb CLI 进程。但 `OpenLocalAbstract` 是 scrcpy 媒体流唯一接入点，必须走 raw socket（无法用 CLI 实现）。两边各管一边：

| ADBTransport 方法 | 实现方式 |
|---|---|
| `Devices` / `Shell` / `Push` / `Connect` / `Pair` / `Reverse` / `ReverseRemove` | `exec(ADBExecutable, ...)` —— LocalADBTransport 不加 host/port 参数；RemoteADBTransport 加 `-H host -P port` |
| `OpenLocalAbstract` | raw TCP → adb host protocol（`adbService("host:transport:serial")` + `adbService("localabstract:NAME")`）。沿用现有 `sdriver/scrcpy/remote_adb.go` 已经实现的路径 |

两个 transport 构造时都必须显式注入 `ADBExecutable string` 字段；`utils.GetADBPath()` 自动下载副作用收敛在 `LocalADBTransport.New()` 一处：

- `LocalADBTransport.New()` 调用 `utils.GetADBPath()` 取得路径，注入到 `ADBExecutable`。下载路径从 `cwd` 改为 `os.UserCacheDir()/webscreen/adb/`，文件名带版本，避免覆盖。
- `RemoteADBTransport.New(host, port, adbExecutable)` 由 bridge registry 显式传入路径（默认沿用 `LocalADBTransport` 找到的那份）；**不**触发自动下载。
- 测试时通过依赖注入传 fake `ADBExecutable`（用脚本桩或 fake adb-server 监听 raw socket）。`Devices/Shell/Push/Pair` 等测试用 mock exec 捕获 argv；`OpenLocalAbstract` 测试用 fake adb-server 监听 raw socket，按 adb host protocol 响应。两类测试互不重叠。

## Phase 边界

| Phase | 范围 | 不动 |
|---|---|---|
| 0a | 接口定义（transport.go）+ DeviceRef + adapter 骨架 + LocalADBTransport 实现 + RemoteADBTransport 实现 + 全部 adb 调用收口 | bridge registry / API / 前端 |
| 0b | scrcpy driver 并发清算（scid / tmpDir / 动态端口） | bridge registry / API / 前端 |
| 0c | CheckOrigin / audit log / timeout / GetADBPath 收敛 | bridge registry / API / 前端 |
| 0d | 前端 device_id → DeviceRef 迁移；老链接兼容；localStorage upgrade | bridge registry / API |
| 1a | bridge registry 持久化 + bootstrap 兼容层 | API / 前端 |
| 1b | bridge API（list/add/test/toggle/delete） | 前端 |
| 1c | 健康探针实现 + per-bridge timeout / circuit-breaker | 前端 |
| 1d | 前端 bridge 管理 UI、设备分组、insecure 红标、deep test 按钮 | — |

每 Phase 一个 PR，PR 之间不允许 squash。Phase 0 必须全部落完才能开 Phase 1。

## 与 webscreen-root-agent 的接缝

- 本变更只引入接口，不实现 RootAgentTransport。
- `webscreen-root-agent` Phase 2 完成时，新增文件 `sdriver/scrcpy/root_agent_scrcpy_transport.go`，实现 `DeviceProvider + ScrcpyTransport`。
- 该 transport 注册成虚拟 bridge：

  ```go
  Bridge{
      ID:           "agent-" + agentID,
      Name:         deviceName + " (agent)",
      Transport:    TransportRootAgent,
      SecurityMode: SecurityShimHMAC,
      Enabled:      true,
  }
  ```

- 注册时机：root-agent 完成 hello + auth 后，写入 in-memory 注册表；session 断开时删除（不持久化）。
- 前端在桥列表里显示该桥的 `transport=root_agent`，没有 "test" 按钮（已经是 push 协议，无意义），有 "disconnect agent" 按钮。

## 不在本变更范围的开放问题

- 多用户 ACL、设备分组权限：未来变更。
- 桥端 shim agent（替换 socat）：另立变更，复用 root-agent 的 control JSON + raw stream 协议。
- Linux driver（`sdriver/linux`、`linuxRecorder`）的 transport 抽象：另立变更，本变更不动。
- WebRTC 媒体下沉到桥：暂缓。
