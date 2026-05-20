# Tasks: device-transport-refactor

## 0. 审核门禁

- [x] 用户拍板 Phase 顺序：Phase 0a → 0b → 0c → 0d → 1a → 1b → 1c → 1d，跨 Phase 不合并 PR。
- [x] 用户确认 RootAgentTransport 不实现 ADBTransport，接口三分。
- [x] 用户确认 socat 兼容期标 `insecure_socat`，安全责任在桥机一侧：桥机 socat `bind=Tailscale IP` + 桥机宿主防火墙 + Tailscale ACL。webscreen 端只显示警告，无法替代桥侧 ACL，不靠 token。
- [x] 用户确认健康检查不用 `OpenLocalAbstract`，三段式：TCP / `adb -H -P devices -l` / 可选 per-device shell。
- [x] 用户确认 DeviceRef 编码为 `v1.<base64url(json)>`，legacy 裸 serial 按 local_adb 迁移并 warn。
- [x] 用户确认 `WEBSCREEN_REMOTE_ADB_*` 降级为 bootstrap ephemeral bridge，不持久化。
- [x] 用户确认本变更不动 streamAgent / linuxRecorder / sdriver/linux / WebRTC 主链路。
- [x] 用户确认本变更不实现 root-agent transport，只确保接口能容纳；具体实现走 `webscreen-root-agent` 变更。
- [x] 用户确认 Go toolchain 政策：本地装 1.25.4 或开启 `GOTOOLCHAIN=auto` 自动下载（go.mod 第 3 行已确认 `go 1.25.4`，无 toolchain 行）。**不**为本机 1.23.6 降项目版本。
- [x] 用户确认 token 存储策略：Windows DPAPI 优先，回退 0600 文件（macOS Keychain / Linux libsecret 留待 Phase 1c 补）。
- [x] 用户确认前端 localStorage 老 key 保留窗口：默认 30 天后清理；迁移目标按 `webscreen_device_configs` / `webscreen_ignored_devices` 真实结构走。

## 1. Baseline

- [x] 列出全部 12 处 ADB 调用点和 2 种调用风格（写入 proposal 背景表）。
- [x] 列出 scrcpy driver 三处并发不安全位点（写入 design）。
- [x] 列出 14 个引用 `device_id` 的文件（grep 结果归档）。
- [ ] 记录现有 launcher（`launcher/`）启动 webscreen 时设置的全部 env，确认 bootstrap 兼容层覆盖完整。
- [ ] 记录现有 frontend localStorage key 名空间（grep `webscreen.` 前缀），确认迁移脚本范围。

## 2. Phase 0a: 接口拆分 + adb 调用收口

PR 边界：纯重构，不改运行时行为。

- [x] 新增 `sdriver/scrcpy/transport.go`：`DeviceProvider` / `ADBTransport` / `ScrcpyTransport` / `ADBTransportCaps`。
- [x] 新增 `sdriver/device_ref.go`：`DeviceRef`, `Encode/Decode`, legacy 迁移 helper, 单元测试覆盖 `fixtures/device_ref/*.json`。
- [x] 新增 `sdriver/scrcpy/local_adb_transport.go`：实现 `ADBTransport`，封装现有 `utils.GetADBPath()` + `exec`。
- [x] 新增 `sdriver/scrcpy/remote_adb_transport.go`：替换 `remote_adb.go` 现有全局 env 读取，构造函数显式接受 `host, port`，实现 `ADBTransport`（`OpenLocalAbstract` 复用现有 `adbService` 逻辑）。
- [x] 新增 `sdriver/scrcpy/adb_scrcpy_transport.go`：内部 adapter，`adbScrcpyTransport{adb ADBTransport}` 实现 `ScrcpyTransport`。
- [x] 替换 `webservice/android/connect.go:11/23/65/82`：所有调用走 `ADBTransport`。
- [x] 替换 `sdriver/scrcpy/adb.go:122/159`：硬编码 `"adb"` 改走 `transport.Shell(...)`。
- [x] 删除 `sdriver/scrcpy/adbutils.go` 中与 `webservice/android` 重复的 `ConnectDevice/PairDevice/GetConnectedDevices`，保留通用 helper（`scrcpyParamsToArgs`, `toScrcpyCommand`）。
- [x] 删除 `webservice/android/connect.go:11 ExecADB` 重复定义，保留单一定义在 transport 实现内。
- [x] `LocalADBTransport.New()`：调用 `utils.GetADBPath()` 取得路径并注入到 `ADBExecutable` 字段；CLI 操作走 `exec(ADBExecutable, ...)`。
- [x] `RemoteADBTransport.New(host, port, adbExecutable)`：构造时显式接受 `adbExecutable`；不调用 `GetADBPath`；CLI 操作走 `exec(ADBExecutable, "-H", host, "-P", port, ...)`；`OpenLocalAbstract` 走 raw socket（复用现有 `remote_adb.go` 的 `adbService` 路径）。
- [x] 单元测试：
  - [x] `transport_contract_test.go`：用 fake transport 验证 `ADBTransport` 全部方法签名能编译。
  - [x] `device_ref_test.go`：编码 / 解码 / legacy fallback / 未知版本拒绝 / 字段约束。
  - [x] `local_adb_transport_test.go`：mock `exec.Command`，验证 `-s SERIAL` 路由。
  - [x] `remote_adb_transport_test.go`：local fake adb-server，验证 `host:transport:` + `devices` / `OpenLocalAbstract`。
