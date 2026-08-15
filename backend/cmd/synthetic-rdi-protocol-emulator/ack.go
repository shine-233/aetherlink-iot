package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

type ACKMode string

const (
	ACKSuccess  ACKMode = "success"
	ACKFailure  ACKMode = "failure"
	ACKFailOnce ACKMode = "fail-once"
)

// ACKResponder is independent from the MQTT transport.  This lets tests
// exercise success, failure, and retry behavior without racing a second
// response publisher or writing through the backend simulation endpoint.
type ACKResponder struct {
	mu       sync.Mutex
	mode     ACKMode
	attempts map[string]int
}

func NewACKResponder(mode ACKMode) (*ACKResponder, error) {
	switch mode {
	case ACKSuccess, ACKFailure, ACKFailOnce:
		return &ACKResponder{mode: mode, attempts: make(map[string]int)}, nil
	default:
		return nil, fmt.Errorf("unsupported ACK mode %q; use success, failure, or fail-once", mode)
	}
}

func (r *ACKResponder) Respond(identity SyntheticIdentity, command Frame, timestamp int64, sequence int) (Frame, error) {
	if r == nil {
		return Frame{}, fmt.Errorf("ACK responder is nil")
	}
	if err := validateReplayFrame(command); err != nil {
		return Frame{}, err
	}
	var request commandPayload
	if err := json.Unmarshal(command.Payload, &request); err != nil || strings.TrimSpace(request.Method) == "" {
		return Frame{}, fmt.Errorf("command payload must contain a method")
	}

	messageID, err := commandTopicMessageID(command.Topic, identity.PID)
	if err != nil {
		return Frame{}, err
	}

	r.mu.Lock()
	r.attempts[messageID]++
	attempt := r.attempts[messageID]
	mode := r.mode
	r.mu.Unlock()

	success := mode == ACKSuccess || (mode == ACKFailOnce && attempt > 1)
	return BuildACKFrame(identity, command, success, timestamp, sequence)
}
