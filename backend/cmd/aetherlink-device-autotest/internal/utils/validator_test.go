package utils

import "testing"

func TestParseMessageFromTopicExtractsConcreteMessageID(t *testing.T) {
	tests := []struct {
		name    string
		topic   string
		pattern string
		want    string
	}{
		{
			name:    "direct attribute set",
			topic:   "devices/attributes/set/dev-001/msg-123",
			pattern: "devices/attributes/set/dev-001/+",
			want:    "msg-123",
		},
		{
			name:    "direct command",
			topic:   "devices/command/dev-001/cmd-456",
			pattern: "devices/command/dev-001/+",
			want:    "cmd-456",
		},
		{
			name:    "gateway attribute response",
			topic:   "gateway/attributes/response/gw-001/rsp-789",
			pattern: "gateway/attributes/response/gw-001/+",
			want:    "rsp-789",
		},
		{
			name:    "empty pattern falls back to last segment",
			topic:   "devices/event/evt-001",
			pattern: "",
			want:    "evt-001",
		},
		{
			name:    "hash suffix still validates fixed prefix",
			topic:   "devices/attributes/set/dev-001/msg-123",
			pattern: "devices/attributes/#",
			want:    "msg-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMessageFromTopic(tt.topic, tt.pattern)
			if err != nil {
				t.Fatalf("ParseMessageFromTopic returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("message_id = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseMessageFromTopicRejectsAmbiguousOrMismatchedTopics(t *testing.T) {
	tests := []struct {
		name    string
		topic   string
		pattern string
	}{
		{
			name:    "empty topic",
			topic:   "",
			pattern: "devices/attributes/set/dev-001/+",
		},
		{
			name:    "fixed segment mismatch",
			topic:   "devices/command/dev-001/msg-123",
			pattern: "devices/attributes/set/dev-001/+",
		},
		{
			name:    "missing wildcard segment",
			topic:   "devices/attributes/set/dev-001",
			pattern: "devices/attributes/set/dev-001/+",
		},
		{
			name:    "invalid hash position",
			topic:   "devices/attributes/set/dev-001/msg-123",
			pattern: "devices/#/set/+",
		},
		{
			name:    "wildcard is not a concrete message id",
			topic:   "devices/attributes/set/dev-001/+",
			pattern: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := ParseMessageFromTopic(tt.topic, tt.pattern); err == nil {
				t.Fatalf("ParseMessageFromTopic = %q, want error", got)
			}
		})
	}
}
