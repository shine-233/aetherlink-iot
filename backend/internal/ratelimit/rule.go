// Package ratelimit 实现了对标 ThingsBoard 的分布式集群限流与多策略配额引擎。
// 支持复合时间窗口定义，例如 "100:1,1000:60"（1秒内最多100次突发，60秒内最多1000次持续请求）。
package ratelimit

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// RateLimitRule 定义单个窗口限流规则。
type RateLimitRule struct {
	Limit         int64         `json:"limit"`
	WindowSeconds int64         `json:"window_seconds"`
	Window        time.Duration `json:"-"`
}

// String 返回规则的字符串表达形式，例如 "100:1"。
func (r RateLimitRule) String() string {
	return fmt.Sprintf("%d:%d", r.Limit, r.WindowSeconds)
}

// ParseRateLimitRules 解析形如 "100:1,1000:60" 的规则字符串。
func ParseRateLimitRules(expr string) ([]RateLimitRule, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("rate limit expression cannot be empty")
	}

	parts := strings.Split(expr, ",")
	rules := make([]RateLimitRule, 0, len(parts))
	seenWindows := make(map[int64]bool)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		segments := strings.Split(part, ":")
		if len(segments) != 2 {
			return nil, fmt.Errorf("invalid rate limit rule format: %q, expected <limit>:<window_seconds>", part)
		}

		limit, err := strconv.ParseInt(strings.TrimSpace(segments[0]), 10, 64)
		if err != nil || limit <= 0 {
			return nil, fmt.Errorf("invalid rate limit value %q in %q: must be positive integer", segments[0], part)
		}

		windowSec, err := strconv.ParseInt(strings.TrimSpace(segments[1]), 10, 64)
		if err != nil || windowSec <= 0 {
			return nil, fmt.Errorf("invalid rate limit window %q in %q: must be positive integer seconds", segments[1], part)
		}

		if seenWindows[windowSec] {
			return nil, fmt.Errorf("duplicate rate limit window %d seconds in expression %q", windowSec, expr)
		}
		seenWindows[windowSec] = true

		rules = append(rules, RateLimitRule{
			Limit:         limit,
			WindowSeconds: windowSec,
			Window:        time.Duration(windowSec) * time.Second,
		})
	}

	if len(rules) == 0 {
		return nil, fmt.Errorf("no valid rate limit rules found in expression %q", expr)
	}

	// 规则按窗口时间升序排列（短窗口突发在前，长窗口平滑在后）
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].WindowSeconds < rules[j].WindowSeconds
	})

	return rules, nil
}

// FormatRateLimitRules 将规则切片序列化回标准规则字符串。
func FormatRateLimitRules(rules []RateLimitRule) string {
	if len(rules) == 0 {
		return ""
	}
	parts := make([]string, len(rules))
	for i, r := range rules {
		parts[i] = r.String()
	}
	return strings.Join(parts, ",")
}

// ValidateRateLimitString 校验规则字符串合法性。
func ValidateRateLimitString(expr string) error {
	_, err := ParseRateLimitRules(expr)
	return err
}
