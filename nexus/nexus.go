package nexus

import (
	"errors"
	"fmt"
	"log"
	"scrmux/consts"
	db "scrmux/database"
	"scrmux/sdriver"
	"scrmux/sdriver/gamestream"
	linuxDriver "scrmux/sdriver/linux"
	rockchiphdmirx "scrmux/sdriver/rockchip-hdmirx"
	"scrmux/sdriver/scrcpy"
	"strconv"
	"sync"
	"uuid"

	"github.com/gorilla/websocket"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v5"
)

type Nexus struct {
	sync.RWMutex
	bindings        map[string]*remoteBinding // remoteUUID -> 路由绑定关系
	sessions        map[string]*ViewSession   // sessionID -> ViewSession
	sessionToRemote map[string]string         // sessionID -> remoteUUID（反向索引，方便断线时 O(1) 查找）

	// Timestamp
	// timestampStartTime time.Time

	mtu uint16 // RTP 分包器的 MTU 大小，通常 1200 左右很安全
}

func NewNexus(mtu uint16) *Nexus {
	if mtu == 0 {
		mtu = 1200 // 默认 MTU 大小
	}
	return &Nexus{
		sessions:        make(map[string]*ViewSession),
		bindings:        make(map[string]*remoteBinding),
		sessionToRemote: make(map[string]string),
		mtu:             mtu,
		// timestampStartTime: time.Now(),
	}
}

func (n *Nexus) Close() error {
	n.Lock()
	defer n.Unlock()
	for _, binding := range n.bindings {
		close(binding.stopCh)
		if binding.sdriver != nil {
			binding.sdriver.Stop()
		}
	}
	return nil
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
		RemoteUUID:   remoteUUID,
		RemoteConfig: config,
		sdriver:      driver,
		viewers:      make(map[string]*ViewSession),
		stopCh:       make(chan struct{}),
	}
	n.bindings[remoteUUID] = binding
	n.Unlock()

	// 启动该设备的专属媒体分发泵
	go binding.pumpRemote()
	return nil
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
	driver := b.sdriver
	v.controlEventHandler = b.HandleControlEvent
	n.Unlock()

	// 锁外请求关键帧：新观众中途加入时立刻能看到画面，无需等待下一个 GOP
	driver.RequestIDR(isFirst)
	return nil
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
		stopBinding.sdriver.Stop()
	}
}

