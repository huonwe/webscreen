package nexus

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"uuid"
	"webscreen/consts"
	"webscreen/db"
	"webscreen/sdriver"
	"webscreen/sdriver/scrcpy"

	"github.com/pion/interceptor"
	pionSDP "github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v5"
)

type Nexus struct {
	sync.RWMutex
	viewSessions map[string]*ViewSession
	remotes      map[string]*Remote

	db *sql.DB
}

type Remote struct {
	sync.RWMutex
	RemoteUUID string
	// Opened   bool
	sdriver sdriver.SDriver

	viewSession   []*ViewSession
	masterSession *ViewSession
}

type ViewSession struct {
	SessionID        string
	UserID           string
	ConnectionConfig ConnectionConfig
	NexusConnected   bool

	RemoteUUID string
	CanControl bool
	IsMaster   bool

	// Only used for WebRTC connections, to store the final SDP answer after negotiation.
	FinalSDP2Nexus string
	videoTrack     *webrtc.TrackLocalStaticRTP
	audioTrack     *webrtc.TrackLocalStaticRTP
	rtpSenderVideo *webrtc.RTPSender
	rtpSenderAudio *webrtc.RTPSender
	peerConnection *webrtc.PeerConnection

	remote *Remote
}

func NewNexus(db *sql.DB) Nexus {
	return Nexus{
		viewSessions: make(map[string]*ViewSession),
		remotes:      make(map[string]*Remote),
		db:           db,
	}
}

func (n *Nexus) NewViewSession(user string, cfg ConnectionConfig) (*ViewSession, error) {
	viewSession := ViewSession{
		SessionID:  uuid.New().String(), // Generate a unique ID for the view session
		UserID:     user,
		CanControl: true,
		RemoteUUID: cfg.RemoteUUID,
	}

	err := viewSession.connectToNexus(n, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect view session to Nexus: %v", err)
	}
	n.viewSessions[viewSession.SessionID] = &viewSession
	return &viewSession, nil
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
		if cfg.VideoCodec != "h264" && cfg.VideoCodec != "h265" {
			return errors.New("WebRTC: Unsupported video codec: " + cfg.VideoCodec)
		}
		sdp := cfg.Extra["sdp"]
		if sdp == "" {
			return errors.New("SDP is required for WebRTC connection")
		}
		sdpOffersH265 := func(sdp string) bool {
			lower := strings.ToLower(sdp)
			return strings.Contains(lower, "h265") || strings.Contains(lower, "hevc")
		}

		if strings.EqualFold(cfg.VideoCodec, "h265") && !sdpOffersH265(sdp) {
			log.Printf("[webscreen] browser SDP does not offer H.265 (HEVC)")
			return errors.New("This browser does not support H.265 (HEVC); streaming with H.264 instead.")
		}

		videoMimeType, err := getMimeTypeFromString(cfg.VideoCodec)
		if err != nil {
			return err
		}
		audioMimeType, err := getMimeTypeFromString(cfg.AudioCodec)
		if err != nil {
			return err
		}
		if audioMimeType != "audio/opus" {
			return errors.New("WebRTC: Unsupported audio codec: " + cfg.AudioCodec)
		}
		mimeTypes := []string{videoMimeType, audioMimeType}
		m := createMediaEngine(mimeTypes)
		err = m.RegisterHeaderExtension(
			webrtc.RTPHeaderExtensionCapability{URI: pionSDP.TransportCCURI},
			webrtc.RTPCodecTypeVideo,
		)
		if err != nil {
			return fmt.Errorf("failed to register header extension: %v", err)
		}
		i := &interceptor.Registry{}
		if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
			return fmt.Errorf("failed to register default interceptors: %v", err)
		}
		settingEngine := webrtc.SettingEngine{}
		err = settingEngine.SetEphemeralUDPPortRange(UDP_PORT_START, UDP_PORT_END)
		if err != nil {
			return fmt.Errorf("failed to set ephemeral UDP port range: %v", err)
		}
		api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i), webrtc.WithSettingEngine(settingEngine))
		// Configure ICE servers (STUN) for NAT traversal
		config := webrtc.Configuration{
			ICEServers: []webrtc.ICEServer{
				{URLs: []string{"stun:stun.l.google.com:19302"}},
			},
		}
		// Create PeerConnection
		peerConnection, err := api.NewPeerConnection(config)
		if err != nil {
			return fmt.Errorf("failed to create PeerConnection: %v", err)
		}
		videoTrack, audioTrack := createAVTrack(videoMimeType, audioMimeType, cfg.Extra["av_sync"] == "true")
		// 2. Add SHARED tracks to PeerConnection
		rtpSenderVideo, err := peerConnection.AddTrack(videoTrack)
		if err != nil {
			return fmt.Errorf("failed to add video track: %v", err)
		}
		rtpSenderAudio, err := peerConnection.AddTrack(audioTrack)
		if err != nil {
			return fmt.Errorf("failed to add audio track: %v", err)
		}

		// Set Remote Description (Offer from browser)
		offer := webrtc.SessionDescription{
			Type: webrtc.SDPTypeOffer,
			SDP:  sdp,
		}
		if err := peerConnection.SetRemoteDescription(offer); err != nil {
			return fmt.Errorf("failed to set remote description: %v", err)
		}

		// Create Answer
		answer, err := peerConnection.CreateAnswer(nil)
		if err != nil {
			return fmt.Errorf("failed to create answer: %v", err)
		}

		// 设置 Local Description 并等待 ICE 收集完成
		gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

		if err := peerConnection.SetLocalDescription(answer); err != nil {
			return fmt.Errorf("failed to set local description: %v", err)
		}

		// 阻塞等待 ICE 收集完成 (通常几百毫秒)
		<-gatherComplete
		v.FinalSDP2Nexus = peerConnection.LocalDescription().SDP
		v.NexusConnected = true
		v.ConnectionConfig = cfg
		v.audioTrack = audioTrack
		v.videoTrack = videoTrack
		v.rtpSenderVideo = rtpSenderVideo
		v.rtpSenderAudio = rtpSenderAudio
		v.peerConnection = peerConnection
	case "webtransport":
		// Implement WebTransport connection logic here
	case "websocket":
		// Implement WebSocket connection logic here
	default:
		return errors.New("Unsupported protocol: " + cfg.Protocol)
	}
	return nil
}

