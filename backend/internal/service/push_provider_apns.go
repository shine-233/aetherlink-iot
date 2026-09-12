// 文件用途：APNs（Apple Push Notification service）推送 Provider（ROADMAP P1.4）。
// 核心逻辑：用 .p8 私钥签一个 ES256 JWT 当 Bearer，走 HTTP/2 调用
// `POST https://api.{env}.push.apple.com/3/device/{token}` 投递。
//
// 关键注意事项：
//  1. **必须 HTTP/2**。APNs 对 HTTP/1.1 请求直接返回 400；Go 的 http.Client 只有在使用
//     支持 h2 的 Transport 时才会协商升级，因此这里显式 `http2.ConfigureTransport`。
//  2. **错误分类与 FCM 对齐**：429 / 5xx / 网络失败 = 可重试；
//     其余非 2xx = 终态。APNs 用 410 表示令牌对该话题已失效，
//     正是最典型的"重试一万次也没用"的场景，必须判终态。
//  3. 载荷里自定义字段只能放 `aps` 之外，且值必须是 APNs 接受的类型；
//     这里一律收敛成字符串，与 FCM 的 data 处理保持一致。
//  4. **未配置即不构造**（与 FCM 同规则）。空壳 Provider 会让能力矩阵报 Push=true
//     而每次发送都失败。
//  5. 本实现**未与真实 APNs 联调过**。httptest（开启 HTTP/2）覆盖了成功、可重试、
//     终态与请求头断言，但真实 APNs 的响应语义需在拿到真实凭据后补验
//     （见 push_provider_apns_test.go 里由环境变量触发的真实联调用例）。
package service

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
)

// APNs Provider 名称与端点。
const (
	PushProviderAPNS = "apns"

	apnsProductionHost = "https://api.push.apple.com"
	apnsSandboxHost    = "https://api.sandbox.push.apple.com"
)

var (
	ErrAPNSNotConfigured = errors.New("apns provider is not configured")
	ErrAPNSBadKey        = errors.New("apns signing key is invalid")
	ErrAPNSSendFailed    = errors.New("apns send failed")
)

// APNsConfig APNs Provider 配置。
type APNsConfig struct {
	// KeyID .p8 密钥的 Key ID（开发者后台生成）。
	KeyID string
	// TeamID 开发者团队 ID。
	TeamID string
	// Topic 通常是 App 的 Bundle ID，对应 `apns-topic` 头。
	Topic string
	// PrivateKeyP8 .p8 文件的 PEM 内容（不是文件路径）。
	PrivateKeyP8 string
	// Sandbox true 走 sandbox 端点，用于开发联调。
	Sandbox bool
	// BaseURL 覆盖端点，供测试使用。留空按 Sandbox 选择官方端点。
	BaseURL string
	// HTTPClient 覆盖 HTTP 客户端，仅供测试注入（测试服���的 TLS 证书需要信任）。
	// 留空则构造一个强制协商 HTTP/2 的客户端。
	HTTPClient *http.Client
	// Timeout 单次 HTTP 超时。<=0 用默认 10 秒。
	Timeout time.Duration
}

// apnsProvider APNs 实现。
type apnsProvider struct {
	keyID   string
	teamID  string
	topic   string
	signer  crypto.Signer
	client  *http.Client
	baseURL string
	now     func() time.Time

	mu     sync.Mutex
	jwt    string
	jwtExp time.Time
}

// NewAPNsProvider 由配置构造 APNs Provider。凭据缺失即返回错误（见文件头注意事项 4）。
func NewAPNsProvider(cfg APNsConfig) (PushProvider, error) {
	missing := make([]string, 0, 4)
	if strings.TrimSpace(cfg.KeyID) == "" {
		missing = append(missing, "key_id")
	}
	if strings.TrimSpace(cfg.TeamID) == "" {
		missing = append(missing, "team_id")
	}
	if strings.TrimSpace(cfg.Topic) == "" {
		missing = append(missing, "topic")
	}
	if strings.TrimSpace(cfg.PrivateKeyP8) == "" {
		missing = append(missing, "private_key_p8")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("%w: missing %s", ErrAPNSNotConfigured, strings.Join(missing, ", "))
	}
	signer, err := parseAPNSSigningKey(cfg.PrivateKeyP8)
	if err != nil {
		return nil, err
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = apnsProductionHost
		if cfg.Sandbox {
			baseURL = apnsSandboxHost
		}
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				// APNs 只接受 HTTP/2（见文件头注意事项 1）。
				// 设了 TLSClientConfig 后 Go 不会自动启用 h2，必须显式强制。
				ForceAttemptHTTP2: true,
			},
		}
	}
	return &apnsProvider{
		keyID:   strings.TrimSpace(cfg.KeyID),
		teamID:  strings.TrimSpace(cfg.TeamID),
		topic:   strings.TrimSpace(cfg.Topic),
		signer:  signer,
		client:  client,
		baseURL: strings.TrimRight(baseURL, "/"),
		now:     time.Now,
	}, nil
}

