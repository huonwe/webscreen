package sunshine

import (
	"crypto/aes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"time"
)

// --- 配置区域 ---
const (
	SunshineIP   = "192.168.0.61" // 请修改为你的 Sunshine IP
	SunshinePort = "47984"        // 默认端口
	DeviceName   = "GoClient"     // 设备名称
	PIN          = "1234"         // PIN 码
)

func SSTest() {
	log.SetFlags(log.Ltime)
	log.Printf("=== Sunshine 配对工具 ===")
	log.Printf("目标: https://%s:%s", SunshineIP, SunshinePort)
	log.Printf("PIN码: %s", PIN)

	// 1. 生成标准的 UUID (伪随机)
	uuid := generateUUID()
	log.Printf("生成设备ID: %s", uuid)

	// 2. 生成证书
	certPEM, keyPEM, certDER, err := generateCert()
	if err != nil {
		log.Fatalf("证书生成失败: %v", err)
	}
	// 计算证书指纹 (用于调试)
	certHash := sha256.Sum256(certDER)
	log.Printf("证书指纹(SHA256): %x", certHash)

	// 3. 准备 TLS 客户端
	clientCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		log.Fatalf("加载证书失败: %v", err)
	}
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,                          // 忽略 Sunshine 自签名证书错误
				Certificates:       []tls.Certificate{clientCert}, // 必须携带客户端证书
			},
		},
		Timeout: 5 * time.Second,
	}

	// 4. 生成挑战 (Salt + Challenge)
	saltBytes := make([]byte, 16)
	rand.Read(saltBytes)
	// Moonlight 使用大写 Hex
	saltHex := fmt.Sprintf("%X", saltBytes)

	// 计算 Challenge: AES(Key=SHA256(Salt+PIN), Data=SHA256(Cert))
	challenge, err := calculateChallenge(certDER, saltBytes, PIN)
	if err != nil {
		log.Fatalf("挑战计算失败: %v", err)
	}
	log.Printf("生成挑战数据: %s", challenge)

	// 转换 PEM 证书为 Hex 字符串 (关键缺失步骤!)
	clientCertHex := fmt.Sprintf("%X", certPEM)

	// 5. 构造请求 URL
	params := url.Values{}
	params.Add("uniqueid", uuid)
	params.Add("devicename", DeviceName)
	params.Add("updateState", "1")
	params.Add("phrase", PIN)
	params.Add("salt", saltHex)
	params.Add("clientchallenge", challenge)
	params.Add("clientcert", clientCertHex) // 【修复】必须发送证书给服务端

	// 注意：url.Values Encode 会对内容进行 URL 编码，这通常是正确的
	// 但如果 Sunshine 对 Hex 字符串的解析极其严格，可能需要手动拼接
	// 这里先用 Encode()，通常标准库没问题
	targetURL := fmt.Sprintf("https://%s:%s/pair?%s", SunshineIP, SunshinePort, params.Encode())

	// 6. 循环发送请求
	log.Println(">>> 正在发送配对请求...")
	log.Println(">>> 请现在去 Sunshine WebUI -> PIN 页面输入: " + PIN)

	for {
		resp, err := client.Get(targetURL)
		if err != nil {
			log.Printf("请求错误 (Sunshine未启动?): %v", err)
			time.Sleep(3 * time.Second)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		bodyStr := string(body)

		if resp.StatusCode == 200 {
			if contains(bodyStr, "paired") && contains(bodyStr, "1") {
				log.Println("✅ 配对成功！")
				log.Println("请保存以下证书和私钥，后续连接必须使用它们：")
				saveFile("client_cert.pem", certPEM)
				saveFile("client_key.pem", keyPEM)
				return
			} else if contains(bodyStr, "The client is not authorized") {
				log.Println("❌ 验证失败：请检查 PIN 码是否输入正确，或尝试刷新 Sunshine 页面")
			} else {
				log.Printf("等待中... 服务端状态: %s", bodyStr)
			}
		} else {
			log.Printf("HTTP %d: %s", resp.StatusCode, bodyStr)
		}

		time.Sleep(2 * time.Second)
	}
}

// --- 核心加密逻辑 ---

func calculateChallenge(certDER []byte, salt []byte, pin string) (string, error) {
	// 1. 计算 AES Key = SHA256(Salt + PIN) 的前 16 字节
	// 关键点：Salt 是原始字节，PIN 是字符串字节
	keyBase := append(salt, []byte(pin)...)
	keyHash := sha256.Sum256(keyBase)
	aesKey := keyHash[:16]

	// 2. 计算数据 Data = SHA256(CertDER)
	certSig := sha256.Sum256(certDER)

	// 3. AES-128-ECB 加密
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", err
	}

	encrypted := make([]byte, len(certSig)) // 32 bytes
	blockSize := block.BlockSize()          // 16 bytes

	// 手动执行 ECB 模式 (分块加密)
	for i := 0; i < len(certSig); i += blockSize {
		block.Encrypt(encrypted[i:i+blockSize], certSig[i:i+blockSize])
	}

	// Moonlight 使用大写 Hex
	return fmt.Sprintf("%X", encrypted), nil
}

// --- 辅助函数 ---

func generateCert() ([]byte, []byte, []byte, error) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "NVIDIA GameStream Client"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour * 24 * 3650),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, nil, err
	}

	certOut := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyOut := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	return certOut, keyOut, der, nil
}

func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant is 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[0:len(substr)] == substr || (len(s) > len(substr) && contains(s[1:], substr))
}

func saveFile(name string, data []byte) {
	os.WriteFile(name, data, 0644)
	log.Printf("已保存: %s", name)
}
