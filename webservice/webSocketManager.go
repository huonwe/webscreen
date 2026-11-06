package webservice

import (
	"fmt"
	"log"
	"sync"
	sagent "webscreen/streamAgent"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

type WSSubscriber struct {
	Conn *websocket.Conn
}

type WSDeviceBroadcaster struct {
	Agent          *sagent.Agent
	AgentConfigKey string
	Subscribers    map[uint32]*WSSubscriber
	Lock           sync.RWMutex
}

type WebSocketManager struct {
	sync.RWMutex
	broadcasters         map[string]*WSDeviceBroadcaster
	currentReceiptNumber map[string]uint32
}

func NewWebSocketManager() *WebSocketManager {
	return &WebSocketManager{
		broadcasters:         make(map[string]*WSDeviceBroadcaster),
		currentReceiptNumber: make(map[string]uint32),
	}
}

func (manager *WebSocketManager) NewSubscriber(deviceIdentifier string, conn *websocket.Conn, agentConfig sagent.AgentConfig) (uint32, error) {
	manager.Lock()
	broadcaster, exists := manager.broadcasters[deviceIdentifier]
	if !exists {
		broadcaster = &WSDeviceBroadcaster{
			Subscribers: make(map[uint32]*WSSubscriber),
		}
		manager.broadcasters[deviceIdentifier] = broadcaster
	}

	receiptNo := manager.currentReceiptNumber[deviceIdentifier]
	broadcaster.Lock.Lock()
	if broadcaster.Subscribers[receiptNo] != nil {
		log.Printf("Warning: Overwriting existing subscriber for device %s, receiptNo %d", deviceIdentifier, receiptNo)
		broadcaster.Subscribers[receiptNo].Conn.Close()
	}
	sub := &WSSubscriber{
		Conn: conn,
	}
	broadcaster.Subscribers[receiptNo] = sub
	broadcaster.Lock.Unlock()

	manager.currentReceiptNumber[deviceIdentifier] = (manager.currentReceiptNumber[deviceIdentifier] + 1) % MAX_CLIENTS_PER_DEVICE
	manager.Unlock()

	manager.setCleanup(conn, deviceIdentifier, receiptNo)

	return receiptNo, nil
}

func (manager *WebSocketManager) Start(deviceIdentifier string, receiptNo uint32, agentConfig sagent.AgentConfig) error {
	err := manager.ensureAgent(deviceIdentifier, receiptNo, agentConfig)
	if err != nil {
		return fmt.Errorf("failed to ensure agent: %v", err)
	}
	return nil
}

func (manager *WebSocketManager) ensureAgent(deviceIdentifier string, receiptNo uint32, agentConfig sagent.AgentConfig) error {
	manager.Lock()
	broadcaster, exists := manager.broadcasters[deviceIdentifier]
	if !exists {
		manager.Unlock()
		return fmt.Errorf("broadcaster should exist at this point")
	}

	configKey := agentConfigKey(agentConfig)
	if broadcaster.Agent != nil && (!broadcaster.Agent.Alive() || broadcaster.AgentConfigKey != configKey) {
		log.Printf("[websocket] rebuilding agent for device %s (alive=%v configChanged=%v)",
			deviceIdentifier, broadcaster.Agent.Alive(), broadcaster.AgentConfigKey != configKey)
		broadcaster.Agent.Close()
		broadcaster.Agent = nil
	}

	if broadcaster.Agent == nil {
		// Pass nil to videoTrack and audioTrack as they are not used for pure websocket streaming.
		// Note: ensure streamAgent does not crash if these are nil, or handles it appropriately.
		agent := sagent.New(agentConfig, nil, nil)

		// InitDriver requires a dummy codec mapping since we are not gathering ICE candidates
		dummyCodec := webrtc.RTPCodecParameters{
			RTPCodecCapability: webrtc.RTPCodecCapability{
				MimeType: webrtc.MimeTypeH264,
			},
			PayloadType: PAYLOAD_TYPE_H264_PROFILE_HIGH_5_1,
		}

		err := agent.InitDriver(dummyCodec)
		if err != nil {
			manager.Unlock()
			return fmt.Errorf("failed to init driver: %v", err)
		}

		// Double check concurrency
		if broadcaster.Agent != nil {
			manager.Unlock()
			return nil
		}

		agent.OnVideoFrame = func(data []byte) {
			manager.BroadcastVideo(deviceIdentifier, data)
		}

		broadcaster.Agent = agent
		broadcaster.AgentConfigKey = configKey
		go agent.Start()

		go func() {
			for event := range agent.FeedbackEvents() {
				broadcaster.Lock.RLock()
				for _, sub := range broadcaster.Subscribers {
					if sub.Conn != nil {
						sub.Conn.WriteMessage(websocket.TextMessage, event)
					}
				}
				broadcaster.Lock.RUnlock()
			}
		}()
	}
	manager.Unlock()

	return nil
}

func (manager *WebSocketManager) GetAgent(deviceIdentifier string) (*sagent.Agent, bool) {
	manager.RLock()
	defer manager.RUnlock()
	b, exists := manager.broadcasters[deviceIdentifier]
	if !exists || b.Agent == nil {
		return nil, false
	}
	return b.Agent, true
}

func (manager *WebSocketManager) GetSubscriber(deviceIdentifier string, receiptNo uint32) (*WSSubscriber, bool) {
	manager.RLock()
	defer manager.RUnlock()
	b, exists := manager.broadcasters[deviceIdentifier]
	if !exists {
		return nil, false
	}
	b.Lock.RLock()
	defer b.Lock.RUnlock()
	sub, exists := b.Subscribers[receiptNo]
	return sub, exists
}

func (manager *WebSocketManager) setCleanup(conn *websocket.Conn, deviceIdentifier string, receiptNo uint32) {
	// Usually the reading loop will detect close and clean up.
	// WSSubscriber cleanup can be invoked here or manually externally.
}

// BroadcastVideo writes raw video frames to all WS subscribers
func (manager *WebSocketManager) BroadcastVideo(deviceIdentifier string, data []byte) {
	manager.RLock()
	broadcaster, exists := manager.broadcasters[deviceIdentifier]
	manager.RUnlock()

	if !exists {
		return
	}

	broadcaster.Lock.RLock()
	for receiptNo, sub := range broadcaster.Subscribers {
		if sub.Conn != nil {
			err := sub.Conn.WriteMessage(websocket.BinaryMessage, data)
			if err != nil {
				log.Printf("Failed to write video to subscriber %d: %v", receiptNo, err)
			}
		}
	}
	broadcaster.Lock.RUnlock()
}
