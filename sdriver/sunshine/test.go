package sunshine

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	SunshineIP   = "192.168.0.61" // 请修改为你的 Sunshine IP
	SunshinePort = "47989"        // 配对通常使用 HTTP 47989 端口
	DeviceName   = "GoClient"     // 设备名称
	AuthDir      = "sunshine_auth"
)

// 解析服务器返回的 XML
type Root struct {
	XMLName           xml.Name `xml:"root"`
	StatusCode        int      `xml:"status_code,attr"`
	Paired            int      `xml:"paired"`
	PlainCert         string   `xml:"plaincert"`
	ChallengeResponse string   `xml:"challengeresponse"`
	PairingSecret     string   `xml:"pairingsecret"`
}

type PairingManager struct {
	IP         string
	Port       string
	DeviceName string
	PrivateKey *rsa.PrivateKey
	ClientCert *x509.Certificate
	ClientPEM  []byte
	ServerCert *x509.Certificate
}

// AES ECB 加密 (Java 中的 AESLightEngine 默认行为)
func ecbEncrypt(key, input []byte) []byte {
	block, _ := aes.NewCipher(key)
	out := make([]byte, len(input))
	for i := 0; i < len(input); i += 16 {
		block.Encrypt(out[i:i+16], input[i:i+16])
	}
	return out
}

// AES ECB 解密
func ecbDecrypt(key, input []byte) []byte {
	block, _ := aes.NewCipher(key)
	out := make([]byte, len(input))
	for i := 0; i < len(input); i += 16 {
		block.Decrypt(out[i:i+16], input[i:i+16])
	}
	return out
}

// HTTP 请求辅助函数
func (pm *PairingManager) doPairReq(query string) (*Root, error) {
	// 组合请求 URL, 固定 uniqueid 并在请求中包含 phrase
	url := fmt.Sprintf("http://%s:%s/pair?uniqueid=0123456789ABCDEF&uuid=12345678-1234-1234-1234-123456789012&devicename=%s&updateState=1&%s",
		pm.IP, pm.Port, pm.DeviceName, query)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var root Root
	if err := xml.Unmarshal(body, &root); err != nil {
		return nil, err
	}
	if root.StatusCode != 200 && root.StatusCode != 0 {
		return nil, fmt.Errorf("服务器返回错误状态码: %d", root.StatusCode)
	}
	return &root, nil
}

// 执行配对流程
func (pm *PairingManager) Pair(pin string) error {
	fmt.Println("开始与 Sunshine 进行配对...")

	// 1. 获取服务器证书 (getservercert)
	salt := make([]byte, 16)
	rand.Read(salt)

	// 生成 AES Key: SHA256(salt + PIN) 取前 16 字节
	saltPin := append(salt, []byte(pin)...)
	aesKeyHash := sha256.Sum256(saltPin)
	aesKey := aesKeyHash[:16]

	clientCertHex := hex.EncodeToString(pm.ClientPEM)
	req1 := fmt.Sprintf("phrase=getservercert&salt=%x&clientcert=%s", salt, clientCertHex)

	resp1, err := pm.doPairReq(req1)
	if err != nil || resp1.Paired != 1 {
		return fmt.Errorf("步骤1失败: 无法获取服务端证书或配对状态异常")
	}

	serverCertBytes, _ := hex.DecodeString(resp1.PlainCert)
	pm.ServerCert, err = x509.ParseCertificate(serverCertBytes)
	if err != nil {
		return fmt.Errorf("解析服务端证书失败: %v", err)
	}
	fmt.Println("成功获取服务端证书.")

	// 2. 客户端挑战 (clientchallenge)
	randomChallenge := make([]byte, 16)
	rand.Read(randomChallenge)
	encChallenge := ecbEncrypt(aesKey, randomChallenge)

	req2 := fmt.Sprintf("phrase=clientchallenge&clientchallenge=%x", encChallenge)
	resp2, err := pm.doPairReq(req2)
	if err != nil || resp2.Paired != 1 {
		return fmt.Errorf("步骤2失败: 客户端挑战未通过")
	}

	encResp, _ := hex.DecodeString(resp2.ChallengeResponse)
	decResp := ecbDecrypt(aesKey, encResp)

	// 前 32 字节为 serverResponse (SHA256 哈希长度)，后 16 字节为 serverChallenge
	serverResponse := decResp[:32]
	serverChallenge := decResp[32:48]

	// 3. 服务端挑战响应 (serverchallengeresp)
	clientSecret := make([]byte, 16)
	rand.Read(clientSecret)

	// 计算哈希: SHA256(serverChallenge + clientCertSignature + clientSecret)
	hashInput := append(serverChallenge, pm.ClientCert.Signature...)
	hashInput = append(hashInput, clientSecret...)
	challengeRespHash := sha256.Sum256(hashInput)

	encHash := ecbEncrypt(aesKey, challengeRespHash[:])
	req3 := fmt.Sprintf("phrase=serverchallengeresp&serverchallengeresp=%x", encHash)
	resp3, err := pm.doPairReq(req3)
	if err != nil || resp3.Paired != 1 {
		return fmt.Errorf("步骤3失败: 服务端验证客户端挑战失败")
	}

	// 4. 验证服务端签名并检查 PIN
	serverSecretResp, _ := hex.DecodeString(resp3.PairingSecret)
	serverSecret := serverSecretResp[:16]
	serverSignature := serverSecretResp[16:]

	// 验证服务端证书的签名机制是否无被篡改
	hashedSecret := sha256.Sum256(serverSecret)
	err = rsa.VerifyPKCS1v15(pm.ServerCert.PublicKey.(*rsa.PublicKey), crypto.SHA256, hashedSecret[:], serverSignature)
	if err != nil {
		return fmt.Errorf("严重错误: 无法验证服务端的签名, 可能存在中间人攻击 (MITM)")
	}

	// 验证 PIN 码是否正确
	pinHashInput := append(randomChallenge, pm.ServerCert.Signature...)
	pinHashInput = append(pinHashInput, serverSecret...)
	serverChallengeRespHash := sha256.Sum256(pinHashInput)

	if !bytes.Equal(serverChallengeRespHash[:], serverResponse) {
		return fmt.Errorf("配对失败: PIN 码错误")
	}
	fmt.Println("PIN 码验证通过.")

	// 5. 客户端配对密钥 (clientpairingsecret)
	clientSecretHash := sha256.Sum256(clientSecret)
	clientSignature, _ := rsa.SignPKCS1v15(rand.Reader, pm.PrivateKey, crypto.SHA256, clientSecretHash[:])
	clientPairingSecret := append(clientSecret, clientSignature...)

	req4 := fmt.Sprintf("phrase=clientpairingsecret&clientpairingsecret=%x", clientPairingSecret)
	resp4, err := pm.doPairReq(req4)
	if err != nil || resp4.Paired != 1 {
		return fmt.Errorf("步骤5失败: 发送客户端私钥信息失败")
	}

	// 6. 最终配对确认 (pairchallenge)
	req5 := "phrase=pairchallenge"
	resp5, err := pm.doPairReq(req5)
	if err != nil || resp5.Paired != 1 {
		return fmt.Errorf("步骤6失败: 最终配对确认未通过")
	}

	fmt.Println("配对成功！保存服务端证书...")

	// 保存服务端证书到 AuthDir
	serverCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: pm.ServerCert.Raw})
	os.WriteFile(filepath.Join(AuthDir, "server_cert.pem"), serverCertPEM, 0644)

	return nil
}

