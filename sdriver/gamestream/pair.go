package gamestream

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"scrmux/consts"
	"strings"
	"time"
)

const (
	defaultUniqueID   = "0123456789ABCDEF"
	defaultDeviceName = "scrmux"
	defaultUUID       = "12345678-1234-1234-1234-123456789012"
)

type pairXMLRoot struct {
	XMLName           xml.Name `xml:"root"`
	StatusCode        int      `xml:"status_code,attr"`
	StatusMessage     string   `xml:"status_message,attr"`
	Paired            int      `xml:"paired"`
	PlainCert         string   `xml:"plaincert"`
	ChallengeResponse string   `xml:"challengeresponse"`
	PairingSecret     string   `xml:"pairingsecret"`
}

type authStore struct {
	dir        string
	privateKey *rsa.PrivateKey
	clientCert *x509.Certificate
	clientPEM  []byte
}

func loadOrCreateAuth() (*authStore, error) {
	dir := consts.DATA_DIR + "/gamestream_auth"
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create auth dir: %w", err)
	}
	// log.Println("Loading or creating auth in", dir)
	keyPath := filepath.Join(dir, "client_key.pem")
	certPath := filepath.Join(dir, "client_cert.pem")

	store := &authStore{dir: dir}
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		priv, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, fmt.Errorf("generate rsa key: %w", err)
		}
		store.privateKey = priv
		keyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(priv),
		})
		if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
			return nil, fmt.Errorf("write client key: %w", err)
		}

		tmpl := x509.Certificate{
			SerialNumber:          big.NewInt(1),
			Subject:               pkix.Name{CommonName: "NVIDIA GameStream Client"},
			NotBefore:             time.Now().Add(-time.Hour),
			NotAfter:              time.Now().AddDate(20, 0, 0),
			KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			BasicConstraintsValid: true,
		}
		der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
		if err != nil {
			return nil, fmt.Errorf("create client cert: %w", err)
		}
		store.clientPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
		if err := os.WriteFile(certPath, store.clientPEM, 0644); err != nil {
			return nil, fmt.Errorf("write client cert: %w", err)
		}
		store.clientCert, err = x509.ParseCertificate(der)
		if err != nil {
			return nil, fmt.Errorf("parse client cert: %w", err)
		}
		return store, nil
	}

	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read client key: %w", err)
	}
	keyBlock, _ := pem.Decode(keyData)
	if keyBlock == nil {
		return nil, errors.New("invalid client key PEM")
	}
	priv, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse client key: %w", err)
	}
	store.privateKey = priv

	certData, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("read client cert: %w", err)
	}
	certBlock, _ := pem.Decode(certData)
	if certBlock == nil {
		return nil, errors.New("invalid client cert PEM")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse client cert: %w", err)
	}
	store.clientPEM = certData
	store.clientCert = cert
	return store, nil
}

func serverCertPath(ip string) string {
	dir := consts.DATA_DIR + "/gamestream_auth"
	safeIP := strings.ReplaceAll(ip, ":", "_")
	return filepath.Join(dir, fmt.Sprintf("server_cert_%s.pem", safeIP))
}

// IsHostPaired reports whether a pinned server certificate exists locally for ip.
func IsHostPaired(ip string) bool {
	if ip == "" {
		return false
	}
	_, err := os.Stat(serverCertPath(ip))
	return err == nil
}

func (s *authStore) loadServerCert(ip string) (*x509.Certificate, error) {
	data, err := os.ReadFile(serverCertPath(ip))
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid server cert PEM")
	}
	return x509.ParseCertificate(block.Bytes)
}

func (s *authStore) saveServerCert(ip string, cert *x509.Certificate) error {
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	return os.WriteFile(serverCertPath(ip), pemBytes, 0644)
}

func (s *authStore) httpsClient(serverCert *x509.Certificate) (*http.Client, error) {
	tlsCert := tls.Certificate{
		Certificate: [][]byte{s.clientCert.Raw},
		PrivateKey:  s.privateKey,
	}
	tlsCfg := &tls.Config{
		Certificates:       []tls.Certificate{tlsCert},
		InsecureSkipVerify: true, // GameStream server certs do not carry IP SANs; pinned below
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return errors.New("server presented no certificate")
			}
			if !bytes.Equal(rawCerts[0], serverCert.Raw) {
				return errors.New("server certificate mismatch")
			}
			return nil
		},
	}
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
	}, nil
}

// PairDevice performs the NVIDIA GameStream / Sunshine PIN pairing handshake
// and persists the client key/cert and pinned server certificate locally.
func PairDevice(ip string, port int, pin string) error {
	return pairDeviceWithDir(ip, port, pin)
}

func pairDeviceWithDir(ip string, port int, pin string) error {
	if ip == "" {
		return errors.New("gamestream: host ip is required")
	}
	if port <= 0 {
		port = 47989
	}
	if pin == "" {
		return errors.New("gamestream: pairing pin is required")
	}
	store, err := loadOrCreateAuth()
	if err != nil {
		return err
	}
	return store.pair(ip, port, pin)
}

