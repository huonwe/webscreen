# TDD: device-transport-refactor

## 原则

先用纯函数 + fake transport 锁住接口契约、DeviceRef 编解码、并发不变量、健康探针分支；再接 fake remote-adb-server 锁住网络层；最后跑真实 socat / 真实设备的 regression。

任何阶段失败都不能用 fallback / cache / global state 掩盖。具体：

- DeviceRef decode 失败必须返回明确 error，禁止静默 fallback 到默认设备。
- 桥探针失败必须写入 `BridgeStatus.LastError`，禁止用上次成功结果遮盖。
- transport 调用超时必须返回 error，禁止 retry 隐藏。
- audit 日志写入失败必须 fail-loud（stderr + 计数器），禁止 swallow。

## 设计门禁验证（红测先行）

四个门禁每个都要有专门的红测，禁止在实现里偷偷绕过。

### 门禁 1: RootAgentTransport 不实现 ADBTransport

红测：

- 编译期：`var _ ADBTransport = (*rootAgentScrcpyTransport)(nil)` 必须**编译失败**。在 Phase 0a 加一个 `compile_assertion_test.go`，里面只放正向断言：

  ```go
  var (
      _ DeviceProvider  = (*rootAgentScrcpyTransport)(nil)
      _ ScrcpyTransport = (*rootAgentScrcpyTransport)(nil)
      // 注意：故意不断言 ADBTransport
  )
  ```

- 运行期：调用者要拿到 ADBTransport 必须通过 type assertion；`rootAgentScrcpyTransport` 在 `BridgeRegistry.GetADB(bridgeID)` 上必须返回 `ErrTransportNotADB`。红测覆盖。

### 门禁 2: socat 兼容期不靠 token

红测：

- `Bridge{SecurityMode: SecurityInsecureSocat}` 在 `Add` 时即使带 token 也必须**丢弃 token + warn**，注册表里 `TokenRef == ""`。
- `Bridge.Probe` 不读 token。
- webscreen 启动时如果发现至少一个 insecure 桥，必须打印 `[bridge] WARNING: insecure_socat bridge <id> present; bridge-side ACL required (socat bind, host firewall, Tailscale ACL). webscreen cannot enforce this.`。红测捕获该日志。
- 该警告与 webscreen 自身的 `-host` bind 参数**无关**：测试覆盖 `-host 0.0.0.0` 和 `-host 100.x.y.z` 两种启动方式，警告内容必须保持一致（不出现"请把 webscreen 端绑到 Tailscale IP"之类误导文案）。

### 门禁 3: 健康检查不用 OpenLocalAbstract

红测：

- `Bridge.Probe` 代码路径里 grep 不到 `OpenLocalAbstract` 调用。用 static check：probe 函数内只允许 `Dial`, `Devices`, `Shell`。
- 用 fake adb server：返回 `OKAY` for `host:version` / `host:devices-l`，**拒绝** `localabstract:` 请求。probe 必须返回 `TCPOk=true, ADBOk=true, DeviceCount=0`，不报错。
- 用 fake server：device 列表里包含 `unauthorized` 状态。probe 必须返回 `DeviceCount=1` 不报 fail。
- deep test：fake shell echo 返回 `alive\n`。probe 必须返回 `LatencyMs > 0` 且无 error。

### 门禁 4: DeviceRef 版本化

红测：

- 字段顺序：`Encode()` 输出 JSON 内字段必须按 ascii 排序（`agent_id`, `bridge_id`, `serial`, `transport`），不同顺序构造的 ref 编码完全相同。
- `Decode("v2.xxx")` 返回 `ErrUnknownDeviceRefVersion`，不尝试解析。
- `Decode("v1.<corrupt-base64>")` 返回 `ErrInvalidDeviceRef`。
- `Decode("v1.<base64 of unknown transport>")` 返回 `ErrUnknownTransport`。
- `Decode("5f9e7947")` 返回 `DeviceRef{Transport: TransportLocalADB, Serial: "5f9e7947"}` 并打 warn 日志（红测捕获 warn）。
- legacy 路径只接受 `[A-Za-z0-9._:-]{1,64}`（**`:` 允许**：Wi-Fi ADB serial 常见形如 `192.168.1.100:5555`）。含 `/`、`|`、空白、控制字符（< 0x20 或 0x7f）的输入按 ErrInvalidDeviceRef 拒绝，避免路径注入和分隔符歧义。
- Display / Model 字段变化不影响 `Encode()` 结果（用同 ref 不同 display 构造 DeviceDescriptor，两个 ref 编码相等）。