- [ ] 回归：`go test ./...` 通过；旧 launcher 启动 webscreen 后 `/api/device/list` 行为不变。

## 3. Phase 0b: scrcpy driver 并发清算

PR 边界：动 `sdriver/scrcpy/driver.go` 内 New/Close 及临时文件管理；不改协议、不改 transport 接口。

- [ ] `ScrcpyDriver` 新增 `tmpDir`, `proxyLn` 字段。
- [ ] `New(...)`：
  - [ ] `scid = GenerateSCID()` 替换 `sdriver/scrcpy/driver.go:86` 的 `"00000000"`（scid 改动**只在本 Phase 出现**，0a 不动）。
  - [ ] `ScrcpyOptions.SCID` 必填（空字符串时 transport 拒绝并报 `ErrSCIDRequired`），由 ScrcpyDriver 构造 session 前注入。
  - [ ] `os.MkdirTemp("", "scrcpy-srv-*")` 替换 `SCRCPY_SERVER_LOCAL_PATH`。
  - [ ] reverse 模式 `net.Listen("127.0.0.1:0")` 拿动态端口；direct 模式直接调 `OpenLocalAbstract`。
  - [ ] 提取 `gatherScrcpyConns(transport ScrcpyTransport, opts ScrcpyOptions) (*ScrcpySession, error)` helper：
    - reverse 路径（LocalADB tunnel_forward=false）：accept 三条 conn，**不读 dummy**，直接 `readDeviceMeta(firstConn)`，对应 `session.NeedsForwardDummy=false`。
    - direct 路径（RemoteADB / LocalADB forward）：拿到三条 raw conn 后 `io.ReadFull(firstConn, dummy[:1])` 再 `readDeviceMeta(firstConn)`，对应 `session.NeedsForwardDummy=true`。
    - 把 `driver.go:331 connectRemoteADBSockets` 和 `driver.go:344-384` 的 reverse Accept 块合并到这个 helper，去掉两条重复路径。
    - root-agent 已在 `agent_transport.go:135-180` 完成 stream 收集；按 SEAM 约束 2 在 webscreen-root-agent 侧迁移到 `StartScrcpySession` 返回 `ScrcpySession{NeedsForwardDummy: true}`。
- [ ] `Close()`：删 tmpDir、关 proxyLn、`ReverseRemove`、按 `scid` 精确 kill 远端 scrcpy-server 进程。
- [ ] 删除常量 `SCRCPY_SERVER_LOCAL_PATH` / `SCRCPY_PROXY_PORT_DEFAULT`（grep 确认无外部引用）。
- [ ] 修改 `driver.go:127/132` 改成 tmpDir 路径。
- [ ] 单元测试：
  - [ ] `driver_concurrent_test.go`：并发起 4 个 driver 实例，验证 scid / tmpDir / 端口互不冲突。
  - [ ] `driver_cleanup_test.go`：`Close()` 后 tmpDir 被删除，proxyLn 关闭。
- [ ] 集成测试（手动）：用单 webscreen 进程对同一桥的两个设备各起 session，观察两条 scrcpy 视频流并行。
- [ ] 回归：单设备路径行为不变；旧 reverse 模式仍可用。

## 4. Phase 0c: 横切安全（CheckOrigin / audit / timeout / GetADBPath）

- [ ] `webservice/handleScreen.go:15`：`CheckOrigin` 改为读 `wm.config.AllowedOrigins`，默认 `same-host`。
- [ ] `WebMaster` 添加 `SetAllowedOrigins([]string)` 方法；启动 flag `-allow-origin` 支持多次。
- [ ] 新增 `sdriver/scrcpy/audit.go`：`AuditEntry` + 全局 logger。
- [ ] 所有 `ADBTransport` 方法在 transport 实现层包装 audit 写入。
- [ ] 所有 `ADBTransport` 方法在调用层强制 `context.WithTimeout(5s)`，可被外层 ctx 覆盖。
- [ ] `utils/adb.go:GetADBPath`：
  - [ ] 下载路径从 `cwd` 改为 `os.UserCacheDir()/webscreen/adb/`。
  - [ ] 文件名包含版本号。
  - [ ] 仅 `LocalADBTransport` 调用；`RemoteADBTransport` 移除依赖。
