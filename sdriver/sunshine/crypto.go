package sunshine

import (
	"crypto/aes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"time"
)

// GenerateCerts 生成 RSA 密钥对和自签名证书
// 返回:
// 1. certPEM: 证书 PEM 字符串 (用于 HTTP TLS 配置)
// 2. privPEM: 私钥 PEM 字符串 (用于 HTTP TLS 配置)
// 3. certDER: 证书原始 DER 字节 (关键！用于生成 GameStream 配对挑战)
func GenerateCerts() (string, string, []byte, error) {
	// 1. 生成 RSA 私钥 (2048位)
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", nil, err
	}

	// 2. 构建证书模板
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "NVIDIA GameStream Client", // 这是一个固定标识，最好保留
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour * 24 * 365 * 10), // 10年有效期

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// 3. 自签名创建证书 (DER 格式)
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return "", "", nil, err
	}

	// 4. 编码为 PEM 格式
	certBlock := &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}
	certPem := pem.EncodeToMemory(certBlock)

	privBytes := x509.MarshalPKCS1PrivateKey(priv)
	privBlock := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes}
	privPem := pem.EncodeToMemory(privBlock)

	return string(certPem), string(privPem), derBytes, nil
}

// CreateChallenge 计算 GameStream 配对挑战
// pin: 4位数字字符串
// salt: 16字节随机十六进制字符串
// certPem: 证书的 PEM 字符串
func CreateChallenge(pin string, saltHex string, certDer []byte) (string, error) { // 1. 解析 Salt
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return "", err
	}

	// 2. 生成 AES Key
	// Key = SHA256(salt + pin) 的前16字节
	keyBase := append(salt, []byte(pin)...)
	hasher := sha256.New()
	hasher.Write(keyBase)
	keyHash := hasher.Sum(nil)
	aesKey := keyHash[:16] // AES-128

	// 3. 生成要加密的数据 (Signature)
	// 关键修正：必须对 DER 二进制数据进行哈希，而不是 PEM 字符串
	hasher.Reset()
	hasher.Write(certDer) // 使用传入的 DER 字节
	certSignature := hasher.Sum(nil)

	// 4. 使用 AES-ECB 加密
	// 因为 Data 是32字节，刚好是两个 AES 块(16字节)，不需要填充
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", err
	}

	encrypted := make([]byte, len(certSignature))

	// ECB 模式手动分块加密
	blockSize := block.BlockSize()
	for i := 0; i < len(certSignature); i += blockSize {
		block.Encrypt(encrypted[i:i+blockSize], certSignature[i:i+blockSize])
	}

	// 5. 返回 Hex 字符串
	return hex.EncodeToString(encrypted), nil
}

// RandomSalt 生成指定长度的随机 Hex 字符串
func RandomSalt(size int) string {
	b := make([]byte, size)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
