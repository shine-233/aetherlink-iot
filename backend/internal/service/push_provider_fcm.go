// 文件用途：FCM（Firebase Cloud Messaging）HTTP v1 推送 Provider（ROADMAP P1.4）。
// 核心逻辑：用服务账号私钥签一个 RS256 JWT 换 OAuth2 访问令牌，再调用
// `POST /v1/projects/{project}/messages:send` 投递。
//
// 关键注意事项：
//  1. **错误必须分可重试与终态**。FCM 返回 404 / UNREGISTERED 表示令牌已失效，
//     重试一万次也不会成功；把它当成普通失败会占满重试预算，还让"令牌失效"
//     这个事实被埋在最后一次 last_error 里。故这里返回 `PushTerminalError`，
//     由 PushService 直接置 dead。
//  2. 429 / 5xx 判为可重试：前者要遵守 Retry-After，后者是对方临时故障。
//     其余 4xx 一律终态——参数错、权限错，重试无意义。
//  3. 访问令牌必须缓存到过期前：每条推送都去换令牌会把配额耗在换票上。
//  4. **未配置即不构造**（见 AssemblePush*）。构造一个没有凭据的 Provider
//     会让能力矩阵报 Push=true，而每次发送都失败——这正是要避免的假成功。
//  5. 本实现**未与真实 FCM 联调过**。httptest 覆盖了令牌交换、成功、可重试、
//     终态与网络失败各条分支，但真实 FCM 的响应细节（尤其是错误码分布）
//     需在拿到真实凭据后补一次联调验证。
package service

import (
	"context"
	"crypto"
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
	"net/url"
	"strings"
	"sync"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
)

// FCM Provider 名称与默认端点。
const (
	PushProviderFCM = "fcm"

	fcmDefaultTokenURL = "https://oauth2.googleapis.com/token"
	fcmDefaultSendURL  = "https://fcm.googleapis.com/v1/projects/%s/messages:send"
	fcmOAuthScope      = "https://www.googleapis.com/auth/firebase.messaging"
)

// FCM 只服务 Android 平台（iOS 需 APNs 通道，本项目尚未实现；
// h5 没有系统级推送通道）。平台取值复用 model 里的常量，避免两处各写一份字符串。
const pushPlatformFCM = model.PushPlatformAndroid

var (
	ErrFCMNotConfigured   = errors.New("fcm provider is not configured")
	ErrFCMBadAccount      = errors.New("fcm service account is invalid")
	ErrFCMTokenExchange   = errors.New("fcm access token exchange failed")
	ErrFCMSendFailed      = errors.New("fcm send failed")
	ErrFCMResponseInvalid = errors.New("fcm returned an unexpected response")
)

// PushTerminalError 标记"重试也不会成功"的失败。
// PushService 见到它就不再排下一次重试，直接置 dead。
type PushTerminalError struct {
	Reason string
}

func (e *PushTerminalError) Error() string { return e.Reason }

// IsPushTerminal 判断一个错误是否属于终态失败。
func IsPushTerminal(err error) bool {
	var target *PushTerminalError
	return errors.As(err, &target)
}

// fcmServiceAccount 服务账号凭据里我们真正用到的字段。
type fcmServiceAccount struct {
	Type        string `json:"type"`
	ProjectID   string `json:"project_id"`
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

// FCMConfig FCM Provider 配置。
type FCMConfig struct {
	// ProjectID Firebase 项目 ID。为空即视为未配置。
	ProjectID string
	// ServiceAccountJSON 服务账号 JSON 内容（不是文件路径——把路径交给本层去读
	// 会让"凭据能不能读到"变成一个运行期才暴露的问题）。
	ServiceAccountJSON string
	// TokenURL / SendURL 可覆盖，供测试与代理环境使用。留空用官方端点。
	TokenURL string
	SendURL  string
	// Timeout 单次 HTTP 超时。<=0 用默认 10 秒。
	Timeout time.Duration
}

// fcmProvider FCM HTTP v1 实现。
type fcmProvider struct {
	projectID string
	account   fcmServiceAccount
	signer    crypto.Signer
	client    *http.Client
	tokenURL  string
	sendURL   string

	mu       sync.Mutex
	token    string
	tokenExp time.Time
	now      func() time.Time
}

// NewFCMProvider 由配置构造 FCM Provider。
// 凭据缺失或不可解析时返回错误：见文件头注意事项 4。
func NewFCMProvider(cfg FCMConfig) (PushProvider, error) {
	if strings.TrimSpace(cfg.ProjectID) == "" {
		return nil, ErrFCMNotConfigured
	}
	if strings.TrimSpace(cfg.ServiceAccountJSON) == "" {
		return nil, ErrFCMNotConfigured
	}
	var account fcmServiceAccount
	if err := json.Unmarshal([]byte(cfg.ServiceAccountJSON), &account); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFCMBadAccount, err)
	}
	if strings.TrimSpace(account.ClientEmail) == "" || strings.TrimSpace(account.PrivateKey) == "" {
		return nil, fmt.Errorf("%w: client_email and private_key are required", ErrFCMBadAccount)
	}
	signer, err := parseServiceAccountKey(account.PrivateKey)
	if err != nil {
		return nil, err
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	tokenURL := strings.TrimSpace(cfg.TokenURL)
	if tokenURL == "" {
		tokenURL = strings.TrimSpace(account.TokenURI)
	}
	if tokenURL == "" {
		tokenURL = fcmDefaultTokenURL
	}
	sendURL := strings.TrimSpace(cfg.SendURL)
	if sendURL == "" {
		sendURL = fmt.Sprintf(fcmDefaultSendURL, strings.TrimSpace(cfg.ProjectID))
	}
	return &fcmProvider{
		projectID: strings.TrimSpace(cfg.ProjectID),
		account:   account,
		signer:    signer,
		client:    &http.Client{Timeout: timeout},
		tokenURL:  tokenURL,
		sendURL:   sendURL,
		now:       time.Now,
	}, nil
}

