package gamestream

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type serverInfo struct {
	AppVersion             string `xml:"appversion"`
	GfeVersion             string `xml:"GfeVersion"`
	HttpsPort              int    `xml:"HttpsPort"`
	PairStatus             int    `xml:"PairStatus"`
	CurrentGame            int    `xml:"currentgame"`
	ServerCodecModeSupport int    `xml:"ServerCodecModeSupport"`
	Hostname               string `xml:"hostname"`
}

type appListXML struct {
	XMLName xml.Name `xml:"root"`
	Apps    []appXML `xml:"App"`
}

type appXML struct {
	ID    int    `xml:"ID"`
	Title string `xml:"AppTitle"`
}

type launchXML struct {
	XMLName     xml.Name `xml:"root"`
	StatusCode  int      `xml:"status_code,attr"`
	StatusMsg   string   `xml:"status_message,attr"`
	GameSession int      `xml:"gamesession"`
	SessionURL0 string   `xml:"sessionUrl0"`
	Resume      int      `xml:"resume"`
}

type httpSession struct {
	ip         string
	httpPort   int
	httpsPort  int
	appVersion [4]int
	isSunshine bool
	client     *http.Client
	rikey      [16]byte
	rikeyID    uint32
	sessionURL string
}

func newHTTPSession(cfg *streamConfig, store *authStore) (*httpSession, error) {
	httpClient := &http.Client{Timeout: 10 * time.Second}
	info, err := fetchServerInfoHTTP(httpClient, cfg.ip, cfg.port)
	if err != nil {
		return nil, fmt.Errorf("gamestream: serverinfo failed: %w", err)
	}

	if info.PairStatus != 1 {
		if cfg.pin == "" {
			return nil, fmt.Errorf("gamestream: host %s is not paired (pair via console or provide pin)", cfg.ip)
		}
		if err := store.pair(cfg.ip, cfg.port, cfg.pin); err != nil {
			return nil, fmt.Errorf("gamestream: pairing failed: %w", err)
		}
		info, err = fetchServerInfoHTTP(httpClient, cfg.ip, cfg.port)
		if err != nil {
			return nil, fmt.Errorf("gamestream: serverinfo after pair failed: %w", err)
		}
	}

	serverCert, err := store.loadServerCert(cfg.ip)
	if err != nil {
		if cfg.pin == "" {
			return nil, fmt.Errorf("gamestream: missing pinned server cert for %s (re-pair required): %w", cfg.ip, err)
		}
		if err := store.pair(cfg.ip, cfg.port, cfg.pin); err != nil {
			return nil, fmt.Errorf("gamestream: re-pair failed: %w", err)
		}
		serverCert, err = store.loadServerCert(cfg.ip)
		if err != nil {
			return nil, err
		}
	}

	httpsPort := info.HttpsPort
	if httpsPort <= 0 {
		httpsPort = cfg.port - 5
		if httpsPort <= 0 {
			httpsPort = 47984
		}
	}

	tlsClient, err := store.httpsClient(serverCert)
	if err != nil {
		return nil, err
	}

	hs := &httpSession{
		ip:         cfg.ip,
		httpPort:   cfg.port,
		httpsPort:  httpsPort,
		appVersion: parseAppVersion(info.AppVersion),
		isSunshine: strings.Contains(strings.ToLower(info.GfeVersion), "sunshine") || info.GfeVersion == "" || info.AppVersion == "",
		client:     tlsClient,
	}

	// Verify mTLS /serverinfo and refresh currentgame
	if secInfo, err := hs.getServerInfoHTTPS(); err == nil {
		if secInfo.AppVersion != "" {
			hs.appVersion = parseAppVersion(secInfo.AppVersion)
		}
		info.CurrentGame = secInfo.CurrentGame
	}

	if _, err := rand.Read(hs.rikey[:]); err != nil {
		return nil, err
	}
	var idBytes [4]byte
	if _, err := rand.Read(idBytes[:]); err != nil {
		return nil, err
	}
	hs.rikeyID = binary.BigEndian.Uint32(idBytes[:])

	appID, err := hs.resolveAppID(cfg.app)
	if err != nil {
		return nil, err
	}

	if err := hs.launchOrResume(cfg, appID, info.CurrentGame); err != nil {
		return nil, err
	}
	return hs, nil
}