func (s *authStore) pair(ip string, port int, pin string) error {
	httpClient := &http.Client{Timeout: 120 * time.Second}
	doPair := func(query string) (*pairXMLRoot, error) {
		u := fmt.Sprintf("http://%s:%d/pair?uniqueid=%s&uuid=%s&devicename=%s&updateState=1&%s",
			ip, port, defaultUniqueID, defaultUUID, defaultDeviceName, query)
		resp, err := httpClient.Get(u)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		var root pairXMLRoot
		if err := xml.Unmarshal(body, &root); err != nil {
			return nil, fmt.Errorf("parse pair response: %w", err)
		}
		if root.StatusCode != 200 && root.StatusCode != 0 {
			return nil, fmt.Errorf("pair error status %d (%s)", root.StatusCode, root.StatusMessage)
		}
		return &root, nil
	}

	// 1. getservercert (retry once after /unpair if a stale pairing session is stuck on the host)
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	aesHash := sha256.Sum256(append(append([]byte(nil), salt...), []byte(pin)...))
	aesKey := aesHash[:16]

	getServerCertQuery := fmt.Sprintf("phrase=getservercert&salt=%x&clientcert=%s",
		salt, hex.EncodeToString(s.clientPEM))
	resp1, err := doPair(getServerCertQuery)
	if err != nil || resp1.Paired != 1 {
		// Reset stale pairing state on host and retry once
		unpairURL := fmt.Sprintf("http://%s:%d/unpair?uniqueid=%s&uuid=%s", ip, port, defaultUniqueID, defaultUUID)
		if uResp, uErr := httpClient.Get(unpairURL); uErr == nil {
			uResp.Body.Close()
		}
		resp1, err = doPair(getServerCertQuery)
	}
	if err != nil {
		return fmt.Errorf("getservercert: %w", err)
	}
	if resp1.Paired != 1 {
		return errors.New("getservercert: host rejected pairing request")
	}
	serverCert, err := parseServerCertHex(resp1.PlainCert)
	if err != nil {
		return fmt.Errorf("parse server cert: %w", err)
	}

	// 2. clientchallenge
	randomChallenge := make([]byte, 16)
	if _, err := rand.Read(randomChallenge); err != nil {
		return err
	}
	encChallenge, err := aesECBEncrypt(aesKey, randomChallenge)
	if err != nil {
		return err
	}
	resp2, err := doPair(fmt.Sprintf("phrase=clientchallenge&clientchallenge=%x", encChallenge))
	if err != nil {
		return fmt.Errorf("clientchallenge: %w", err)
	}
	if resp2.Paired != 1 {
		return errors.New("clientchallenge failed")
	}
	encResp, err := hex.DecodeString(resp2.ChallengeResponse)
	if err != nil || len(encResp) < 48 {
		return errors.New("clientchallenge: invalid response length")
	}
	decResp, err := aesECBDecrypt(aesKey, encResp)
	if err != nil {
		return err
	}
	serverResponse := decResp[:32]
	serverChallenge := decResp[32:48]

	// 3. serverchallengeresp
	clientSecret := make([]byte, 16)
	if _, err := rand.Read(clientSecret); err != nil {
		return err
	}
	hashInput := make([]byte, 0, len(serverChallenge)+len(s.clientCert.Signature)+len(clientSecret))
	hashInput = append(hashInput, serverChallenge...)
	hashInput = append(hashInput, s.clientCert.Signature...)
	hashInput = append(hashInput, clientSecret...)
	challengeRespHash := sha256.Sum256(hashInput)

	encHash, err := aesECBEncrypt(aesKey, challengeRespHash[:])
	if err != nil {
		return err
	}
	resp3, err := doPair(fmt.Sprintf("phrase=serverchallengeresp&serverchallengeresp=%x", encHash))
	if err != nil {
		return fmt.Errorf("serverchallengeresp: %w", err)
	}
	if resp3.Paired != 1 {
		return errors.New("serverchallengeresp failed")
	}

	// 4. Verify server signature and PIN
	serverSecretResp, err := hex.DecodeString(resp3.PairingSecret)
	if err != nil || len(serverSecretResp) <= 16 {
		return errors.New("invalid pairingsecret from server")
	}
	serverSecret := serverSecretResp[:16]
	serverSig := serverSecretResp[16:]

	rsaPub, ok := serverCert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return errors.New("server cert public key is not RSA")
	}
	hashedSecret := sha256.Sum256(serverSecret)
	if err := rsa.VerifyPKCS1v15(rsaPub, crypto.SHA256, hashedSecret[:], serverSig); err != nil {
		return fmt.Errorf("server signature verification failed: %w", err)
	}

	pinInput := make([]byte, 0, len(randomChallenge)+len(serverCert.Signature)+len(serverSecret))
	pinInput = append(pinInput, randomChallenge...)
	pinInput = append(pinInput, serverCert.Signature...)
	pinInput = append(pinInput, serverSecret...)
	expectedResp := sha256.Sum256(pinInput)
	if !bytes.Equal(expectedResp[:], serverResponse) {
		return errors.New("pairing failed: incorrect PIN")
	}

	// 5. clientpairingsecret
	clientSecretHash := sha256.Sum256(clientSecret)
	clientSig, err := rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, clientSecretHash[:])
	if err != nil {
		return fmt.Errorf("sign client secret: %w", err)
	}
	clientPairingSecret := append(append([]byte(nil), clientSecret...), clientSig...)
	resp4, err := doPair(fmt.Sprintf("phrase=clientpairingsecret&clientpairingsecret=%x", clientPairingSecret))
	if err != nil {
		return fmt.Errorf("clientpairingsecret: %w", err)
	}
	if resp4.Paired != 1 {
		return errors.New("clientpairingsecret rejected")
	}

	// 6. pairchallenge: try HTTP first (matching Sunshine), then HTTPS, then mTLS /serverinfo check
	var errHTTP, errHTTPS error
	if resp5, err := doPair("phrase=pairchallenge"); err == nil && resp5.Paired == 1 {
		return s.saveServerCert(ip, serverCert)
	} else {
		errHTTP = err
		if errHTTP == nil {
			errHTTP = fmt.Errorf("http pairchallenge returned paired=%d", resp5.Paired)
		}
	}

	httpsPort := port - 5
	if info, err := fetchServerInfoHTTP(httpClient, ip, port); err == nil && info.HttpsPort > 0 {
		httpsPort = info.HttpsPort
	}
	if httpsPort <= 0 {
		httpsPort = 47984
	}

	if err := s.pairChallengeHTTPS(ip, httpsPort, serverCert); err == nil {
		return s.saveServerCert(ip, serverCert)
	} else {
		errHTTPS = err
	}

	// Verify via mTLS /serverinfo in case the host already finalized pairing on step 5
	var errMTLS error
	if tlsClient, err := s.httpsClient(serverCert); err == nil {
		u := fmt.Sprintf("https://%s:%d/serverinfo?uniqueid=%s&uuid=%s", ip, httpsPort, defaultUniqueID, defaultUUID)
		if resp, err := tlsClient.Get(u); err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			var info serverInfo
			if uErr := xml.Unmarshal(body, &info); uErr == nil && info.PairStatus == 1 {
				return s.saveServerCert(ip, serverCert)
			} else {
				errMTLS = fmt.Errorf("status=%d pairStatus=%d unmarshalErr=%v", resp.StatusCode, info.PairStatus, uErr)
			}
		} else {
			errMTLS = err
		}
	} else {
		errMTLS = err
	}

	return fmt.Errorf("pairchallenge failed (http: %v; https: %v; mtls_serverinfo: %v)", errHTTP, errHTTPS, errMTLS)
}

