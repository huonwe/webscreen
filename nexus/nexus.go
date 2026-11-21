package nexus

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"uuid"
	"webscreen/consts"
	"webscreen/db"
	"webscreen/sdriver"
	"webscreen/sdriver/scrcpy"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v5"
)

type remoteBinding struct {
	remote          *Remote
	masterSessionID string
	viewers         map[string]*ViewSession
	viewersSnap     atomic.Pointer[[]*ViewSession] // 无锁快照，供 pumpRemote 极速读取
	stopCh          chan struct{}                  // 用于通知 pump 协程退出
}

type Nexus struct {
	sync.RWMutex
	bindings        map[string]*remoteBinding // remoteUUID -> 路由绑定关系
	sessions        map[string]*ViewSession   // sessionID -> ViewSession
	sessionToRemote map[string]string         // sessionID -> remoteUUID（反向索引，方便断线时 O(1) 查找）

	// Timestamp
	// timestampStartTime time.Time

	db  *sql.DB
	mtu uint16 // RTP 分包器的 MTU 大小，通常 1200 左右很安全
}

func NewNexus(db *sql.DB, mtu uint16) *Nexus {
	if mtu == 0 {
		mtu = 1200 // 默认 MTU 大小
	}
	return &Nexus{
		sessions:        make(map[string]*ViewSession),
		bindings:        make(map[string]*remoteBinding),
		sessionToRemote: make(map[string]string),
		db:              db,
		mtu:             mtu,
		// timestampStartTime: time.Now(),
	}
}

// 1. 连接设备并启动媒体泵
func (n *Nexus) ConnectToRemote(remoteUUID string, config RemoteConfig) error {
	n.RLock()
	if _, exists := n.bindings[remoteUUID]; exists {
		log.Printf("[nexus] Remote %s already connected, no need to connect again, skipping", remoteUUID)
		n.RUnlock()
		return nil
	}
	n.RUnlock()

	driver, err := n.setupRemote(remoteUUID, config)
	if err != nil {
		return err
	}
	if err := driver.Start(); err != nil {
		return err
	}

	n.Lock()
	if _, exists := n.bindings[remoteUUID]; exists {
		n.Unlock()
		driver.Stop()
		return nil
	}
	binding := &remoteBinding{
		remote:  &Remote{RemoteUUID: remoteUUID, sdriver: driver},
		viewers: make(map[string]*ViewSession),
		stopCh:  make(chan struct{}),
	}
	n.bindings[remoteUUID] = binding
	n.Unlock()

	// 启动该设备的专属媒体分发泵
	go n.pumpRemote(remoteUUID, binding)
	return nil
}

// 2. 媒体泵：从 SDriver 读取 AVBox，零拷贝广播给所有观众
func (n *Nexus) pumpRemote(remoteUUID string, b *remoteBinding) {
	videoCh, audioCh, _ := b.remote.sdriver.GetReceivers()
	var box sdriver.AVBox
	var ok bool
	// var localTimestamp uint32 = 0

	// var packets []*rtp.Packet
	// var pocketizer rtp.Packetizer
	// mediaMeta := b.remote.sdriver.MediaMeta()
	// if mediaMeta.VideoCodec == "h264" {
	// 	pocketizer = n.videoPacketizerH264
	// } else if mediaMeta.VideoCodec == "h265" {
	// 	pocketizer = n.videoPacketizerH265
	// }
	for {
		// elapsedUs := time.Since(n.timestampStartTime).Microseconds()
		var thisIsVideo bool
		select {
		case <-b.stopCh:
			return
		case box, ok = <-videoCh:
			if !ok {
				videoCh = nil // 防止已关闭的 channel 持续触发空转
			}
			thisIsVideo = true

		case box, ok = <-audioCh:
			if !ok {
				audioCh = nil
			}
		}

		if videoCh == nil && audioCh == nil {
			return // 音视频流均已结束，自动退出
		}
		if !ok {
			continue
		}

		if snap := b.viewersSnap.Load(); snap != nil {
			for _, viewer := range *snap {
				viewer.PushBox(box, thisIsVideo)
			}
		}
	}
}

// 3. 绑定观众到设备（首个人自动成为 Master，后续观众自动触发关键帧）
func (n *Nexus) BindViewSessionToRemote(sessionID, remoteUUID string) error {
	n.Lock()
	v, vOk := n.sessions[sessionID]
	b, rOk := n.bindings[remoteUUID]
	if !vOk || !rOk {
		n.Unlock()
		n.CloseViewSession(sessionID)
		return fmt.Errorf("session or remote not available")
	}

	isFirst := len(b.viewers) == 0
	if isFirst {
		b.masterSessionID = sessionID
	}
	b.viewers[sessionID] = v
	b.refreshSnapshot() // 原子更新快照
	n.sessionToRemote[sessionID] = remoteUUID
	driver := b.remote.sdriver
	n.Unlock()

	// 锁外请求关键帧：新观众中途加入时立刻能看到画面，无需等待下一个 GOP
	driver.RequestIDR(isFirst)
	return nil
}

