# Seam: device-transport-refactor ↔ webscreen-root-agent

本文档定义两个并行 OpenSpec change 之间的接缝。任何一侧改动若影响下列约束，必须同时更新另一侧并 cross-link PR。

## 约束 1: root-agent transport 只挂 DeviceProvider + ScrcpyTransport

`webscreen-root-agent` 提供的 transport 实现：

```go
// 由 webscreen-root-agent 在 Phase 2 完成后新增
type rootAgentScrcpyTransport struct {
    listener     net.Listener
    sessions     *agentSessionRegistry   // 已在 sdriver/scrcpy/agent_transport.go
    pairingToken []byte
    // ...
}

var (
    _ DeviceProvider  = (*rootAgentScrcpyTransport)(nil)
    _ ScrcpyTransport = (*rootAgentScrcpyTransport)(nil)
    // 故意不实现 ADBTransport
)
```

`device-transport-refactor` 必须保证：

- `ADBTransport` 接口签名不强行要求实现 `ScrcpyTransport` 之外的能力。
- `BridgeRegistry.GetADB(bridgeID)` 对 `transport=root_agent` 的桥返回 `ErrTransportNotADB`，不 panic、不 fallback。
- 任何调用方需要 ADB 能力时必须通过 `GetADB`，不能用 `interface{}` 类型断言强转 `rootAgentScrcpyTransport`。

## 约束 2: ScrcpySession 是接缝唯一交付物

`webscreen-root-agent` 当前已经在 `sdriver/scrcpy/agent_transport.go:163-172` 直接调用 `da.readDeviceMeta` / `da.assignConn`。重构后，agent transport 不再直接动 ScrcpyDriver 内部方法，而是把三条 raw stream 通过 `ScrcpySession` 返回：

```go
type ScrcpySession struct {
    SCID              string
    Video             net.Conn // raw bytes, scrcpy 原协议序列
    Audio             net.Conn // opts.Audio=false 时为 nil
    Control           net.Conn // opts.Control=false 时为 nil
    NeedsForwardDummy bool     // root-agent transport 必须返回 true，详见约束 5.1
    Close             func() error
}
```

ScrcpyDriver 改造（属于 `device-transport-refactor` Phase 0b）：

- 把 `connectRemoteADBSockets`（`driver.go:408`）和 `agent_transport.go:135-180` 的 stream 收集逻辑提取到 helper `gatherScrcpyConns(transport ScrcpyTransport) (*ScrcpySession, error)`。
- helper 按 `session.NeedsForwardDummy` 决定是否先读 1 字节 dummy，再统一 `readDeviceMeta(firstConn) → assignConn(...)`。`readDeviceMeta` / `assignConn` 序列对所有 transport 保持不变（design.md 门禁：root-agent 不消费 dummy byte / device meta / codec header；helper 自己读 dummy 不算 agent 消费）。
- ScrcpyDriver `New(...)` 接受 `ScrcpyTransport`，调用 `StartScrcpySession` 拿到 session，然后跑现有 `transferVideo / transferAudio / control` loop。

## 约束 3: 虚拟 bridge 注册

`webscreen-root-agent` agent 完成 hello + auth 后：

```go
registry.RegisterEphemeral(Bridge{
    ID:           "agent-" + agentID,
    Name:         deviceName + " (agent)",
    Transport:    TransportRootAgent,
    SecurityMode: SecurityShimHMAC,    // agent 已做 HMAC，不是 insecure
    Enabled:      true,
})
```

`device-transport-refactor` Phase 1a 必须保证：

- `RegisterEphemeral` 不写盘。
- ephemeral bridge 在前端 UI 显示 transport=root_agent，**禁用** delete / toggle 按钮（生命周期由 agent session 控制）。
- agent session 断开时调用 `registry.Unregister(bridgeID)`。

## 约束 4: 健康检查路径

root-agent 桥的健康检查**不走** TCP / `adb -H -P devices -l`（adb host 协议在 agent 桥上不可用）。改走：

- 检查 `agentSessionRegistry` 内 session 是否 active。
- 上次 heartbeat 时间是否在窗口内（design.md:317-335 已定义 ping/pong）。

`Bridge.Probe` 实现按 `Transport` 字段分派：

