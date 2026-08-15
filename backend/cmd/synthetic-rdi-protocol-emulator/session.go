package main

func BuildSyntheticSession(identity SyntheticIdentity, ackMode ACKMode) (ReplaySession, error) {
	status, err := BuildStatusFrame(identity, true, 1700000100, 1)
	if err != nil {
		return ReplaySession{}, err
	}
	telemetry, err := BuildTelemetryFrame(identity, map[string]any{
		"temperature_1": 25.5,
		"humidity_1":    45.0,
	}, 1700000101, 2)
	if err != nil {
		return ReplaySession{}, err
	}
	command, err := BuildCommandFrame(identity, "synthetic-msg-001", "identify", map[string]any{"request": true}, 1700000102, 3)
	if err != nil {
		return ReplaySession{}, err
	}
	responder, err := NewACKResponder(ackMode)
	if err != nil {
		return ReplaySession{}, err
	}
	ack, err := responder.Respond(identity, command, 1700000103, 4)
	if err != nil {
		return ReplaySession{}, err
	}
	return NewReplaySession(identity, []Frame{status, telemetry, command, ack})
}