// 4. 控制指令转发（利用 SDriver.SendEvent）
func (n *Nexus) DispatchEvent(sessionID string, ev sdriver.Event) error {
	n.RLock()
	v, ok := n.sessions[sessionID]
	if !ok || !v.CanControl {
		n.RUnlock()
		return errors.New("unauthorized or session closed")
	}
	remoteUUID, bound := n.sessionToRemote[sessionID]
	b := n.bindings[remoteUUID]
	n.RUnlock()

	if !bound || b == nil {
		return errors.New("remote not bound")
	}
	return b.remote.sdriver.SendEvent(ev)
}

// 5. 统一关闭入口（Detach-then-Close：绝对无死锁，天然幂等）
func (n *Nexus) CloseViewSession(sessionID string) {
	n.Lock()
	v, exists := n.sessions[sessionID]
	if !exists {
		n.Unlock()
		return
	}
	delete(n.sessions, sessionID)

	remoteUUID, bound := n.sessionToRemote[sessionID]
	delete(n.sessionToRemote, sessionID)

	var cascadeSessions []*ViewSession
	var stopBinding *remoteBinding

	if bound {
		if b, ok := n.bindings[remoteUUID]; ok {
			delete(b.viewers, sessionID)
			if b.masterSessionID != sessionID {
				log.Println("View session exit: ", sessionID)
				b.refreshSnapshot() // 原子更新快照
			}

			if b.masterSessionID == sessionID {
				log.Println("Cleaning all view sessions for remote", remoteUUID)
				// Master 断线：整条路由摘除，准备级联清理
				delete(n.bindings, remoteUUID)
				stopBinding = b
				for vid, peer := range b.viewers {
					delete(n.sessions, vid)
					delete(n.sessionToRemote, vid)
					cascadeSessions = append(cascadeSessions, peer)
				}
			}
		}
	}
	n.Unlock()

	// --- 以下全部在锁外执行，彻底免疫回调重入死锁 ---
	v.Close()
	for _, peer := range cascadeSessions {
		peer.Close()
	}
	if stopBinding != nil {
		close(stopBinding.stopCh) // 立即停止 pump 协程
		stopBinding.remote.sdriver.Stop()
	}
}

func (n *Nexus) NewViewSession(userID string, config ConnectionConfig) (*ViewSession, error) {
	var packetizer rtp.Packetizer
	if config.VideoCodec == "h264" {
		packetizer = rtp.NewPacketizer(
			n.mtu, // MTU 大小，通常 1200 左右很安全
			0,     // Payload Type，Pion 会自动覆盖它，填 0 即可
			0,     // SSRC (TrackLocalStaticRTP 会自动覆盖它，填 0 即可)
			&codecs.H264Payloader{},
			rtp.NewRandomSequencer(), // 随机序列号生成器
			90000,                    // 视频的基准时钟频率 (WebRTC 规定视频固定为 90kHz)
		)
	} else if config.VideoCodec == "h265" {
		packetizer = rtp.NewPacketizer(
			n.mtu, // MTU 大小，通常 1200 左右很安全
			0,     // Payload Type，Pion 会自动覆盖它，填 0 即可
			0,     // SSRC (TrackLocalStaticRTP 会自动覆盖它，填 0 即可)
			&codecs.H265Payloader{},
			rtp.NewRandomSequencer(), // 随机序列号生成器
			90000,                    // 视频的基准时钟频率 (WebRTC 规定视频固定为 90kHz)
		)
	}
	sessionID := uuid.New().String()
	viewSession := &ViewSession{
		SessionID:         sessionID,
		UserID:            userID,
		useLocalTimestamp: config.Extra["use_local_timestamp"] == "true",
		videoTSMapper:     newRTPTimestampMapper(90000),
		audioTSMapper:     newRTPTimestampMapper(48000),
		// CanControl:       config.CanControl,
		ConnectionConfig: config,
		videoCh:          make(chan sdriver.AVBox, 15),
		audioCh:          make(chan sdriver.AVBox, 15),
		done:             make(chan struct{}),

		audioPacketizer: rtp.NewPacketizer(n.mtu, 0, 0, &codecs.OpusPayloader{}, rtp.NewRandomSequencer(), 48000),
		videoPacketizer: packetizer,
	}

	err := viewSession.connectToNexus(n, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect view session to Nexus: %v", err)
	}

	n.Lock()
	n.sessions[sessionID] = viewSession
	n.Unlock()

	return viewSession, nil
}