// parseServiceAccountKey 解析 PEM 私钥。
func parseServiceAccountKey(raw string) (crypto.Signer, error) {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, fmt.Errorf("%w: private_key is not PEM", ErrFCMBadAccount)
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		signer, ok := key.(crypto.Signer)
		if !ok {
			return nil, fmt.Errorf("%w: private key is not a signer", ErrFCMBadAccount)
		}
		return signer, nil
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("%w: unsupported private key format", ErrFCMBadAccount)
}

// Name 通道名。
func (p *fcmProvider) Name() string { return PushProviderFCM }

// Supports FCM 只服务 Android（iOS 走 APNs，见文件头说明）。
func (p *fcmProvider) Supports(platform string) bool {
	return strings.EqualFold(strings.TrimSpace(platform), pushPlatformFCM)
}

// Send 投递一条推送。
func (p *fcmProvider) Send(ctx context.Context, msg PushMessage) error {
	if strings.TrimSpace(msg.Token) == "" {
		return &PushTerminalError{Reason: "push token is empty"}
	}
	token, err := p.accessToken(ctx)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]interface{}{
		"message": map[string]interface{}{
			"token": msg.Token,
			"notification": map[string]string{
				"title": msg.Title,
				"body":  msg.Body,
			},
			"data": fcmStringData(msg.Data),
		},
	})
	if err != nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "push payload could not be encoded")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.sendURL, strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		// 网络层失败属于可重试：对方可能只是暂时不可达。
		return fmt.Errorf("%w: %v", ErrFCMSendFailed, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		// 可重试（见文件头注意事项 2）。
		return fmt.Errorf("%w: status %d: %s", ErrFCMSendFailed, resp.StatusCode, strings.TrimSpace(string(body)))
	default:
		return &PushTerminalError{Reason: fmt.Sprintf("%s: status %d: %s", ErrFCMSendFailed, resp.StatusCode, strings.TrimSpace(string(body)))}
	}
}

// fcmStringData FCM 的 data 字段只接受字符串值；
// 非字符串在这里转成字符串，而不是让整个请求被 FCM 400 拒掉。
func fcmStringData(data map[string]interface{}) map[string]string {
	out := make(map[string]string, len(data))
	for k, v := range data {
		switch value := v.(type) {
		case string:
			out[k] = value
		case nil:
			out[k] = ""
		default:
			raw, err := json.Marshal(value)
			if err != nil {
				out[k] = fmt.Sprint(value)
				continue
			}
			out[k] = string(raw)
		}
	}
	return out
}

// accessToken 取有效的访问令牌，过期前复用。
func (p *fcmProvider) accessToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	if p.token != "" && now.Before(p.tokenExp) {
		return p.token, nil
	}
	assertion, err := p.signJWT(now)
	if err != nil {
		return "", err
	}
	form := url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {assertion},
	}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL, strings.NewReader(form))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrFCMTokenExchange, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("%w: status %d: %s", ErrFCMTokenExchange, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil || tokenResp.AccessToken == "" {
		return "", fmt.Errorf("%w: %s", ErrFCMResponseInvalid, strings.TrimSpace(string(body)))
	}
	expiresIn := tokenResp.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	p.token = tokenResp.AccessToken
	// 提前 60 秒过期：把"刚好在请求途中过期"这个竞态挤掉。
	p.tokenExp = now.Add(time.Duration(expiresIn)*time.Second - 60*time.Second)
	return p.token, nil
}

// signJWT 签一个 OAuth2 jwt-bearer 断言。
func (p *fcmProvider) signJWT(now time.Time) (string, error) {
	header := map[string]string{"alg": "RS256", "typ": "JWT"}
	claims := map[string]interface{}{
		"iss":   p.account.ClientEmail,
		"scope": fcmOAuthScope,
		"aud":   p.tokenURL,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}
	headerRaw, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsRaw, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signingInput := base64.RawURLEncoding.EncodeToString(headerRaw) + "." + base64.RawURLEncoding.EncodeToString(claimsRaw)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := p.signer.Sign(rand.Reader, digest[:], crypto.SHA256)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrFCMBadAccount, err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}