```go
switch b.Transport {
case TransportLocalADB, TransportRemoteADB:
    return b.probeADB(ctx, deep)
case TransportRootAgent:
    return b.probeAgent(ctx)
}
```

`probeAgent` 仅查 in-memory session 状态，不开网络连接（agent 是主动连进来的）。

## 约束 5: scid 来源

`webscreen-root-agent` design.md:238 定义 config 由 Windows 下发 `scid`。`device-transport-refactor` Phase 0b 把 ScrcpyDriver 的 scid 改为 `GenerateSCID()` per-instance。两者通过 `ScrcpyOptions.SCID` 字段（design.md "ScrcpyTransport" 节定义）协同：

- ScrcpyDriver 启动时 `GenerateSCID()`，存 `da.scid`。
- 调用 `transport.StartScrcpySession(ctx, ref, ScrcpyOptions{SCID: da.scid, ...})`。
- `ScrcpyOptions.SCID` 为空字符串时 transport 必须返回 `ErrSCIDRequired`，不允许 transport 内部默认填值。
- `rootAgentScrcpyTransport` 把 `opts.SCID` 写入 control 协议的 config message 下发给 agent。
- agent 启 scrcpy-server 时用该 scid。
- 多并发 driver 各自 scid 唯一，agent 收到 stream_attach 时按 session_id（不是 scid）路由，session_id 也已经唯一。

## 约束 5.1: NeedsForwardDummy=true（root-agent 必须）

`device-transport-refactor` design.md "Dummy byte 状态对照表" 规定：root-agent transport 返回的 `ScrcpySession.NeedsForwardDummy` **必须为 `true`**。原因：

- scrcpy-server 在 device 侧用 forward 隧道模式启动（`tunnel_forward=true`）。
- agent 主动 dial PC 后，PC 收到的第一字节是 scrcpy 的 dummy byte（design.md:155-161 已规定 agent 不消费 dummy）。
- `gatherScrcpyConns` helper 看 `session.NeedsForwardDummy=true` → 调用 `io.ReadFull(firstConn, dummy[:1])` → 然后 `readDeviceMeta(firstConn)`。

实现侧：

```go
func (t *rootAgentScrcpyTransport) StartScrcpySession(ctx context.Context, ref DeviceRef, opts ScrcpyOptions) (*ScrcpySession, error) {
    // ... 等待 session、下发 config、收集三条 stream attach
    return &ScrcpySession{
        SCID:              opts.SCID,
        Video:             videoConn,
        Audio:             audioConn,
        Control:           controlConn,
        NeedsForwardDummy: true,        // 强制 true
        Close:             closeFn,
    }, nil
}
```

## 约束 6: Phase 时序

| 时间 | device-transport-refactor | webscreen-root-agent |
|---|---|---|
| 现在 | Phase 0a 起 | Phase 2 进行中 (Windows transport 已部分落地) |
| Phase 0a 完成后 | 接口定义稳定 | 可以开始把 `rootAgentScrcpyTransport` 改造成实现 `ScrcpyTransport` 接口 |
| Phase 0b 完成后 | ScrcpyDriver 改造完成 | agent_transport.go 的 stream 收集逻辑迁移到 `StartScrcpySession` |
| Phase 1a 完成后 | bridge registry 可用 | `RegisterEphemeral` 调用点接入 |
| Phase 1b 完成后 | API 可用 | UI 显示 agent bridge |
| Phase 1d 完成后 | UI 完整 | root-agent Phase 5 真机验收时一并验证多桥场景 |

跨 change 的合并顺序：`device-transport-refactor` Phase 0a/0b 必须先于 `webscreen-root-agent` Phase 2 的 stream 收集重构。

## 约束 7: 不重叠的范围

- 协议 (control JSON / stream attach / heartbeat / file push) → 完全归 `webscreen-root-agent`。本变更不改协议、不改 fixture。
- HMAC token 生命周期、replay window、SELinux → 完全归 `webscreen-root-agent`。
- 接口定义、bridge 注册表、health probe、CheckOrigin、audit、并发清算 → 完全归 `device-transport-refactor`。本变更内不改 `agent_proto.go` / `agent_transport.go` 的协议逻辑。

如果实现时发现需要跨界，必须先停下来在两个 change 各加一条 task 项，跨界改动作为新 PR 单独 review。
