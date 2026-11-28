package gamestream

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"scrmux/consts"
	"scrmux/sdriver"
	"testing"
	"time"
)

func TestRSDecodeRecovery(t *testing.T) {
	const (
		dataShards   = 4
		parityShards = 2
		shardLen     = 64
	)
	total := dataShards + parityShards
	shards := make([][]byte, total)
	for i := 0; i < dataShards; i++ {
		shards[i] = make([]byte, shardLen)
		for b := 0; b < shardLen; b++ {
			shards[i][b] = byte((i*37 + b*13 + 7) & 0xFF)
		}
	}
	// Generate parity shards using systematic RS matrix: P[j][i] = inv((parityShards + i) ^ j)
	for j := 0; j < parityShards; j++ {
		shards[dataShards+j] = make([]byte, shardLen)
		for i := 0; i < dataShards; i++ {
			coeff := gfInv[(parityShards+i)^j]
			for b := 0; b < shardLen; b++ {
				shards[dataShards+j][b] ^= gfMul(coeff, shards[i][b])
			}
		}
	}

	orig0 := append([]byte(nil), shards[0]...)
	orig2 := append([]byte(nil), shards[2]...)

	// Drop 2 data shards (shards 0 and 2)
	shards[0] = nil
	shards[2] = nil
	present := []bool{false, true, false, true, true, true}

	if err := rsDecode(shards, present, dataShards, parityShards, shardLen); err != nil {
		t.Fatalf("rsDecode failed: %v", err)
	}
	if !bytes.Equal(shards[0], orig0) {
		t.Fatalf("shard 0 mismatch after recovery")
	}
	if !bytes.Equal(shards[2], orig2) {
		t.Fatalf("shard 2 mismatch after recovery")
	}
}

func TestEncryptControlPacket(t *testing.T) {
	var key [16]byte
	for i := range key {
		key[i] = byte(i + 1)
	}
	blk, err := aes.NewCipher(key[:])
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(blk)
	if err != nil {
		t.Fatal(err)
	}

	payload := []byte{0xAA, 0xBB, 0xCC}
	pkt := encryptControlPacket(aead, 42, 0x0206, payload, true)

	if hdrType := binary.LittleEndian.Uint16(pkt[0:2]); hdrType != 0x0001 {
		t.Fatalf("unexpected header type 0x%x", hdrType)
	}
	if seq := binary.LittleEndian.Uint32(pkt[4:8]); seq != 42 {
		t.Fatalf("unexpected seq %d", seq)
	}

	var iv [12]byte
	binary.LittleEndian.PutUint32(iv[0:4], 42)
	iv[10] = 'C'
	iv[11] = 'C'

	tag := pkt[8:24]
	ct := pkt[24:]
	sealed := append(append([]byte(nil), ct...), tag...)
	plain, err := aead.Open(nil, iv[:], sealed, nil)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if ptype := binary.LittleEndian.Uint16(plain[0:2]); ptype != 0x0206 {
		t.Fatalf("unexpected inner ptype 0x%x", ptype)
	}
	if plen := binary.LittleEndian.Uint16(plain[2:4]); plen != 3 {
		t.Fatalf("unexpected inner len %d", plen)
	}
	if !bytes.Equal(plain[4:], payload) {
		t.Fatalf("unexpected inner payload %x", plain[4:])
	}
}

func TestEmitVideoFrameSplitsNALs(t *testing.T) {
	d := &Driver{
		videoChan: make(chan sdriver.AVBox, 8),
		done:      make(chan struct{}),
		meta:      sdriver.MediaMeta{VideoCodec: consts.CODEC_TYPE_H264},
	}

	// SPS (type 7) + PPS (type 8) + IDR (type 5)
	annexB := []byte{
		0x00, 0x00, 0x00, 0x01, 0x67, 0x42, 0x00, 0x1E,
		0x00, 0x00, 0x00, 0x01, 0x68, 0xCE, 0x3C, 0x80,
		0x00, 0x00, 0x01, 0x65, 0x88, 0x84,
	}
	waitingForIDR := true
	isIDR := d.emitVideoFrame(annexB, 1, &waitingForIDR)
	if !isIDR || waitingForIDR {
		t.Fatalf("expected IDR frame and waitingForIDR cleared")
	}
	if len(d.videoChan) != 1 {
		t.Fatalf("expected 1 bundled Annex-B keyframe box, got %d", len(d.videoChan))
	}
	keyframe := <-d.videoChan
	expected := []byte{
		0x00, 0x00, 0x00, 0x01, 0x67, 0x42, 0x00, 0x1E,
		0x00, 0x00, 0x00, 0x01, 0x68, 0xCE, 0x3C, 0x80,
		0x00, 0x00, 0x00, 0x01, 0x65, 0x88, 0x84,
	}
	if keyframe.NoDuration || !bytes.Equal(keyframe.Data, expected) {
		t.Fatalf("unexpected bundled keyframe box: %+v", keyframe)
	}
}

func TestParseConfig(t *testing.T) {
	cfg, err := parseConfig(map[string]string{
		"ip":             "192.168.1.10",
		"video_codec":    consts.CODEC_TYPE_H265,
		"video_bit_rate": "15M",
		"resolution":     "2560x1440",
		"frame_rate":     "120",
	})
	if err != nil {
		t.Fatalf("parseConfig failed: %v", err)
	}
	if cfg.bitrate != 15000 || cfg.width != 2560 || cfg.height != 1440 || cfg.fps != 120 || cfg.codec != consts.CODEC_TYPE_H265 {
		t.Fatalf("unexpected parsed config: %+v", cfg)
	}
}

func TestParseServerCertHex(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Sunshine"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	// Sunshine sends hex-encoded PEM
	if cert, err := parseServerCertHex(hex.EncodeToString(pemBytes)); err != nil || !bytes.Equal(cert.Raw, der) {
		t.Fatalf("PEM-in-hex parse failed: err=%v", err)
	}
	// GFE sends hex-encoded DER
	if cert, err := parseServerCertHex(hex.EncodeToString(der)); err != nil || !bytes.Equal(cert.Raw, der) {
		t.Fatalf("DER-in-hex parse failed: err=%v", err)
	}
}
