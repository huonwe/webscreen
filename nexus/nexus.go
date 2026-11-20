package nexus

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"uuid"
	"webscreen/db"
	"webscreen/sdriver"

	"github.com/pion/interceptor"
	pionSDP "github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v5"
)

type Nexus struct {
	sync.RWMutex
	viewSessions map[string]ViewSession
	remotes      map[string]Remote
	remoteStatus map[string]bool

	db *sql.DB
}

type Remote struct {
	RemoteID string
	// Opened   bool
	sdriver sdriver.SDriver

	viewSession   []ViewSession
	masterSession *ViewSession
}

type ViewSession struct {
	SessionID        string
	UserID           string
	ConnectionConfig ConnectionConfig
	NexusConnected   bool

	RemoteID   string
	CanControl bool
	IsMaster   bool

	// Only used for WebRTC connections, to store the final SDP answer after negotiation.
	finalSDP2Nexus string
	videoTrack     *webrtc.TrackLocalStaticRTP
	audioTrack     *webrtc.TrackLocalStaticRTP
	rtpSenderVideo *webrtc.RTPSender
	rtpSenderAudio *webrtc.RTPSender
	peerConnection *webrtc.PeerConnection

	remote Remote
}

func NewNexus(db *sql.DB) Nexus {
	return Nexus{
		viewSessions: make(map[string]ViewSession),
		remotes:      make(map[string]Remote),
		remoteStatus: make(map[string]bool),
		db:           db,
	}
}

func (n *Nexus) NewViewSession(user string, cfg ConnectionConfig) (ViewSession, error) {
	viewSession := ViewSession{
		SessionID:  uuid.New().String(), // Generate a unique ID for the view session
		UserID:     user,
		CanControl: true,
		RemoteID:   cfg.RemoteID,
	}

	err := viewSession.connectToNexus(n, cfg)
	if err != nil {
		return ViewSession{}, fmt.Errorf("failed to connect view session to Nexus: %v", err)
	}
	n.viewSessions[viewSession.SessionID] = viewSession
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
		v.finalSDP2Nexus = peerConnection.LocalDescription().SDP
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

func (n *Nexus) StartDummyVideo() {}

func (n *Nexus) setupRemote(remoteID string, config RemoteConfig) error {
	// Implement the logic to set up a remote device using the provided remoteID
	deviceType, err := db.DeviceTypeByUUID(n.db, remoteID)
	if err != nil {
		return fmt.Errorf("failed to get device type for remoteID %s: %v", remoteID, err)
	}
	// Do something with the retrieved device type, e.g., store it or use it for further processing
	return nil
}

func (n *Nexus) closeRemote(remoteID string) error {
	// Implement the logic to close the connection to a remote device using the provided remoteID
	return nil
}