func (n *Nexus) BindViewSessionToRemote(v *ViewSession, remoteUUID string) error {
	if _, exists := n.remotes[remoteUUID]; !exists {
		return fmt.Errorf("remote with UUID %s is not connected to the nexus yet", remoteUUID)
	}
	v.remote = n.remotes[remoteUUID]
	if v.remote.masterSession == nil {
		v.remote.masterSession = v
		v.IsMaster = true
	}
	log.Println("session Info: ", v.IsMaster, v.SessionID, v.UserID, v.ConnectionConfig.Protocol, v.ConnectionConfig.VideoCodec, v.ConnectionConfig.AudioCodec)
	switch v.ConnectionConfig.Protocol {
	case "webrtc":
		v.peerConnection.OnConnectionStateChange(
			func(state webrtc.PeerConnectionState) {
				switch state {
				case webrtc.PeerConnectionStateDisconnected, webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
					log.Printf("[webscreen] ViewSession %s disconnected from Nexus (state: %s)", v.SessionID, state.String())
					n.closeViewSession(v.SessionID)
				default:
					log.Printf("[webscreen] ViewSession %s connection state changed: %s", v.SessionID, state.String())
				}
			},
		)
	}
	v.remote.viewSession = append(v.remote.viewSession, v)
	return nil
}

func (n *Nexus) ConnectToRemote(remoteUUID string, config RemoteConfig) error {
	if _, exists := n.remotes[remoteUUID]; exists {
		log.Printf("[webscreen] Remote with UUID %s is already connected", remoteUUID)
		return nil
	}
	driver, err := n.setupRemote(remoteUUID, config)
	if err != nil {
		return fmt.Errorf("failed to set up remote: %v", err)
	}
	err = driver.Start()
	if err != nil {
		return fmt.Errorf("failed to start driver for remoteUUID %s: %v", remoteUUID, err)
	}
	n.remotes[remoteUUID] = &Remote{
		RemoteUUID: remoteUUID,
		sdriver:    driver,
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

func (n *Nexus) closeRemote(remoteUUID string) {
	// Implement the logic to close the connection to a remote device using the provided remoteID
	n.Lock()
	remote, exists := n.remotes[remoteUUID]
	log.Printf("[webscreen] Closing remote with UUID %s, exists: %v", remoteUUID, exists)
	if !exists {
		log.Printf("remote with UUID %s does not exist", remoteUUID)
		n.Unlock()
		return
	}
	log.Println("Closing remote sdriver: ", remote.sdriver)
	if remote.sdriver != nil {
		log.Printf("[webscreen] Stopping scrcpy driver for remote with UUID %s", remoteUUID)
		remote.sdriver.Stop()
	}
	delete(n.remotes, remoteUUID)
	log.Printf("[webscreen] Remote with UUID %s has been closed and removed from Nexus", remoteUUID)
	n.Unlock()
	for _, v := range remote.viewSession {
		n.closeViewSession(v.SessionID)
	}
}

func (n *Nexus) closeViewSession(sessionID string) {
	n.Lock()
	_, exists := n.viewSessions[sessionID]
	if !exists {
		log.Printf("view session with ID %s does not exist", sessionID)
		n.Unlock()
		return
	}
	// viewSession.Close(n)
	// if !viewSession.NexusConnected {
	// 	return nil
	// }
	log.Println("Closing view session:", n.viewSessions[sessionID].SessionID, n.viewSessions[sessionID].UserID, n.viewSessions[sessionID].ConnectionConfig.Protocol)
	switch n.viewSessions[sessionID].ConnectionConfig.Protocol {
	case "webrtc":
		if n.viewSessions[sessionID].peerConnection != nil {
			n.viewSessions[sessionID].peerConnection.Close()
		}
	case "webtransport":
		// Implement WebTransport disconnection logic here
	case "websocket":
		// Implement WebSocket disconnection logic here
	default:
		log.Printf("[webscreen] Closing Unknown protocol: %s", n.viewSessions[sessionID].ConnectionConfig.Protocol)
	}
	// n.viewSessions[viewSession.SessionID] = nil
	isMaster := n.viewSessions[sessionID].IsMaster
	remoteUUID := n.viewSessions[sessionID].RemoteUUID
	delete(n.viewSessions, sessionID)
	n.Unlock()

	if isMaster {
		log.Printf("[webscreen] Master view session %s is closing, closing remote %s", sessionID, remoteUUID)
		n.closeRemote(remoteUUID)
	}

}

// func (v *ViewSession) Close(n *Nexus) {

// }
