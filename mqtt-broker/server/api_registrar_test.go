// 文件用途：维护 server\api_registrar_test.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"github.com/DrmagicE/gmqtt/config"
)

func TestAPIRegistrar_serveAPIServer(t *testing.T) {
	a := assert.New(t)
	var serveCount, shutdownCount int
	srv := &server{
		wg:       sync.WaitGroup{},
		exitChan: make(chan struct{}),
		apiRegistrar: &apiRegistrar{
			gRPCServers: []*gRPCServer{
				{
					server: &grpc.Server{},
					serve: func(errChan chan error) error {
						serveCount++
						return nil
					},
					shutdown: func() {
						shutdownCount++
					},
					endpoint: "tcp://127.0.0.1:1234",
				},
			},
			httpServers: []*httpServer{
				{
					gRPCEndpoint: "tcp://127.0.0.1:1234",
					serve: func(errChan chan error) error {
						serveCount++
						return nil
					},
					shutdown: func() {
						shutdownCount++
					},
					endpoint: "tcp://127.0.0.1:1234",
				},
			},
		},
	}
	srv.wg.Add(1)
	close(srv.exitChan)
	srv.serveAPIServer()
	done := make(chan struct{})
	go func() {
		srv.wg.Wait()
		close(done)
	}()
	select {
	case <-time.After(1 * time.Second):
		t.Fatal("serveAPIServer should exit immediately")
	case <-done:
	}
	a.Equal(2, serveCount)
	a.Equal(2, shutdownCount)

}

func TestAPIRegistrar_serveAPIServer_WithError(t *testing.T) {
	a := assert.New(t)
	var serveCount, shutdownCount int
	srv := &server{
		wg:       sync.WaitGroup{},
		exitChan: make(chan struct{}),
		apiRegistrar: &apiRegistrar{
			gRPCServers: []*gRPCServer{
				{
					server: &grpc.Server{},
					serve: func(errChan chan error) error {
						serveCount++
						return errors.New("some thing wrong")
					},
					shutdown: func() {
						shutdownCount++
					},
					endpoint: "tcp://127.0.0.1:1234",
				},
			},
			httpServers: []*httpServer{
				{
					gRPCEndpoint: "tcp://127.0.0.1:1234",
					serve: func(errChan chan error) error {
						serveCount++
						return nil
					},
					shutdown: func() {
						shutdownCount++
					},
					endpoint: "tcp://127.0.0.1:1234",
				},
			},
		},
	}
	srv.wg.Add(1)
	srv.serveAPIServer()
	done := make(chan struct{})
	go func() {
		srv.wg.Wait()
		close(done)
	}()
	select {
	case <-time.After(1 * time.Second):
		t.Fatal("serveAPIServer should exit immediately")
	case <-done:
	}
	a.Equal(1, serveCount)
	a.Equal(2, shutdownCount)
}

func TestAPIRegistrar_serveAPIServer_WithErrorChan(t *testing.T) {
	a := assert.New(t)
	var serveCount, shutdownCount int
	srv := &server{
		wg:       sync.WaitGroup{},
		exitChan: make(chan struct{}),
		apiRegistrar: &apiRegistrar{
			gRPCServers: []*gRPCServer{
				{
					server: &grpc.Server{},
					serve: func(errChan chan error) error {
						errChan <- errors.New("something wrong")
						return nil
					},
					shutdown: func() {
						shutdownCount++
					},
					endpoint: "tcp://127.0.0.1:1234",
				},
			},
			httpServers: []*httpServer{
				{
					gRPCEndpoint: "tcp://127.0.0.1:1234",
					serve: func(errChan chan error) error {
						serveCount++
						return nil
					},
					shutdown: func() {
						shutdownCount++
					},
					endpoint: "tcp://127.0.0.1:1234",
				},
			},
		},
	}
	srv.wg.Add(1)
	srv.serveAPIServer()
	done := make(chan struct{})
	go func() {
		srv.wg.Wait()
		close(done)
	}()
	select {
	case <-time.After(1 * time.Second):
		t.Fatal("serveAPIServer should exit immediately")
	case <-done:
	}
	a.Equal(1, serveCount)
	a.Equal(2, shutdownCount)
}