## Phase 0a 红测

### 1. DeviceRef 编解码

fixture：`fixtures/device_ref/`

- `local_5f9e7947.json` → `local_adb / / 5f9e7947 / `
- `remote_mac_coral_5f9e7947.json` → `remote_adb / mac-coral / 5f9e7947 / `
- `root_agent_xyz.json` → `root_agent / agent-xyz / / xyz`
- `legacy_bare_serial.txt` → legacy 字符串

红测：

- 每个 fixture round-trip：JSON → DeviceRef → Encode() → Decode() → 等于原 DeviceRef。
- 各种 reject 用例（见门禁 4）。
- Encode 输出长度 < 256（避免 URL 过长）。

### 2. ADBTransport 契约 —— CLI exec 与 raw socket 分两类测

按 design.md "adb 可执行注入策略" 二分。两类测试**不重叠**：

**CLI 类（mock exec.Command 捕获 argv）**：

- 用包装层 `execCommand var = exec.Command` 在测试里替换为捕获实现（参考 Go stdlib 测试惯例）。
- `LocalADBTransport.New(adbPath="/fake/adb")`，调用 `.Devices(ctx)` → 期望 argv `["/fake/adb", "devices", "-l"]`。
- `LocalADBTransport.Shell(ctx, "5f9e7947", "echo", "hi")` → argv `["/fake/adb", "-s", "5f9e7947", "shell", "echo", "hi"]`。
- `LocalADBTransport.Push(ctx, "5f9e7947", "src", "dst")` → argv `["/fake/adb", "-s", "5f9e7947", "push", "src", "dst"]`。
- `RemoteADBTransport.New(host="100.85.28.3", port=15037, adbPath="/fake/adb")` → 任何 CLI 调用 argv 前两段必须是 `["/fake/adb", "-H", "100.85.28.3", "-P", "15037", ...]`。
- `RemoteADBTransport.Devices` 解析 fake stdout `"5f9e7947 device usb:3-1 product:NE2210 ..."`，返回的 `DeviceDescriptor.Ref.BridgeID` 等于构造时传入的 bridge id。
- `RemoteADBTransport` 测试**不**起 raw adb-server，全部走 stdout 文本解析。
- 构造 `RemoteADBTransport` 时不传 `adbPath` → 编译失败或返回 `ErrADBExecutableRequired`。

**Raw socket 类（fake adb-server 监听 TCP，按 host protocol 响应）**：

- 仅覆盖 `OpenLocalAbstract`。
- fake server 在 `127.0.0.1:0` 监听，按 adb host protocol：
  - 收到 `0017host:transport:5f9e7947\n` → 回 `OKAY`。
  - 收到 `0014localabstract:scrcpy_<scid>\n` → 回 `OKAY` 后进入 raw byte 透传模式。
  - 其他 host 命令返回 `FAIL` + 错误信息。
- 测试用例：
  - 正常路径：`RemoteADBTransport.OpenLocalAbstract(ctx, "5f9e7947", "scrcpy_AB12CD34")` → 返回 net.Conn，写入 `"hello"` → fake server 收到 `"hello"`。
  - serial 不存在：server 返回 `FAIL` → transport 返回 redacted error。
  - localabstract 不存在：server 返回 `FAIL` → transport 返回 error，且 outer conn 被 close。
- 不支持的能力（如 LocalADB 用 `OpenLocalAbstract` 但 caps 标 false）调用返回 `ErrCapabilityNotSupported`，不 panic。

### 3. adb 调用收口

静态检查（CI script）：

