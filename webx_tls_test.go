package webx

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/client"
)

// writeSelfSigned 生成自签 ECDSA 证书对（测试内联，不落 testdata 目录）
func writeSelfSigned(t *testing.T) (certPath, keyPath string) {
	t.Helper()
	dir := t.TempDir()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "webx.test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"webx.test", "localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	certFile, err := os.Create(certPath)
	if err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		t.Fatalf("pem cert: %v", err)
	}
	_ = certFile.Close()
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyFile, err := os.Create(keyPath)
	if err != nil {
		t.Fatalf("write key: %v", err)
	}
	if err := pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}); err != nil {
		t.Fatalf("pem key: %v", err)
	}
	_ = keyFile.Close()
	return certPath, keyPath
}

// TestTLSCertLoadFailFast 证书缺失 → New 返回含两个文件路径的错误
func TestTLSCertLoadFailFast(t *testing.T) {
	_, err := New(WithTLS("/definitely/missing/cert.pem", "/definitely/missing/key.pem", false))
	if err == nil {
		t.Fatal("New with missing cert: want error, got nil")
	}
	for _, p := range []string{"/definitely/missing/cert.pem", "/definitely/missing/key.pem"} {
		if !strings.Contains(err.Error(), p) {
			t.Fatalf("error = %q, want contains path %q", err.Error(), p)
		}
	}
}

// TestHTTPSOverTLS 自签证书 HTTPS（ALPN 通告 h2，未注册 h2 factory 时 h1-over-TLS 兜底）
func TestHTTPSOverTLS(t *testing.T) {
	cert, key := writeSelfSigned(t)
	a, err := TestApp(WithTLS(cert, key, true))
	if err != nil {
		t.Fatalf("TestApp(TLS): %v", err)
	}
	t.Cleanup(func() { _ = a.Shutdown(context.Background()) })

	hc, err := client.NewClient(client.WithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	req := newProtocolRequest(t, "GET", a.URL()+"/healthz/live")
	resp := newProtocolResponse()
	if err := hc.Do(context.Background(), req, resp); err != nil {
		t.Fatalf("HTTPS Do: %v", err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("HTTPS status = %d, want 200", resp.StatusCode())
	}
	if !strings.Contains(string(resp.Body()), "alive") {
		t.Fatalf("body = %s, want alive", resp.Body())
	}
}
