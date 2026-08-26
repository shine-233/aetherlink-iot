// 文件用途：提供第三方 HTTP 请求的基础方法和签名 Webhook 发送能力。
// 核心逻辑：统一创建 JSON 请求，复用带超时的默认 http.Client，并支持 HMAC-SHA256 签名头。
// 关键注意事项：返回 *http.Response 的导出函数（DisconnectDevice 等）由调用方负责关闭 Body；
// 包内消费型封装必须在函数内 defer Close。新增调用点必须避免连接泄漏。
package http_client

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

const defaultHTTPTimeout = 10 * time.Second

var defaultHTTPClient = &http.Client{Timeout: defaultHTTPTimeout}

// signedRequestHTTPClient does not follow redirects: a webhook redirect is not
// an acknowledgement from the configured endpoint, and following it can change
// POST semantics or forward a signed payload to an unintended destination.
var signedRequestHTTPClient = &http.Client{
	Timeout: defaultHTTPTimeout,
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func Get(url string) ([]byte, error) {
	resp, err := defaultHTTPClient.Get(url)
	if err != nil {
		logrus.Error(err.Error())
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("Get failed with error: " + resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	logrus.Info("Response: ", string(body))
	return body, nil
}

func PostJson(targetUrl string, payload []byte) (*http.Response, error) {
	req, err := newJSONRequest(context.Background(), http.MethodPost, targetUrl, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	return defaultHTTPClient.Do(req)
}

func generateHMAC(message, secret string) string {
	key := []byte(secret)
	h := hmac.New(sha256.New, key)
	h.Write([]byte(message))
	signature := h.Sum(nil)
	return hex.EncodeToString(signature)
}

func SendSignedRequest(url, message, secret string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultHTTPTimeout)
	defer cancel()
	return sendSignedRequest(ctx, url, message, secret)
}

func SendSignedRequestWithTimeout(ctx context.Context, url, message, secret string) error {
	return sendSignedRequest(ctx, url, message, secret)
}

func sendSignedRequest(ctx context.Context, url, message, secret string) error {
	req, err := newJSONRequest(ctx, http.MethodPost, url, bytes.NewBufferString(message))
	if err != nil {
		return fmt.Errorf("create signed request: %w", err)
	}

	signature := generateHMAC(message, secret)
	req.Header.Set("X-Signature-256", "sha256="+signature)

	resp, err := signedRequestHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("send signed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("signed request failed with HTTP status %d", resp.StatusCode)
	}

	logrus.Info("Webhook request sent, status code:", resp.StatusCode)
	return nil
}

func newJSONRequest(ctx context.Context, method, targetUrl string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, targetUrl, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}