```bash
grep -RnE 'exec\.Command(Context)?\([^,]+,\s*"adb"' sdriver/ webservice/ utils/ \
  | grep -v 'sdriver/scrcpy/local_adb_transport.go' \
  | grep -v 'sdriver/scrcpy/remote_adb_transport.go'
# expect empty
```

加入 PR CI。

### 4. encoder 探测路由

红测：

- 在 fake remote-adb bridge 上挂一个返回 `opus.encoder` 的 shell handler。
- 创建 `ScrcpyDriver` 时传 `RemoteADBTransport`。
- `SupportOpusAudio()` 必须**通过 transport** 跑，命中 fake bridge handler，而不是本地 adb。
- 验证方式：fake remote bridge 设置 `shellCallCount`，断言 `>=1`；同时验证本地 `exec.Command("adb", ...)` 没被调用。

## Phase 0b 红测

### 1. scid 唯一性

- 并发 100 次 `GenerateSCID()`，结果全集无重复。
- 单 driver 实例 `scid` 字段在构造后不可变；测试反射验证字段非空、非 `"00000000"`。

### 2. 临时目录隔离

- 并发起 4 个 driver，各 `tmpDir` 路径不同。
- 每个 tmpDir 权限 0700。
- `Close()` 后 tmpDir 不存在；强 kill 后用 cleanup goroutine 兜底（process exit 时清理）。

### 3. reverse 端口动态分配

- 起 driver A，reverse 用端口 `pA`；起 driver B，reverse 用端口 `pB`；`pA != pB`。
- 关闭 driver A 后 `pA` 立刻可被新 driver 复用。

### 4. Close 精确 kill

- mock shell 记录所有 kill 命令；`Close()` 必须发 `pkill -f scrcpy-server.*scid=<本 driver 的 scid>`，**不能**发 `pkill app_process`、`pkill -f scrcpy-server`（无 scid 过滤）。

## Phase 0c 红测

### 1. CheckOrigin

- `same-host` 默认：`Origin: http://localhost:8081` + `Host: localhost:8081` → 通过。
- 跨 origin：`Origin: http://evil.com` + `Host: localhost:8081` → 拒绝（返回 false）。
- 显式 allowlist `["http://localhost:8081"]`：精确匹配通过；前后空格 / 大小写差异拒绝。
- `Origin: null`（file:// 或某些跨域场景）→ 拒绝。

### 2. Audit 日志

- 调用 `transport.Shell(ctx, serial, "su", "-c", "echo SECRET")`，audit 条目里 `Summary` 不含 `SECRET`，只含 `shell` 和 redacted summary。
- token / PIN 注入到任意调用参数，都不进 audit。
- shell stdout 不进 audit（只记 exit code / 时长）。
- audit 写入失败时 stderr 报错且 metric `audit_write_failures` +1，不 swallow。

### 3. transport timeout

- fake transport `Shell` 故意 sleep 10s，外层 `WithTimeout(5s)` → 必须返回 `context.DeadlineExceeded` 包装错误，bridge `LastError` 包含 `timeout`。
- 调用方可传更长 ctx 覆盖默认 5s（不是硬编码）。

### 4. GetADBPath 收敛

- `RemoteADBTransport` 构造 + Devices 调用 → grep 不到 `GetADBPath` 调用 stack。
- `LocalADBTransport.New()` 未找到 adb 时下载到 `os.UserCacheDir()/webscreen/adb/`，**不写 cwd**。
- 下载文件名含版本号，不覆盖既有版本。

## Phase 0d 红测

### 1. handleScreen.go subscriber key

- POST 含 `device_ref: "v1.xxx"` → subscriber key 等于该 ref encoded。
- POST 仅含 legacy `device_id: "5f9e7947"`（无 device_ref）→ subscriber key 等于 `DeviceRef{Transport:local_adb, Serial:"5f9e7947"}.Encode()`。
- 同一台设备两种 payload 同时连入，落到同一个 subscriber slot，不会被当成两台设备。
- 返回的 `DeviceInfo` JSON 必须**同时**包含 `device_id`（legacy serial）、`device_ref`（v1 编码）、`bridge_id`、`display`、`model` 五个字段。