// parseAPNSSigningKey 解析 .p8（PKCS8 PEM）里的 EC 私钥。
func parseAPNSSigningKey(raw string) (crypto.Signer, error) {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, fmt.Errorf("%w: private_key_p8 is not PEM", ErrAPNSBadKey)
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAPNSBadKey, err)
	}
	ecKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%w: APNs requires an EC (P-256) key", ErrAPNSBadKey)
	}
	return ecKey, nil
}

// Name 通道名。
func (p *apnsProvider) Name() string { return PushProviderAPNS }

// Supports APNs 只服务 iOS。
func (p *apnsProvider) Supports(platform string) bool {
	return strings.EqualFold(strings.TrimSpace(platform), model.PushPlatformIOS)
}

// Send 投递一条推送。
func (p *apnsProvider) Send(ctx context.Context, msg PushMessage) error {
	token := strings.TrimSpace(msg.Token)
	if token == "" {
		return &PushTerminalError{Reason: "push token is empty"}
	}
	bearer, err := p.authToken()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(apnsPayload(msg))
	if err != nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "push payload could not be encoded")
	}
	url := p.baseURL + "/3/device/" + token
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "bearer "+bearer)
	req.Header.Set("apns-topic", p.topic)
	req.Header.Set("apns-push-type", "alert")
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrAPNSSendFailed, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return fmt.Errorf("%w: status %d: %s", ErrAPNSSendFailed, resp.StatusCode, strings.TrimSpace(string(body)))
	default:
		// 410 Unregistered / 400 BadDeviceToken / 403 等：重试无意义。
		// 带上状态码的人读解释：凭一个数字去查文档是运维最常浪费的时间。
		return &PushTerminalError{
			Reason: fmt.Sprintf("%s: %s: %s", ErrAPNSSendFailed, apnsStatusLabel(resp.StatusCode), strings.TrimSpace(string(body))),
		}
	}
}

// apnsPayload 组装 APNs 载荷：自定义数据只能放在 aps 之外。
func apnsPayload(msg PushMessage) map[string]interface{} {
	out := map[string]interface{}{
		"aps": map[string]interface{}{
			"alert": map[string]string{
				"title": msg.Title,
				"body":  msg.Body,
			},
			"sound": "default",
		},
	}
	for k, v := range fcmStringData(msg.Data) {
		out[k] = v
	}
	return out
}

// authToken 取有效的 ES256 JWT，过期前复用。
// APNs 建议令牌最长 1 小时，且不要每条推送都重新签。
func (p *apnsProvider) authToken() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	if p.jwt != "" && now.Before(p.jwtExp) {
		return p.jwt, nil
	}
	signed, exp, err := p.signJWT(now)
	if err != nil {
		return "", err
	}
	p.jwt = signed
	p.jwtExp = exp
	return signed, nil
}

// signJWT 签一个 ES256 JWT。
func (p *apnsProvider) signJWT(now time.Time) (string, time.Time, error) {
	// 55 分钟：APNs 上限 1 小时，留出时钟漂移与在途请求的余量。
	expiresAt := now.Add(55 * time.Minute)
	header := map[string]string{"alg": "ES256", "typ": "JWT", "kid": p.keyID}
	claims := map[string]interface{}{
		"iss": p.teamID,
		"iat": now.Unix(),
		"exp": expiresAt.Unix(),
	}
	headerRaw, err := json.Marshal(header)
	if err != nil {
		return "", time.Time{}, err
	}
	claimsRaw, err := json.Marshal(claims)
	if err != nil {
		return "", time.Time{}, err
	}
	signingInput := base64.RawURLEncoding.EncodeToString(headerRaw) + "." + base64.RawURLEncoding.EncodeToString(claimsRaw)
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := p.signer.Sign(rand.Reader, digest[:], crypto.SHA256)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("%w: %v", ErrAPNSBadKey, err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), expiresAt, nil
}

// apnsStatusLabel 仅供日志/诊断使用，把状态码翻成人能读的原因。
func apnsStatusLabel(status int) string {
	switch status {
	case 400:
		return "bad request (often a malformed payload or device token)"
	case 403:
		return "certificate or topic mismatch"
	case 410:
		return "device token is no longer active for the topic"
	case 413:
		return "payload too large"
	case 429:
		return "too many requests"
	default:
		return "status " + strconv.Itoa(status)
	}
}