func (s *authStore) pairChallengeHTTPS(ip string, httpsPort int, serverCert *x509.Certificate) error {
	client, err := s.httpsClient(serverCert)
	if err != nil {
		return err
	}
	u := fmt.Sprintf("https://%s:%d/pair?uniqueid=%s&uuid=%s&devicename=%s&updateState=1&phrase=pairchallenge",
		ip, httpsPort, defaultUniqueID, defaultUUID, defaultDeviceName)
	resp, err := client.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var root pairXMLRoot
	if err := xml.Unmarshal(body, &root); err != nil {
		return err
	}
	if root.Paired != 1 {
		return fmt.Errorf("https pairchallenge returned paired=%d", root.Paired)
	}
	return nil
}

func parseServerCertHex(hexStr string) (*x509.Certificate, error) {
	raw, err := hex.DecodeString(strings.TrimSpace(hexStr))
	if err != nil || len(raw) == 0 {
		return nil, errors.New("invalid server certificate hex")
	}
	if block, _ := pem.Decode(raw); block != nil && len(block.Bytes) > 0 {
		raw = block.Bytes
	}
	return x509.ParseCertificate(raw)
}

func aesECBEncrypt(key, input []byte) ([]byte, error) {
	if len(input)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("aes-ecb input length %d not multiple of 16", len(input))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(input))
	for i := 0; i < len(input); i += aes.BlockSize {
		block.Encrypt(out[i:i+aes.BlockSize], input[i:i+aes.BlockSize])
	}
	return out, nil
}

func aesECBDecrypt(key, input []byte) ([]byte, error) {
	if len(input)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("aes-ecb input length %d not multiple of 16", len(input))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(input))
	for i := 0; i < len(input); i += aes.BlockSize {
		block.Decrypt(out[i:i+aes.BlockSize], input[i:i+aes.BlockSize])
	}
	return out, nil
}
