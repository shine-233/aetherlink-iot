package uplink

import "testing"

func TestStatusMetadataSourceNormalizesMissingOrInvalidSource(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]interface{}
		want     string
	}{
		{name: "missing metadata", metadata: nil, want: "unknown"},
		{name: "missing source", metadata: map[string]interface{}{}, want: "unknown"},
		{name: "non string source", metadata: map[string]interface{}{"source": 123}, want: "unknown"},
		{name: "empty source", metadata: map[string]interface{}{"source": ""}, want: "unknown"},
		{name: "status message source", metadata: map[string]interface{}{"source": "status_message"}, want: "status_message"},
		{name: "heartbeat source", metadata: map[string]interface{}{"source": "heartbeat_expired"}, want: "heartbeat_expired"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := statusMetadataSource(tt.metadata); got != tt.want {
				t.Fatalf("statusMetadataSource() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHeartbeatManagedStatusAcceptsOnlyHeartbeatExpiredSource(t *testing.T) {
	tests := []struct {
		source string
		want   bool
	}{
		{source: "heartbeat_expired", want: true},
		{source: "status_message", want: false},
		{source: "timeout_expired", want: false},
		{source: "unknown", want: false},
		{source: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			if got := isHeartbeatExpiredStatusSource(tt.source); got != tt.want {
				t.Fatalf("isHeartbeatExpiredStatusSource(%q) = %v, want %v", tt.source, got, tt.want)
			}
		})
	}
}