### 2. localStorage 迁移（按真实结构）

当前结构（`public/static/console.js:6-7`）：

```js
localStorage.webscreen_device_configs = JSON.stringify({
    "5f9e7947": { pin: "...", display: "OnePlus", ... },
    "abc123":   { ... }
});
localStorage.webscreen_ignored_devices = JSON.stringify(["serial-x", "serial-y"]);
```

jsdom 测试用例：

- **配置对象迁移**：
  - 注入上面的 `webscreen_device_configs`。
  - 跑 `migrateDeviceConfigs()`。
  - 断言每个 entry 新增 `_ref` 字段，值是 `v1.` 编码的 DeviceRef（Transport=local_adb, Serial=该 key）。
  - 断言老 key（`"5f9e7947"`, `"abc123"`）**仍在**，配置内容不变。
  - 断言 `localStorage.webscreen_storage_schema_version === "2"`。
- **忽略列表迁移**：
  - 注入 `webscreen_ignored_devices = ["serial-x", "serial-y"]`。
  - 跑迁移。
  - 断言 `webscreen_ignored_device_refs = [v1.<x>, v1.<y>]`，老数组保留。
- **幂等**：
  - 已迁移过的 localStorage 再跑一次迁移 → 状态完全不变（含字段顺序、字符串内容）。
- **30 天清理（未来 Phase）**：
  - 注入 `webscreen_storage_migrated_at = 30 天前时间戳`。
  - 调用 `cleanupLegacyDeviceKeys()`（本 Phase 仅写函数桩，下一 Phase 启用）。
  - 当前 Phase 测试：函数存在、被调用时不抛错。
- **新增 entry 后兼容**：
  - 迁移完成后，用户在新前端 rename 设备 → 既写 `_ref` 字段也写 legacy serial 字段，旧前端打开仍可见。

### 3. 老链接兼容（本 Phase 不做 301）

- GET `/screen/5f9e7947` → 200 OK，subscriber key 走 legacy 升格成 local_adb ref。
- GET `/screen/v1.invalid_payload` → 400 `invalid_device_ref`。
- GET `/screen/../etc/passwd` → 400 拒绝（路径含特殊字符）。
- 301 跳转留待后续 Phase（在 device_id 字段最终下线前不强制 301，避免老书签突然失效）。

## Phase 1a 红测

### 1. 持久化

- 写入 `bridges.json` 后立刻读回，内容相同。
- 文件权限 0600；改成 0644 后启动时 `refuse to start` 并提示。
- 文件 version != 1 时返回 `unsupported_config_version`。
- 写入失败（磁盘满 / 权限错）时返回明确 error，不 swallow。

### 2. Bootstrap ephemeral

- 设 `WEBSCREEN_REMOTE_ADB=1, HOST, PORT`，启动后 registry 包含 `bootstrap-env` bridge，`bridges.json` 不变。
- 进程退出，重启后 ephemeral 消失（如 env 仍在，新一次启动再注册）。
- 持久 bridge 与 ephemeral 同 ID 时拒绝持久化（保留 ID 名 `bootstrap-env`）。

### 3. Token 存储

- Windows：调用 DPAPI 模拟；失败 fallback 0600 文件。
- 文件 fallback 权限校验。
- `insecure_socat` bridge 即使带 token，token 也不被存储（红测）。

## Phase 1b 红测

### 1. CRUD API

- `POST /api/bridge/add` 缺字段返回 400。
- 重复 add 同 host:port 返回 409 conflict。
- DELETE `bootstrap-env` 返回 409 + 提示。
- 全部 API 在无 PIN cookie 时 401。

### 2. /api/device/list fan-out

- 3 桥并发：A 1s 返回 2 设备，B 1s 返回 1 设备，C 超时 3s。
- 总响应时间 ≤ 3.5s。
- 返回设备总数 3，每个设备项带正确 `device_ref`（含对应 bridge_id）。
- C 桥状态标记为 `LastError: "timeout"`，但不阻塞 A/B 返回。

### 3. 序列化稳定