func fetchServerInfoHTTP(client *http.Client, ip string, port int) (*serverInfo, error) {
	u := fmt.Sprintf("http://%s:%d/serverinfo?uniqueid=%s&uuid=%s", ip, port, defaultUniqueID, defaultUUID)
	resp, err := client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var info serverInfo
	if err := xml.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (h *httpSession) getServerInfoHTTPS() (*serverInfo, error) {
	u := fmt.Sprintf("https://%s:%d/serverinfo?uniqueid=%s&uuid=%s", h.ip, h.httpsPort, defaultUniqueID, defaultUUID)
	resp, err := h.client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var info serverInfo
	if err := xml.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (h *httpSession) resolveAppID(app string) (int, error) {
	if id, err := strconv.Atoi(app); err == nil && id > 0 {
		return id, nil
	}
	u := fmt.Sprintf("https://%s:%d/applist?uniqueid=%s&uuid=%s", h.ip, h.httpsPort, defaultUniqueID, defaultUUID)
	resp, err := h.client.Get(u)
	if err != nil {
		return 0, fmt.Errorf("gamestream: applist failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	var list appListXML
	if err := xml.Unmarshal(body, &list); err != nil {
		return 0, fmt.Errorf("gamestream: parse applist: %w", err)
	}
	if len(list.Apps) == 0 {
		return 0, fmt.Errorf("gamestream: host returned empty applist")
	}
	target := strings.ToLower(strings.TrimSpace(app))
	for _, a := range list.Apps {
		if strings.ToLower(strings.TrimSpace(a.Title)) == target {
			return a.ID, nil
		}
	}
	for _, a := range list.Apps {
		if strings.Contains(strings.ToLower(a.Title), target) {
			return a.ID, nil
		}
	}
	return list.Apps[0].ID, nil
}

func (h *httpSession) launchOrResume(cfg *streamConfig, appID int, currentGame int) error {
	sops := 0
	if cfg.sops {
		sops = 1
	}
	localAudio := 0
	if cfg.hostAudio {
		localAudio = 1
	}
	rikeyHex := hex.EncodeToString(h.rikey[:])
	mode := fmt.Sprintf("%dx%dx%d", cfg.width, cfg.height, cfg.fps)

	action := "launch"
	if currentGame == appID {
		action = "resume"
	} else if currentGame != 0 {
		// Cancel other running app before launching requested app
		_ = h.cancel()
	}

	params := url.Values{}
	params.Set("uniqueid", defaultUniqueID)
	params.Set("uuid", defaultUUID)
	params.Set("appid", strconv.Itoa(appID))
	params.Set("mode", mode)
	params.Set("additionalStates", "1")
	params.Set("sops", strconv.Itoa(sops))
	params.Set("rikey", rikeyHex)
	params.Set("rikeyid", strconv.FormatUint(uint64(h.rikeyID), 10))
	params.Set("localAudioPlayMode", strconv.Itoa(localAudio))
	params.Set("surroundAudioInfo", "196610") // stereo (2 ch, mask 0x3 -> (3<<16)|2 = 196610)
	params.Set("remoteControllersBitmask", "0")
	params.Set("gcmap", "0")
	params.Set("gcpersist", "0")

	u := fmt.Sprintf("https://%s:%d/%s?%s", h.ip, h.httpsPort, action, params.Encode())
	resp, err := h.client.Get(u)
	if err != nil {
		return fmt.Errorf("gamestream: %s failed: %w", action, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var res launchXML
	if err := xml.Unmarshal(body, &res); err != nil {
		return fmt.Errorf("gamestream: parse %s response: %w", action, err)
	}
	if res.StatusCode != 200 && res.StatusCode != 0 {
		return fmt.Errorf("gamestream: %s error %d: %s", action, res.StatusCode, res.StatusMsg)
	}
	if res.GameSession == 0 && res.Resume == 0 && res.SessionURL0 == "" {
		return fmt.Errorf("gamestream: %s returned no session", action)
	}
	h.sessionURL = res.SessionURL0
	if h.sessionURL == "" {
		h.sessionURL = fmt.Sprintf("rtsp://%s:48010", h.ip)
	}
	return nil
}

func (h *httpSession) cancel() error {
	if h == nil || h.client == nil {
		return nil
	}
	u := fmt.Sprintf("https://%s:%d/cancel?uniqueid=%s&uuid=%s", h.ip, h.httpsPort, defaultUniqueID, defaultUUID)
	resp, err := h.client.Get(u)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func parseAppVersion(s string) [4]int {
	var v [4]int
	parts := strings.Split(strings.TrimSpace(s), ".")
	for i := 0; i < len(parts) && i < 4; i++ {
		n, err := strconv.Atoi(parts[i])
		if err == nil {
			v[i] = n
		}
	}
	if v[0] == 0 {
		v = [4]int{7, 1, 431, 0}
	}
	return v
}

func (h *httpSession) appVersionAtLeast(major, minor, patch int) bool {
	if h.appVersion[0] != major {
		return h.appVersion[0] > major
	}
	if h.appVersion[1] != minor {
		return h.appVersion[1] > minor
	}
	return h.appVersion[2] >= patch
}