func (n *Nexus) NewViewSession(userID string, config ConnectionConfig) (*ViewSession, error) {

	var packetizer rtp.Packetizer
	if config.Protocol == consts.PROTOCOL_TYPE_WEBRTC {
		switch config.VideoCodec {
		case consts.CODEC_TYPE_H264:
			packetizer = rtp.NewPacketizer(
				n.mtu, // MTU 大小，通常 1200 左右很安全
				0,     // Payload Type，Pion 会自动覆盖它，填 0 即可
				0,     // SSRC (TrackLocalStaticRTP 会自动覆盖它，填 0 即可)
				&codecs.H264Payloader{},
				rtp.NewRandomSequencer(), // 随机序列号生成器
				90000,                    // 视频的基准时钟频率 (WebRTC 规定视频固定为 90kHz)
			)
		case consts.CODEC_TYPE_H265:
			packetizer = rtp.NewPacketizer(
				n.mtu, // MTU 大小，通常 1200 左右很安全
				0,     // Payload Type，Pion 会自动覆盖它，填 0 即可
				0,     // SSRC (TrackLocalStaticRTP 会自动覆盖它，填 0 即可)
				&codecs.H265Payloader{},
				rtp.NewRandomSequencer(), // 随机序列号生成器
				90000,                    // 视频的基准时钟频率 (WebRTC 规定视频固定为 90kHz)
			)
		case consts.CODEC_TYPE_AV1:
			packetizer = rtp.NewPacketizer(
				n.mtu, // MTU 大小，通常 1200 左右很安全
				0,     // Payload Type，Pion 会自动覆盖它，填 0 即可
				0,     // SSRC (TrackLocalStaticRTP 会自动覆盖它，填 0 即可)
				&codecs.AV1Payloader{},
				rtp.NewRandomSequencer(), // 随机序列号生成器
				90000,                    // 视频的基准时钟频率 (WebRTC 规定视频固定为 90kHz)
			)
		default:
			return nil, fmt.Errorf("unsupported video codec: %s", config.VideoCodec)
		}
	}
	if config.Protocol == consts.PROTOCOL_TYPE_WEBSOCKET {

	}
	sessionID := uuid.New().String()
	viewSession := &ViewSession{
		SessionID:        sessionID,
		UserID:           userID,
		videoTSMapper:    newVideoTimestampMapper(90000, config.DontTrustRemoteTimestamp),
		audioTSMapper:    newAudioTimestampMapper(),
		CanControl:       true,
		ConnectionConfig: config,
		cacheVideoCh:     make(chan sdriver.AVBox, 30),
		cacheAudioCh:     make(chan sdriver.AVBox, 30),
		cacheMsgCh:       make(chan sdriver.Event, 64),
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

	go viewSession.PushingAVM() // 启动独立协程处理视频/音频缓存队列，避免阻塞 pumpRemote

	return viewSession, nil
}

// Implement the logic to connect to the Nexus using the provided configuration.
// This might involve setting up a WebRTC connection, WebSocket, or other protocols based on cfg.Protocol.
// This is NOT going to connect to a remote device directly.
func (v *ViewSession) connectToNexus(n *Nexus, cfg ConnectionConfig) error {
	// if v.NexusConnected {
	// 	log.Println("[scrmux] Viewer is already connected to Nexus")
	// 	return nil
	// }
	switch cfg.Protocol {
	case consts.PROTOCOL_TYPE_WEBRTC:
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
			n.RLock()
			masterSession := n.sessions[r.masterSessionID]
			n.RUnlock()
			masterSession.MasterSessionLock.Lock()
			videoTrack = masterSession.videoTrack
			audioTrack = masterSession.audioTrack
			if videoTrack == nil || audioTrack == nil {
				// 这说明master session是websocket传输
				videoTrack, audioTrack = createAVTrack(videoMimeType, cfg.Extra["av_sync"] == "true")
				masterSession.videoTrack = videoTrack
				masterSession.audioTrack = audioTrack
			}
			masterSession.MasterSessionLock.Unlock()
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
		// rtpSenderVideo = rtpSenderVideo
		// rtpSenderAudio = rtpSenderAudio
		v.peerConnection = peerConnection
		v.peerConnection.OnConnectionStateChange(
			func(state webrtc.PeerConnectionState) {
				switch state {
				case webrtc.PeerConnectionStateConnected:
					log.Printf("[scrmux] ViewSession %s WebRTC connected, requesting IDR", v.SessionID)
					n.RequestRemoteIDR(v.SessionID)
				case webrtc.PeerConnectionStateDisconnected, webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
					log.Printf("[scrmux] ViewSession %s disconnected from Nexus (state: %s)", v.SessionID, state.String())
					n.CloseViewSession(v.SessionID)
				default:
					log.Printf("[scrmux] ViewSession %s connection state changed: %s", v.SessionID, state.String())
				}
			},
		)
		v.peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
			log.Printf("Have DataChannel: Label '%s'\n", d.Label())
			switch d.Label() {
			case "control-ordered":
				v.dataChannelOrdered = d
				d.OnMessage(func(msg webrtc.DataChannelMessage) {
					if v.controlEventHandler != nil {
						if err := v.controlEventHandler(msg.Data); err != nil {
							log.Printf("Error handling ordered data channel message: %v", err)
						}
					}
				})
			case "control-unordered":
				v.dataChannelUnordered = d
				d.OnMessage(func(msg webrtc.DataChannelMessage) {
					if v.controlEventHandler != nil {
						if err := v.controlEventHandler(msg.Data); err != nil {
							log.Printf("Error handling unordered data channel message: %v", err)
						}
					}
				})
			default:
				log.Printf("Unknown DataChannel label: %s\n", d.Label())
				d.OnMessage(func(msg webrtc.DataChannelMessage) {
					log.Printf("DataChannel '%s' message: %s\n", d.Label(), string(msg.Data))
				})
			}
		})
		go func() {
			for {
				pkts, _, err := rtpSenderVideo.ReadRTCP()
				if err != nil {
					return // PeerConnection 关闭时自动退出
				}
				for _, pkt := range pkts {
					switch pkt.(type) {
					case *rtcp.PictureLossIndication, *rtcp.FullIntraRequest:
						// 浏览器解码丢帧请求关键帧时，自动向设备请求 IDR
						n.RequestRemoteIDR(v.SessionID)
					}
				}
			}
		}()

		// Audio 的 RTCP 同样需要消费（防止缓冲区堆积）
		go func() {
			for {
				if _, _, err := rtpSenderAudio.ReadRTCP(); err != nil {
					return
				}
			}
		}()
	case consts.PROTOCOL_TYPE_WEBTRANSPORT:
		// Implement WebTransport connection logic here
	case consts.PROTOCOL_TYPE_WEBSOCKET:
		v.WSConn = cfg.WSConn
		if v.WSConn == nil {
			return errors.New("WebSocket connection is not established")
		}
		v.WSConn.SetCloseHandler(func(code int, text string) error {
			log.Printf("WebSocket connection closed: %d - %s", code, text)
			n.CloseViewSession(v.SessionID)
			return nil
		})
		// v.WSConn.WriteJSON(map[string]any{"status": "ok", "message": "WebSocket connection established"})
		go func() {
			// time.Sleep(5 * time.Second) // 延迟 1 秒再发送，确保浏览器端的 onmessage 回调已注册
			for {
				// Handle WebSocket messages
				mtype, message, err := v.WSConn.ReadMessage()
				if err != nil {
					log.Printf("Error reading WebSocket message: %v", err)
					n.CloseViewSession(v.SessionID)
					return
				}
				// log.Printf("Received WebSocket message: %s", string(message))
				switch mtype {
				case websocket.TextMessage:
					log.Printf("Text message received: %s", string(message))
				case websocket.BinaryMessage:
					err := v.controlEventHandler(message)
					if err != nil {
						log.Printf("Error handling binary WebSocket message: %v", err)
					}
				}
			}
		}()
	default:
		return errors.New("Unsupported protocol: " + cfg.Protocol)
	}
	return nil
}

