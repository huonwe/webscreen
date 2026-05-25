package scrcpy

import "webscreen/sdriver"

func NewADBTransportFromEnvironment() (ADBTransport, error) {
	local, err := NewLocalADBTransport()
	if err != nil {
		return nil, err
	}
	host, port, ok := remoteADBEndpoint()
	if !ok {
		return local, nil
	}
	return NewRemoteADBTransport(sdriver.BridgeID("bootstrap-env"), host, port, local.ADBExecutable)
}