func TestApiRegistrar_RegisterHTTPHandler(t *testing.T) {
	a := assert.New(t)
	// test unix socket
	reg := &apiRegistrar{
		httpServers: []*httpServer{
			{
				gRPCEndpoint: "unix:///var/run/gmqttd.sock",
				endpoint:     "",
				mux:          &runtime.ServeMux{},
				serve: func(errChan chan error) error {
					return nil
				},
				shutdown: func() {
					return
				},
			},
		},
	}
	a.NoError(reg.RegisterHTTPHandler(func(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) (err error) {
		a.Equal(reg.httpServers[0].gRPCEndpoint, endpoint)
		return nil
	}))

	// test tcp socket
	reg = &apiRegistrar{
		httpServers: []*httpServer{
			{
				gRPCEndpoint: "tcp://127.0.0.1:1234",
				endpoint:     "",
				mux:          &runtime.ServeMux{},
				serve: func(errChan chan error) error {
					return nil
				},
				shutdown: func() {
					return
				},
			},
		},
	}
	a.NoError(reg.RegisterHTTPHandler(func(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) (err error) {
		a.Equal("127.0.0.1:1234", endpoint)
		return nil
	}))
}

func TestBuildTLSConfig(t *testing.T) {
	caPath, certPath, keyPath, certPEM := createTLSFixture(t)

	t.Run("verify_false", func(t *testing.T) {
		a := assert.New(t)
		cfg := &config.TLSOptions{
			CACert: "",
			Cert:   certPath,
			Key:    keyPath,
			Verify: false,
		}
		tlsCfg, err := buildTLSConfig(cfg)
		a.NoError(err)
		a.EqualValues(0, tlsCfg.ClientAuth)
		a.Len(tlsCfg.Certificates, 1)
	})

	t.Run("verify_true", func(t *testing.T) {
		a := assert.New(t)
		cfg := &config.TLSOptions{
			CACert: "",
			Cert:   certPath,
			Key:    keyPath,
			Verify: true,
		}
		tlsCfg, err := buildTLSConfig(cfg)
		a.NoError(err)
		a.EqualValues(tls.RequireAndVerifyClientCert, tlsCfg.ClientAuth)
		a.Len(tlsCfg.Certificates, 1)
	})

	t.Run("add_cacert", func(t *testing.T) {
		a := assert.New(t)
		cfg := &config.TLSOptions{
			CACert: caPath,
			Cert:   certPath,
			Key:    keyPath,
		}
		tlsCfg, err := buildTLSConfig(cfg)
		a.NoError(err)
		a.Len(tlsCfg.Certificates, 1)
		opts := x509.VerifyOptions{
			DNSName: "drmagic.local",
			Roots:   tlsCfg.ClientCAs,
		}
		block, _ := pem.Decode(certPEM)
		cert, err := x509.ParseCertificate(block.Bytes)
		_, err = cert.Verify(opts)
		a.Nil(err)
	})

}

func createTLSFixture(t *testing.T) (caPath, certPath, keyPath string, certPEM []byte) {
	t.Helper()

	dir := t.TempDir()
	now := time.Now()
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "AetherLink IoT test CA"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create CA certificate: %v", err)
	}

	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate server key: %v", err)
	}
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "drmagic.local"},
		DNSNames:     []string{"drmagic.local"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caTemplate, &serverKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create server certificate: %v", err)
	}

	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverKey)})

	caPath = filepath.Join(dir, "ca.pem")
	certPath = filepath.Join(dir, "server-cert.pem")
	keyPath = filepath.Join(dir, "server-key.pem")
	for path, data := range map[string][]byte{
		caPath:   caPEM,
		certPath: certPEM,
		keyPath:  keyPEM,
	} {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatalf("write TLS fixture %s: %v", path, err)
		}
	}

	return caPath, certPath, keyPath, certPEM
}
