// 文件用途：统一 AI/LLM 公网出站边界，供自然语言查询、告警分析、助手和规则链复用。
// 安全语义：只允许解析到公网地址的 HTTPS 端点；禁止代理与重定向，拨号时重新校验并固定 IP。
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"

	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/safehttp"
)

const aiLLMMaxRequestBytes = 1024 * 1024

var (
	errAILLMInvalidConfiguration = errors.New("AI provider configuration is invalid")
	errAILLMOriginMismatch       = errors.New("AI provider credential origin mismatch")
	errAILLMRequestFailed        = errors.New("AI provider request failed")
	errAILLMResponseTooLarge     = errors.New("AI provider response is too large")
	errAILLMInvalidResponse      = errors.New("AI provider returned an invalid response")

	aiLLMResolver      safehttp.Resolver = net.DefaultResolver
	newAILLMHTTPClient                   = func(endpoint *url.URL, apiKey string) *http.Client {
		return buildAILLMHTTPClient(endpoint, apiKey, aiLLMResolver)
	}
)

type aiLLMChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func validateAILLMBaseURL(ctx context.Context, rawURL string) (*url.URL, error) {
	endpoint, _, err := safehttp.ValidateWebhookURL(ctx, strings.TrimSpace(rawURL), aiLLMResolver)
	if err != nil {
		return nil, fmt.Errorf("%w: endpoint is not an allowed public HTTPS URL", errAILLMInvalidConfiguration)
	}
	if endpoint.Scheme != "https" {
		return nil, fmt.Errorf("%w: endpoint must use HTTPS", errAILLMInvalidConfiguration)
	}
	if endpoint.RawQuery != "" {
		return nil, fmt.Errorf("%w: endpoint query parameters are not allowed", errAILLMInvalidConfiguration)
	}
	return endpoint, nil
}

func aiLLMChatCompletionsURL(ctx context.Context, baseURL string) (*url.URL, error) {
	endpoint, err := validateAILLMBaseURL(ctx, baseURL)
	if err != nil {
		return nil, err
	}
	endpoint.Path = path.Join(endpoint.Path, "chat/completions")
	endpoint.RawPath = ""
	return endpoint, nil
}

func aiLLMSameOrigin(left, right *url.URL) bool {
	if left == nil || right == nil {
		return false
	}
	return strings.EqualFold(left.Scheme, right.Scheme) &&
		strings.EqualFold(left.Host, right.Host)
}

func buildAILLMHTTPClient(endpoint *url.URL, apiKey string, resolver safehttp.Resolver) *http.Client {
	client := safehttp.NewWebhookClient(safehttp.WebhookClientOptions{Resolver: resolver})
	client.Transport = bindAILLMTransport(endpoint, apiKey, client.Transport)
	client.CheckRedirect = func(req *http.Request, _ []*http.Request) error {
		if !aiLLMSameOrigin(endpoint, req.URL) {
			return errAILLMOriginMismatch
		}
		return safehttp.ErrUnsafeWebhookURL
	}
	return client
}

func bindAILLMTransport(endpoint *url.URL, apiKey string, transport http.RoundTripper) http.RoundTripper {
	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if !aiLLMSameOrigin(endpoint, req.URL) {
			return nil, errAILLMOriginMismatch
		}
		expectedAuthorization := "Bearer " + apiKey
		if req.Header.Get("Authorization") != expectedAuthorization {
			return nil, errAILLMOriginMismatch
		}
		return transport.RoundTrip(req)
	})
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func aiChatCompletion(ctx context.Context, baseURL, apiKey, modelName string, msgs []aiLLMChatMessage, maxTokens int, temperature *float64) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return "", errcode.NewWithMessage(errcode.CodeParamError, "AI provider is not configured")
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.openai.com/v1"
	}
	endpoint, err := aiLLMChatCompletionsURL(ctx, baseURL)
	if err != nil {
		return "", errcode.NewWithMessage(errcode.CodeParamError, errAILLMInvalidConfiguration.Error())
	}
	body := map[string]any{"model": strings.TrimSpace(modelName), "messages": msgs}
	if maxTokens > 0 {
		body["max_tokens"] = maxTokens
	}
	if temperature != nil {
		body["temperature"] = *temperature
	}
	raw, err := json.Marshal(body)
	if err != nil || len(raw) > aiLLMMaxRequestBytes {
		return "", errcode.NewWithMessage(errcode.CodeParamError, "AI request is too large")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(raw))
	if err != nil {
		return "", errcode.NewWithMessage(errcode.CodeParamError, errAILLMInvalidConfiguration.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := newAILLMHTTPClient(endpoint, apiKey)
	client.Timeout = aiModelHTTPTimeout
	resp, err := client.Do(req)
	if err != nil {
		return "", errcode.NewWithMessage(errcode.CodeParamError, errAILLMRequestFailed.Error())
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := readBoundedAILLMResponse(resp.Body)
	if err != nil {
		return "", errcode.NewWithMessage(errcode.CodeParamError, err.Error())
	}
	if resp.StatusCode != http.StatusOK {
		return "", errcode.NewWithMessage(errcode.CodeParamError,
			fmt.Sprintf("AI provider returned HTTP %d", resp.StatusCode))
	}
	return parseAILLMChatResponse(respBody)
}

func readBoundedAILLMResponse(body io.Reader) ([]byte, error) {
	limited, err := io.ReadAll(io.LimitReader(body, aiModelMaxReplyChars+1))
	if err != nil {
		return nil, errAILLMInvalidResponse
	}
	if len(limited) > aiModelMaxReplyChars {
		return nil, errAILLMResponseTooLarge
	}
	return limited, nil
}

func parseAILLMChatResponse(raw []byte) (string, error) {
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed.Choices) == 0 {
		return "", errcode.NewWithMessage(errcode.CodeParamError, errAILLMInvalidResponse.Error())
	}
	return parsed.Choices[0].Message.Content, nil
}