- [ ] 单元测试：
  - [ ] `check_origin_test.go`：same-host 通过；跨 origin 拒绝；显式 allowlist。
  - [ ] `audit_test.go`：token / PIN / shell 内容不进 audit。
  - [ ] `transport_timeout_test.go`：transport 5s 超时返回明确 error。
- [ ] 文档：更新 `launcher/` README 提到新增 flag。

## 5. Phase 0d: 前端字段加 device_ref（device_id 保 legacy 兼容期）

**重要**：当前前端的 `device_id` 是设备名 / 配置 key / 显示，不能直接替换成 ref encoded（ref 太长，且 console.js 大量按 serial 索引 `webscreen_device_configs`）。本 Phase 走"新增字段"路线，旧字段保留 legacy 语义到迁移期结束。

- [ ] `webservice/apiDevices.go DeviceInfo` 新增字段：
  - `device_ref` (string): `DeviceRef.Encode()` 的 v1 编码，新前端用。
  - `bridge_id` (string): bridge 显示用。
  - `display` (string): UI 显示名（默认 model，可被前端 rename 覆盖）。
  - `model` (string): adb 返回的 model 字段。
  - 保留 `device_id` (string): **legacy 裸 serial**，未来 phase 才退场；本 phase 不动其语义，避免老前端炸。
- [ ] `webservice/handleScreen.go:51`：`deviceIdentifier` 改为优先用 `config.DeviceRef`（若为空 fallback `DeviceID`），底层走 `DecodeDeviceRef` 容错。
- [ ] `webservice/webRTCManager.go`：subscriber map 的 key 改用 `DeviceRef.Encode()`；调用方传 ref encoded。`DeviceID` 老值兼容期通过 legacy decode 升格成 `DeviceRef{Transport: local_adb, Serial: <legacy>}`。
- [ ] 后端路由 `/screen/:id`：判断不是 `v1.` 时跑 legacy fallback（按 local_adb 解析）；不强制 301，老链接继续可用（待 Phase 1 之后再加 301）。
- [ ] 前端 `public/static/console.js`：
  - [ ] 在 `loadIgnoredDevices` / `STORAGE_KEY = 'webscreen_device_configs'`（`console.js:6-7`）基础上新增迁移函数 `migrateDeviceConfigs()`：
    - `webscreen_device_configs` 当前结构为 `{ [serial]: { ...config } }`。迁移目标：每个 entry 新增 `_ref` 字段（值为对应 DeviceRef encoded）；老 serial key 保留 30 天。
    - `webscreen_ignored_devices` 当前结构为 `string[]`（serial 数组）。迁移目标：新增并行数组 `webscreen_ignored_device_refs`（v1 encoded），旧数组保留 30 天。
    - 写入 metadata `webscreen_storage_schema_version = 2`，已迁移过则跳过。
    - 迁移函数必须**幂等**：多次调用结果相同。
- [ ] 前端 `public/static/connect.js` / `screen.js`：
  - [ ] 设备列表渲染时读 `device.device_ref || device.device_id`。
  - [ ] 启动 stream 的链接生成优先用 `device_ref`，无 ref 时 fallback `device_id`。
- [ ] 单元测试：
  - [ ] `device_ref_migration_test.go`（Go）：legacy bare serial → `v1.{Transport:local_adb, Serial:<legacy>}`，多次解码幂等。
  - [ ] `console_storage_migration_test.js`（jsdom）：注入 `webscreen_device_configs = {"5f9e7947": {pin: 1234}}`，跑 migrate → entry 新增 `_ref`，老 key 不丢；二次跑无副作用。
  - [ ] `handle_screen_legacy_test.go`：GET `/screen/5f9e7947` 不报错，subscriber key = local_adb encode 结果。
- [ ] 手动验证：清 localStorage 后访问老书签 `http://localhost:8081/screen/5f9e7947` → 正常进入 stream，且 console 设备列表里同时显示 `device_id=5f9e7947` 和 `device_ref=v1.xxx`。

## 6. Phase 1a: bridge registry 持久化 + bootstrap 兼容

- [ ] 新增 `webservice/bridge_registry.go`：内存 + 文件持久化（`config/bridges.json`）。
- [ ] 新增 `webservice/bridge_secret.go`：token 存储抽象（Windows DPAPI 实现 + 文件回退）。
- [ ] `main.go` 启动顺序：
  1. 加载 `config/bridges.json`（不存在则创建空）。
  2. 如检测到 `WEBSCREEN_REMOTE_ADB=1`，注册 `bootstrap-env` ephemeral bridge。
  3. 初始化 webMaster。
