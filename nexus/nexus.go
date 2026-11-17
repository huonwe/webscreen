package nexus

import (
	"errors"
	"uuid"
	"webscreen/sdriver"
)

type Nexus struct {
	viewers      map[string]Viewer
	remotes      map[string]Remote
	remoteStatus map[string]bool
}

type Remote struct {
	ID      string
	Status  bool
	sdriver sdriver.SDriver

	viewers []Viewer
	master  *Viewer
}

type Viewer struct {
	ID string

	remotes []Remote
}

func (n *Nexus) NewViewer() Viewer {
	viewer := Viewer{
		ID:      uuid.New().String(),
		remotes: make([]Remote, 0),
	}
	n.viewers[viewer.ID] = viewer
	return viewer
}

// Implement the logic to connect to the Nexus using the provided configuration.
// This might involve setting up a WebRTC connection, WebSocket, or other protocols based on cfg.Protocol.
// This is NOT going to connect to a remote device directly.
func (u *Viewer) ConnectToNexus(n *Nexus, cfg ConnectionConfig) error {
	switch cfg.Protocol {
	case "webrtc":
		// Implement WebRTC connection logic here
	case "webtransport":
		// Implement WebTransport connection logic here
	case "websocket":
		// Implement WebSocket connection logic here
		return errors.New("WebSocket connection not implemented yet")
	default:
		return errors.New("Unsupported protocol: " + cfg.Protocol)
	}
	return nil
}

func (n *Nexus) StartDummyVideo() {}

func (n *Nexus) setupRemote(remoteID string, config RemoteConfig) error {
	// Implement the logic to set up a remote device using the provided remoteID
	return nil
}

func (n *Nexus) closeRemote(remoteID string) error {
	// Implement the logic to close the connection to a remote device using the provided remoteID
	return nil
}