// 初始化/加载本地证书
func initCerts() (*PairingManager, error) {
	os.MkdirAll(AuthDir, 0755)
	keyPath := filepath.Join(AuthDir, "client_key.pem")
	certPath := filepath.Join(AuthDir, "client_cert.pem")

	pm := &PairingManager{
		IP:         SunshineIP,
		Port:       SunshinePort,
		DeviceName: DeviceName,
	}

	// 检查是否已存在证书
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		fmt.Println("生成新的客户端证书和私钥...")
		// 生成 RSA 2048 密钥
		priv, _ := rsa.GenerateKey(rand.Reader, 2048)
		pm.PrivateKey = priv
		keyBytes := x509.MarshalPKCS1PrivateKey(priv)
		pem.Encode(os.Stdout, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes})
		os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes}), 0600)

		// 生成自签名证书
		template := x509.Certificate{
			SerialNumber:          big.NewInt(1),
			Subject:               pkix.Name{CommonName: "Moonlight Client"},
			NotBefore:             time.Now(),
			NotAfter:              time.Now().AddDate(10, 0, 0),
			KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			BasicConstraintsValid: true,
		}
		derBytes, _ := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
		pm.ClientPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
		os.WriteFile(certPath, pm.ClientPEM, 0644)
		pm.ClientCert, _ = x509.ParseCertificate(derBytes)
	} else {
		// 加载现有证书
		keyData, _ := os.ReadFile(keyPath)
		keyBlock, _ := pem.Decode(keyData)
		pm.PrivateKey, _ = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)

		certData, _ := os.ReadFile(certPath)
		certBlock, _ := pem.Decode(certData)
		pm.ClientPEM = certData
		pm.ClientCert, _ = x509.ParseCertificate(certBlock.Bytes)
	}

	return pm, nil
}

func SSTest() {
	pm, err := initCerts()
	if err != nil {
		fmt.Printf("初始化证书失败: %v\n", err)
		return
	}

	// 在 Sunshine UI 上可以查看到该 PIN 或者在终端中输入生成的 PIN
	pin := "1234" // 请替换为 Sunshine 界面中显示的 4 位配对码
	fmt.Printf("正在使用 PIN 码 [%s] 请求配对...\n", pin)

	err = pm.Pair(pin)
	if err != nil {
		fmt.Printf("错误: %v\n", err)
	} else {
		fmt.Println("与 Sunshine 服务端配对完成！")
	}
}
