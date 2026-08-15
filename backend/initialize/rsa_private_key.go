// 文件用途：加载 RSA 私钥并提供密码解密能力，服务于启动后需要解密敏感配置的流程。
// 核心逻辑：从 PEM 文件解析 PKCS1 私钥，保存到全局变量，并基于该私钥执行 PKCS1v15 解密。
// 关键注意事项：全局私钥必须在解密前初始化完成；注释只能说明现有行为，不引入额外密钥管理承诺。
// 重构建议：后续可将密钥加载器和解密器封装为显式依赖，减少对包级全局变量的依赖。

package initialize

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

var RSAPrivateKey *rsa.PrivateKey

// RsaDecryptInit 从指定文件加载 RSA 私钥，并保存到全局解密上下文。
func RsaDecryptInit(filePath string) (err error) {
	key, err := os.ReadFile(filePath)
	if err != nil {
		return errors.New("加载私钥错误1：" + err.Error())
	}
	block, _ := pem.Decode(key)
	if block == nil {
		return errors.New("加载私钥错误2：")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return errors.New("加载私钥错误3：" + err.Error())
	}
	RSAPrivateKey = privateKey
	return err
}

// DecryptPassword 对 Base64 编码的密文执行 RSA PKCS1v15 解密。
// 未配置私钥时返回明确错误；默认关闭的前端加密能力不得因空指针导致服务崩溃。
func DecryptPassword(encryptedPassword string) ([]byte, error) {
	if RSAPrivateKey == nil {
		return nil, errors.New("RSA private key is not configured; frontend RSA encryption is unavailable")
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encryptedPassword)
	if err != nil {
		return nil, fmt.Errorf("解码密文失败: %v", err)
	}

	decrypted, err := rsa.DecryptPKCS1v15(rand.Reader, RSAPrivateKey, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("解密失败: %v", err)
	}

	return decrypted, nil
}