- [ ] `bridges.json` 文件权限 0600；启动时校验，不符合则 refuse to start 并提示修权。
- [ ] 单元测试：
  - [ ] `registry_persist_test.go`：写入 / 读取 / 版本号检测。
  - [ ] `bootstrap_env_test.go`：env 存在时注册 ephemeral，不写盘；env 不存在时无副作用。
- [ ] 回归：现有 launcher 设置 5 个 env 启动 webscreen，行为完全不变。

## 7. Phase 1b: bridge API

- [ ] 新增 `webservice/handleBridges.go`：
  - [ ] `GET /api/bridge/list`
  - [ ] `POST /api/bridge/add`
  - [ ] `POST /api/bridge/:id/test`
  - [ ] `POST /api/bridge/:id/toggle`
  - [ ] `DELETE /api/bridge/:id`
- [ ] 所有路由挂在现有 PIN 中间件下。
- [ ] `bootstrap-env` 桥拒绝 DELETE，提示用户改 env 变量。
- [ ] `GET /api/device/list` 改造：并发遍历所有 enabled bridge，3s 单桥超时，按 bridge_id 分组返回，每个设备项带 `device_ref`。
- [ ] 单元测试：
  - [ ] `handle_bridges_test.go`：CRUD 全路径。
  - [ ] `device_list_fanout_test.go`：3 个桥并发；其中 1 个故障不阻塞另外 2 个。
- [ ] 回归：单桥场景行为同 0a 末态。

## 8. Phase 1c: 健康探针

- [ ] 新增 `webservice/bridge_probe.go`：实现三段式 probe（TCP / Devices / 可选 shell echo）。
- [ ] `BridgeStatus` 写入注册表，UI 可查。
- [ ] 启动后台 goroutine：每 30s 对 enabled bridge 跑非 deep probe；失败 3 次后标 `disabled_unhealthy`。
- [ ] `POST /api/bridge/:id/test` 接受 `{deep: bool}`。
- [ ] 错误信息 redact 后写入 `LastError`，不直接 expose adb 路径 / 主机内部信息。
- [ ] 单元测试：
  - [ ] `probe_test.go`：fake bridge 模拟 TCP refused / adb fail / device echo timeout 各分支。
  - [ ] `probe_redact_test.go`：错误字符串 redact 规则。
- [ ] 验证：unauthorized 设备探针返回 `device_count=1, status=unauthorized`，不报 fail。

## 9. Phase 1d: 前端 bridge 管理 UI

- [ ] `public/static/console.js`：设备列表渲染分组按 bridge_id 分组（当前主列表渲染在 `console.js`，不在 `connect.js`）。`connect.js` 只在连接对话框里附带桥选择下拉。
- [ ] 新增 `public/static/bridges.js` + 对应 HTML 入口：桥列表、添加桥对话框、test 按钮、deep test 复选框、insecure 红标。
- [ ] insecure_socat 桥在列表里强制显示 "无鉴权" 警告。
- [ ] 删除桥时二次确认。
- [ ] 删除 `bootstrap-env` 桥时弹窗提示 "请改 env 变量"。
- [ ] 单元测试（jsdom）：
  - [ ] 桥分组渲染。
  - [ ] insecure 标记可见性。
  - [ ] deep test 触发请求带 `deep:true`。
- [ ] e2e 手动验证：
  - [ ] 启 webscreen，连两台桥（本地 + Mac socat），UI 显示两组设备。
  - [ ] 关 Mac 桥的 socat，30s 内 UI 标红。
  - [ ] 在 UI 点 deep test，返回 per-device 结果。

## 10. 验收 / 最终回归

- [ ] `grep -rn 'exec.Command.*"adb"' sdriver/ webservice/ utils/` 业务代码内为空。
- [ ] `grep -rn "WEBSCREEN_REMOTE_ADB" sdriver/ webservice/` 业务代码内为空（只剩 main.go bootstrap）。
- [ ] 单 webscreen 进程 + 两桥 + 两设备同时直播无串流。
- [ ] 老 `device_id` 链接和书签 100% 兼容。
- [ ] WebSocket 跨 origin 请求被拒。
- [ ] audit 日志 24h 不含 token / PIN / shell stdout。
- [ ] 旧 launcher 启动方式行为不变。
- [ ] 全部 OpenSpec fixture 对应单测通过。
- [ ] `webscreen-root-agent` Phase 2 PR 能在不动本变更代码前提下挂接 RootAgentTransport（对接缝预审）。

## 11. 文档

- [ ] 更新 `README.md`：多桥说明、bridge config 文件位置、安全注意。
- [ ] 新增 `doc/dev/transport.md`：接口三分 + DeviceRef 编码 + Phase 划分。
- [ ] 新增 `doc/dev/bridges.md`：桥配置示例、insecure 模式注意、Tailscale ACL 模板。
- [ ] 更新 `launcher/` README：新增 flag、新增 env 兼容说明。