- `GET /api/bridge/list` 响应 JSON key 顺序稳定（按 ID asc）。
- token 字段绝不出现在响应里。

## Phase 1c 红测

### 1. 三段式 probe

每条用 fake bridge 单独覆盖：

- TCP refused → `TCPOk=false, ADBOk=false`。
- TCP ok + adb fail（返回 `FAIL`）→ `TCPOk=true, ADBOk=false, LastError` 含 redacted adb 错误。
- TCP ok + adb ok + 空设备 → `TCPOk=true, ADBOk=true, DeviceCount=0, LastError=""`。
- TCP ok + adb ok + 1 unauthorized → `DeviceCount=1`，不报 fail。
- deep + per-device shell timeout → `LastError` 含设备 serial（但不含 stdout）。

### 2. 后台 probe goroutine

- 起 webscreen，2 桥 enabled，30s 后两桥 status 都更新。
- 桥连续 3 次 fail，状态变 `disabled_unhealthy`，UI 标灰。
- 一旦恢复，下一次 probe 标回 enabled。

### 3. Error redact

- redact 规则：去除 IP、token、文件路径中的用户名。
- 红测：错误字符串 `dial tcp 100.85.28.3:15037: connection refused` → redact 后只保留 `connection refused` + bridge id。

## Phase 1d 红测（jsdom）

- 桥列表渲染：3 桥按 id asc。
- insecure 桥渲染 `aria-label="unauthenticated bridge"` + 红色徽标。
- deep test 按钮点击发 `POST /api/bridge/:id/test` body `{"deep":true}`。
- 删除 bootstrap-env 桥按钮 disabled + tooltip。
- 添加桥表单：security_mode = `insecure_socat` 时 token 输入框 disabled + 提示。

## 集成验收

### 双桥双设备

- 启动 webscreen，注册：
  - bridge A：本机 USB 直连一台 emulator。
  - bridge B：100.85.28.3:15037 socat。
- `/api/device/list` 返回 2 个设备，各自 `device_ref` 不同 bridge_id。
- 同时打开两个 stream 页面，两条 scrcpy 视频流并行 60s 无串流、无 driver 之间 crash。
- 关闭其中一个 stream，另一个不受影响。
- 看 audit 日志：双 stream 的命令各自标 bridge id，无串。

### 老链接兼容

- 用户 bookmark `http://localhost:8081/screen/5f9e7947` → 200 OK 正常进入 stream（subscriber key 走 legacy 升格到 `DeviceRef{Transport:local_adb, Serial:"5f9e7947"}`）。本变更**不做** 301 跳转 —— 老书签必须继续可用。

### CheckOrigin

- `curl -H "Origin: http://evil.com" ws://localhost:8081/screen/ws` → 403。

### 桥故障

- 启动后 kill Mac socat。30s 内 UI 桥 B 标红，桥 B 设备从 device list 消失或标 stale。
- 恢复 socat。下个 probe 周期内桥 B 标回 ok。

## 不测试范围

- root-agent transport（由 `webscreen-root-agent` 变更负责测试）。
- WebRTC 编解码（不在本变更范围）。
- linux driver（不在本变更范围）。
- 媒体下沉到桥（暂缓）。

## Fixture 目录

```text
openspec/changes/device-transport-refactor/fixtures/
  device_ref/
    local_5f9e7947.json
    remote_mac_coral_5f9e7947.json
    root_agent_xyz.json
    legacy_bare_serial.txt
    invalid_v2.json
    invalid_corrupt_base64.json
    invalid_unknown_transport.json
    invalid_path_injection.json
  bridge_config/
    valid_minimal.json
    valid_dual_bridge.json
    invalid_version.json
    invalid_bad_perm.json
  probe_responses/
    tcp_refused.txt
    adb_fail.bin
    adb_ok_empty.bin
    adb_ok_one_unauthorized.bin
    deep_shell_alive.txt
  audit_redact/
    contains_token.txt
    contains_pin.txt
    contains_stdout.txt
    expected_redacted.txt
```

实现 PR 时按 Phase 复制对应 fixture 到测试目录；OpenSpec fixture 作为协议金标准。
