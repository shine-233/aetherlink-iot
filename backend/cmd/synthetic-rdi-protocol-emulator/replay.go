package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
)

type ReplaySession struct {
	SchemaVersion  string  `json:"schema_version"`
	Classification string  `json:"classification"`
	Provenance     string  `json:"provenance"`
	Protocol       string  `json:"protocol"`
	PID            string  `json:"pid"`
	DeviceID       string  `json:"device_id"`
	Frames         []Frame `json:"frames"`
}

func NewReplaySession(identity SyntheticIdentity, frames []Frame) (ReplaySession, error) {
	if err := validateSyntheticIdentity(identity); err != nil {
		return ReplaySession{}, err
	}
	if len(frames) == 0 {
		return ReplaySession{}, fmt.Errorf("replay session requires at least one frame")
	}
	session := ReplaySession{
		SchemaVersion:  "synthetic-rdi-session-v1",
		Classification: ProtocolEmulatorTag,
		Provenance:     SyntheticMode,
		Protocol:       ObservedProtocol,
		PID:            identity.PID,
		DeviceID:       identity.DeviceID,
		Frames:         append([]Frame(nil), frames...),
	}
	if err := validateReplaySession(session); err != nil {
		return ReplaySession{}, err
	}
	return session, nil
}

func ReadReplaySession(reader io.Reader) (ReplaySession, error) {
	if reader == nil {
		return ReplaySession{}, fmt.Errorf("replay session reader is required")
	}
	var session ReplaySession
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&session); err != nil {
		return ReplaySession{}, fmt.Errorf("decode replay session: %w", err)
	}
	if err := validateReplaySession(session); err != nil {
		return ReplaySession{}, err
	}
	return session, nil
}

func validateReplaySession(session ReplaySession) error {
	if session.SchemaVersion != "synthetic-rdi-session-v1" {
		return fmt.Errorf("unsupported replay schema %q", session.SchemaVersion)
	}
	if session.Classification != ProtocolEmulatorTag {
		return fmt.Errorf("replay classification must be %q", ProtocolEmulatorTag)
	}
	if session.Provenance != SyntheticMode {
		return fmt.Errorf("replay accepts only provenance %q, got %q", SyntheticMode, session.Provenance)
	}
	if session.Protocol != ObservedProtocol || session.PID == "" || session.DeviceID == "" {
		return fmt.Errorf("replay session protocol or identity is incomplete")
	}
	if len(session.Frames) == 0 {
		return fmt.Errorf("replay session requires at least one frame")
	}
	for index, frame := range session.Frames {
		if err := validateReplayFrame(frame); err != nil {
			return fmt.Errorf("frame %d: %w", index+1, err)
		}
		if frame.Sequence != index+1 {
			return fmt.Errorf("frame %d has sequence %d; expected %d", index+1, frame.Sequence, index+1)
		}
		if frame.Provenance != session.Provenance || frame.Protocol != session.Protocol || frame.PID != session.PID || frame.DeviceID != session.DeviceID {
			return fmt.Errorf("frame %d identity/provenance does not match session", index+1)
		}
	}
	return nil
}

// ReplayFrames deliberately takes a publisher callback.  The default CLI
// uses it for validation-only replay; the optional MQTT runner supplies a
// transport callback.  This keeps the replay semantics testable without any
// network or database side effects.
func ReplayFrames(ctx context.Context, session ReplaySession, publish func(Frame) error) error {
	if err := validateReplaySession(session); err != nil {
		return err
	}
	if publish == nil {
		return fmt.Errorf("replay publisher is required")
	}
	for _, frame := range session.Frames {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := publish(frame); err != nil {
			return fmt.Errorf("publish replay frame %d: %w", frame.Sequence, err)
		}
	}
	return nil
}