func (n *Nexus) setupRemote(remoteUUID string, config RemoteConfig) (sdriver.SDriver, error) {
	// Implement the logic to set up a remote device using the provided remoteID
	deviceInfo, err := db.QueryDeviceTypeByUUID(consts.DB, remoteUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get device type for remoteID %s: %v", remoteUUID, err)
	}
	log.Println("[nexus]remote device info:", deviceInfo)
	// log.Println("[nexus]remote config:", config)
	var driver sdriver.SDriver
	switch deviceInfo.Type {
	case consts.DEVICE_TYPE_ANDROID:
		driver, err = scrcpy.New(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create scrcpy driver for remoteUUID %s: %v", remoteUUID, err)
		}
		// Do something with the created driver, e.g., store it or use it for further processing
	case consts.DEVICE_TYPE_LINUX:
		driver, err = linuxDriver.New(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create linux driver for remoteUUID %s: %v", remoteUUID, err)
		}
	case consts.DEVICE_TYPE_ROCKCHIP_HDMIRX:
		driver, err = rockchiphdmirx.New(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create rockchip-hdmirx driver for remoteUUID %s: %v", remoteUUID, err)
		}
	case consts.DEVICE_TYPE_GAMESTREAM:
		if config["device_ip"] == "" {
			config["device_ip"] = deviceInfo.IP
		}
		if config["device_port"] == "" && deviceInfo.Port > 0 {
			config["device_port"] = strconv.Itoa(deviceInfo.Port)
		}
		driver, err = gamestream.New(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create gamestream driver for remoteUUID %s: %v", remoteUUID, err)
		}
	default:
		return nil, fmt.Errorf("unsupported device type: %s", deviceInfo.Type)
	}

	// Do something with the retrieved device type, e.g., store it or use it for further processing
	return driver, nil
}

func (n *Nexus) RequestRemoteIDR(sessionID string) {
	n.RLock()
	remoteUUID, bound := n.sessionToRemote[sessionID]
	b := n.bindings[remoteUUID]
	n.RUnlock()

	if bound && b != nil && b.sdriver != nil {
		b.sdriver.RequestIDR(false)
	}
}

func (n *Nexus) GetRemoteCapabilities(remoteUUID string) sdriver.DriverCaps {
	n.RLock()
	b, exists := n.bindings[remoteUUID]
	n.RUnlock()

	if !exists || b == nil || b.sdriver == nil {
		return sdriver.DriverCaps{}
	}
	return b.sdriver.Capabilities()
}

func (n *Nexus) GetRemoteMediaMeta(remoteUUID string) sdriver.MediaMeta {
	n.RLock()
	b, exists := n.bindings[remoteUUID]
	n.RUnlock()

	if !exists || b == nil || b.sdriver == nil {
		return sdriver.MediaMeta{}
	}
	return b.sdriver.MediaMeta()
}

// 如果存在master session，返回设备是否正在串流。如果存在master session，同时返回master session的串流配置
func (n *Nexus) IsAlreadyStreaming(remoteUUID string) (bool, RemoteConfig) {
	n.RLock()
	b, exists := n.bindings[remoteUUID]
	n.RUnlock()
	if !exists || b == nil {
		return false, RemoteConfig{}
	}

	if b.masterSessionID != "" && b.sdriver != nil {
		return true, b.RemoteConfig
	}
	return false, RemoteConfig{}
}