// Implement the logic to connect to the Nexus using the provided configuration.
// This might involve setting up a WebRTC connection, WebSocket, or other protocols based on cfg.Protocol.
// This is NOT going to connect to a remote device directly.
func (v *ViewSession) connectToNexus(n *Nexus, cfg ConnectionConfig) error {
	// if v.NexusConnected {
	// 	log.Println("[webscreen] Viewer is already connected to Nexus")
	// 	return nil
	// }
	switch cfg.Protocol {
	case "webrtc":
		var err error
		videoMimeType, err := getMimeTypeFromString(cfg.VideoCodec)
		if err != nil {
			return err
		}
		sdp := cfg.Extra["sdp"]
		if sdp == "" {
			return errors.New("SDP is required for WebRTC connection")
		}

		peerConnection, err := createPeerConnection(cfg)
		if err != nil {
			return fmt.Errorf("failed to create PeerConnection: %v", err)
		}
		var videoTrack, audioTrack *webrtc.TrackLocalStaticRTP
		// Check if the remote is already connected and has tracks
		n.RLock()
		r, exists := n.bindings[cfg.RemoteUUID]
		n.RUnlock()
		if !exists {
			videoTrack, audioTrack = createAVTrack(videoMimeType, cfg.Extra["av_sync"] == "true")
		} else {
			masterSession := n.sessions[r.masterSessionID]
			videoTrack = masterSession.videoTrack
			audioTrack = masterSession.audioTrack
		}

		// 2. Add SHARED tracks to PeerConnection
		rtpSenderVideo, err := peerConnection.AddTrack(videoTrack)
		if err != nil {
			peerConnection.Close()
			return fmt.Errorf("failed to add video track: %v", err)
		}
		rtpSenderAudio, err := peerConnection.AddTrack(audioTrack)
		if err != nil {
			peerConnection.Close()
			return fmt.Errorf("failed to add audio track: %v", err)
		}

		// Set Remote Description (Offer from browser)
		offer := webrtc.SessionDescription{
			Type: webrtc.SDPTypeOffer,
			SDP:  sdp,
		}
		if err := peerConnection.SetRemoteDescription(offer); err != nil {
			peerConnection.Close()
			return fmt.Errorf("failed to set remote description: %v", err)
		}

		// Create Answer
		answer, err := peerConnection.CreateAnswer(nil)
		if err != nil {
			peerConnection.Close()
			return fmt.Errorf("failed to create answer: %v", err)
		}

		// 设置 Local Description 并等待 ICE 收集完成
		gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

		if err := peerConnection.SetLocalDescription(answer); err != nil {
			peerConnection.Close()
			return fmt.Errorf("failed to set local description: %v", err)
		}

		// 阻塞等待 ICE 收集完成 (通常几百毫秒)
		<-gatherComplete
		v.FinalSDP2Nexus = peerConnection.LocalDescription().SDP
		v.ConnectionConfig = cfg
		v.audioTrack = audioTrack
		v.videoTrack = videoTrack
		v.rtpSenderVideo = rtpSenderVideo
		v.rtpSenderAudio = rtpSenderAudio
		v.peerConnection = peerConnection
		v.peerConnection.OnConnectionStateChange(
			func(state webrtc.PeerConnectionState) {
				switch state {
				case webrtc.PeerConnectionStateDisconnected, webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
					log.Printf("[webscreen] ViewSession %s disconnected from Nexus (state: %s)", v.SessionID, state.String())
					n.CloseViewSession(v.SessionID)
				default:
					log.Printf("[webscreen] ViewSession %s connection state changed: %s", v.SessionID, state.String())
				}
			},
		)
	case "webtransport":
		// Implement WebTransport connection logic here
	case "websocket":
		// Implement WebSocket connection logic here
	default:
		return errors.New("Unsupported protocol: " + cfg.Protocol)
	}
	return nil
}

func (n *Nexus) setupRemote(remoteUUID string, config RemoteConfig) (sdriver.SDriver, error) {
	// Implement the logic to set up a remote device using the provided remoteID
	deviceInfo, err := db.QueryDeviceTypeByUUID(n.db, remoteUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get device type for remoteID %s: %v", remoteUUID, err)
	}
	log.Println("[nexus]remote device info:", deviceInfo)
	// log.Println("[nexus]remote config:", config)
	config["deviceID"] = deviceInfo.DeviceID
	var driver sdriver.SDriver
	switch deviceInfo.Type {
	case consts.DeviceTypeAndroid:
		driver, err = scrcpy.New(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create scrcpy driver for remoteUUID %s: %v", remoteUUID, err)
		}
		// Do something with the created driver, e.g., store it or use it for further processing
	default:
		return nil, fmt.Errorf("unsupported device type: %s", deviceInfo.Type)
	}

	// Do something with the retrieved device type, e.g., store it or use it for further processing
	return driver, nil
}

// 辅助方法：在持有 n.Lock() 时刷新快照
func (b *remoteBinding) refreshSnapshot() {
	snap := make([]*ViewSession, 0, len(b.viewers))
	for _, v := range b.viewers {
		snap = append(snap, v)
	}
	b.viewersSnap.Store(&snap)
}
