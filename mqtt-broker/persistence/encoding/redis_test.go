package encoding

import (
	"bytes"
	"testing"
	"time"

	"github.com/DrmagicE/gmqtt"
)

func TestSessionEncodingPreservesSignedConnectedAt(t *testing.T) {
	expected := &gmqtt.Session{
		ClientID:       "session-client",
		ConnectedAt:    time.Unix(-123, 0),
		ExpiryInterval: 42,
	}
	var encoded bytes.Buffer
	EncodeSession(expected, &encoded)

	actual, err := DecodeSession(bytes.NewBuffer(encoded.Bytes()))
	if err != nil {
		t.Fatalf("decode session: %v", err)
	}
	if actual.ClientID != expected.ClientID {
		t.Fatalf("client id = %q, want %q", actual.ClientID, expected.ClientID)
	}
	if actual.ConnectedAt.Unix() != expected.ConnectedAt.Unix() {
		t.Fatalf("connected timestamp = %d, want %d", actual.ConnectedAt.Unix(), expected.ConnectedAt.Unix())
	}
	if actual.ExpiryInterval != expected.ExpiryInterval {
		t.Fatalf("expiry interval = %d, want %d", actual.ExpiryInterval, expected.ExpiryInterval)
	}
}
