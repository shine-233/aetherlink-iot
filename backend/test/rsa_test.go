// 文件用途：验证 RSA 公私钥加载、OAEP 加解密和密码明文恢复流程。
// 核心逻辑：测试运行时生成临时 PEM 密钥，避免依赖仓库密钥或开发机固定路径。
// 关键注意事项：临时密钥仅用于测试，测试结束后由 testing.TempDir 自动清理。
package test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

var RSAPrivateKey *rsa.PrivateKey
var RSAPublicKey *rsa.PublicKey

func RsaDecryptInit(filePath string) error {
	key, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("load private key: %w", err)
	}
	block, _ := pem.Decode(key)
	if block == nil {
		return errors.New("load private key: invalid PEM block")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("parse private key: %w", err)
	}
	RSAPrivateKey = privateKey
	return nil
}

func RsaDecryptPublicInit(filePath string) error {
	key, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("load public key: %w", err)
	}
	block, _ := pem.Decode(key)
	if block == nil {
		return errors.New("load public key: invalid PEM block")
	}

	publicKey, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("parse public key: %w", err)
	}
	RSAPublicKey = publicKey
	return nil
}

func DecryptPassword(encryptedPassword string) ([]byte, error) {
	if RSAPrivateKey == nil {
		return nil, errors.New("private key is not initialized")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedPassword)
	if err != nil {
		return nil, fmt.Errorf("decode encrypted password: %w", err)
	}

	decrypted, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, RSAPrivateKey, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt password: %w", err)
	}

	return decrypted, nil
}

func HashPassword(decryptedPassword []byte, _ []byte) (password []byte, err error) {
	hashedPassword, err := bcrypt.GenerateFromPassword(decryptedPassword, bcrypt.DefaultCost)
	if err != nil {
		return password, fmt.Errorf("hash password: %w", err)
	}
	return hashedPassword, err
}

func Encrypt() (string, error) {
	if RSAPublicKey == nil {
		return "", errors.New("public key is not initialized")
	}
	message := []byte("123456salt")
	encryptedMessage, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, RSAPublicKey, message, nil)
	if err != nil {
		return "", fmt.Errorf("encrypt password: %w", err)
	}
	return base64.StdEncoding.EncodeToString(encryptedMessage), nil
}

func writeRSAFixture(t *testing.T) (string, string) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA fixture: %v", err)
	}
	dir := t.TempDir()
	privatePath := filepath.Join(dir, "private_key.pem")
	publicPath := filepath.Join(dir, "public.pem")
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: x509.MarshalPKCS1PublicKey(&privateKey.PublicKey)})
	if err := os.WriteFile(privatePath, privatePEM, 0o600); err != nil {
		t.Fatalf("write private RSA fixture: %v", err)
	}
	if err := os.WriteFile(publicPath, publicPEM, 0o600); err != nil {
		t.Fatalf("write public RSA fixture: %v", err)
	}
	return privatePath, publicPath
}

func resetRSAFixtureState(t *testing.T) {
	t.Helper()
	RSAPrivateKey = nil
	RSAPublicKey = nil
	t.Cleanup(func() {
		RSAPrivateKey = nil
		RSAPublicKey = nil
	})
}

func TestRSA(t *testing.T) {
	resetRSAFixtureState(t)
	privatePath, publicPath := writeRSAFixture(t)
	if err := RsaDecryptInit(privatePath); err != nil {
		t.Fatalf("initialize private key: %v", err)
	}
	if err := RsaDecryptPublicInit(publicPath); err != nil {
		t.Fatalf("initialize public key: %v", err)
	}

	password, err := Encrypt()
	if err != nil {
		t.Fatal(err)
	}
	passwords, err := DecryptPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSuffix(string(passwords), "salt"); got != "123456" {
		t.Fatalf("decrypted password = %q, want test fixture password", got)
	}
}

func TestRSAInitializationRejectsMissingAndInvalidPEM(t *testing.T) {
	resetRSAFixtureState(t)
	missing := filepath.Join(t.TempDir(), "missing.pem")
	if err := RsaDecryptInit(missing); err == nil {
		t.Fatal("RsaDecryptInit() error = nil, want missing file error")
	}
	invalid := filepath.Join(t.TempDir(), "invalid.pem")
	if err := os.WriteFile(invalid, []byte("not a PEM key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RsaDecryptInit(invalid); err == nil {
		t.Fatal("RsaDecryptInit() error = nil, want invalid PEM error")
	}
	if err := RsaDecryptPublicInit(invalid); err == nil {
		t.Fatal("RsaDecryptPublicInit() error = nil, want invalid PEM error")
	}
}

func TestRSAOperationsFailBeforeInitialization(t *testing.T) {
	resetRSAFixtureState(t)
	if _, err := Encrypt(); err == nil {
		t.Fatal("Encrypt() error = nil, want uninitialized public key error")
	}
	if _, err := DecryptPassword("not-base64"); err == nil {
		t.Fatal("DecryptPassword() error = nil, want uninitialized private key error")
	}
}
