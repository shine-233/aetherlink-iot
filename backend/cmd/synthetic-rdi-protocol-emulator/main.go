package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	mode := flag.String("mode", "manifest", "manifest, session, replay, or device")
	seed := flag.String("seed", "deployment-prep", "deterministic seed for the synthetic identity")
	pid := flag.String("pid", "", "explicit synthetic PID override; must be 12 alphanumeric characters")
	deviceID := flag.String("device-id", "", "explicit synthetic database device ID override")
	broker := flag.String("broker", "", "MQTT broker host:port; network modes require -allow-network")
	replayPath := flag.String("replay-file", "", "JSON replay session file for replay mode")
	ackMode := flag.String("ack-mode", string(ACKSuccess), "success, failure, or fail-once")
	duration := flag.Duration("duration", 0, "optional device session duration; publishes an offline status before exit")
	allowNetwork := flag.Bool("allow-network", false, "explicitly allow a local MQTT connection in device/replay mode")
	username := flag.String("username", "", "optional MQTT username; defaults to the synthetic voucher username")
	password := flag.String("password", "", "optional MQTT password; never printed")
	flag.Parse()

	identity, err := GenerateSyntheticIdentity(*seed)
	if err != nil {
		fatal(err)
	}
	identity, err = OverrideSyntheticIdentity(identity, *pid, *deviceID)
	if err != nil {
		fatal(err)
	}

	parsedACKMode := ACKMode(strings.TrimSpace(*ackMode))
	switch strings.ToLower(strings.TrimSpace(*mode)) {
	case "manifest":
		writeJSON(identity.PublicManifest())
	case "session":
		session, sessionErr := BuildSyntheticSession(identity, parsedACKMode)
		if sessionErr != nil {
			fatal(sessionErr)
		}
		writeJSON(session)
	case "replay":
		if strings.TrimSpace(*replayPath) == "" {
			fatal(fmt.Errorf("-replay-file is required in replay mode"))
		}
		if err := runReplayFile(context.Background(), *replayPath, identity, *broker, *username, *password, *allowNetwork); err != nil {
			fatal(err)
		}
	case "device":
		if !*allowNetwork {
			fatal(fmt.Errorf("device mode is network-capable; pass -allow-network explicitly"))
		}
		if strings.TrimSpace(*broker) == "" {
			fatal(fmt.Errorf("-broker is required in device mode"))
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if *duration < 0 {
			fatal(fmt.Errorf("-duration cannot be negative"))
		}
		if *duration > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, *duration)
			defer cancel()
		}
		if err := runDevice(ctx, identity, *broker, *username, *password, parsedACKMode); err != nil {
			fatal(err)
		}
	default:
		fatal(fmt.Errorf("unsupported mode %q; use manifest, session, replay, or device", *mode))
	}
}

func writeJSON(value any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "synthetic-rdi-protocol-emulator: %v\n", err)
	os.Exit(1)
}

func runReplayFile(ctx context.Context, path string, identity SyntheticIdentity, broker, username, password string, allowNetwork bool) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open replay file: %w", err)
	}
	defer file.Close()
	session, err := ReadReplaySession(file)
	if err != nil {
		return err
	}
	if strings.TrimSpace(broker) == "" {
		writeJSON(map[string]any{
			"mode":           "replay-validation",
			"classification": session.Classification,
			"provenance":     session.Provenance,
			"protocol":       session.Protocol,
			"pid":            session.PID,
			"device_id":      session.DeviceID,
			"frames":         len(session.Frames),
			"network":        false,
		})
		return nil
	}
	if !allowNetwork {
		return fmt.Errorf("replay with -broker is network-capable; pass -allow-network explicitly")
	}
	if strings.TrimSpace(username) == "" {
		if session.PID != identity.PID || session.DeviceID != identity.DeviceID {
			return fmt.Errorf("replay identity does not match the generated PID %q; pass both -username and -password for an explicitly configured isolated test device", identity.PID)
		}
		username = identity.Voucher.Username
	}
	if strings.TrimSpace(password) == "" {
		if username == identity.Voucher.Username && session.PID == identity.PID && session.DeviceID == identity.DeviceID {
			password = identity.Voucher.Password
		} else {
			return fmt.Errorf("network replay requires both -username and -password when credentials are not the generated synthetic identity")
		}
	}
	client, err := connectMQTT(broker, session.PID, username, password)
	if err != nil {
		return err
	}
	defer client.Disconnect(250)
	return ReplayFrames(ctx, session, func(frame Frame) error {
		token := client.Publish(frame.Topic, 1, false, []byte(frame.Payload))
		if !token.WaitTimeout(10 * time.Second) {
			return fmt.Errorf("publish timeout on %s", frame.Topic)
		}
		return token.Error()
	})
}

func connectMQTT(broker string, pid, username, password string) (mqtt.Client, error) {
	broker = strings.TrimSpace(broker)
	if !strings.Contains(broker, "://") {
		broker = "tcp://" + broker
	}
	options := mqtt.NewClientOptions()
	options.AddBroker(broker)
	options.SetClientID("synthetic-rdi-" + strings.ToLower(pid))
	options.SetUsername(username)
	options.SetPassword(password)
	options.SetCleanSession(true)
	options.SetConnectTimeout(10 * time.Second)
	client := mqtt.NewClient(options)
	token := client.Connect()
	if !token.WaitTimeout(15 * time.Second) {
		return nil, fmt.Errorf("MQTT connection timeout to %s", broker)
	}
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("MQTT connection failed to %s: %w", broker, err)
	}
	return client, nil
}

func runDevice(ctx context.Context, identity SyntheticIdentity, broker, username, password string, ackMode ACKMode) error {
	if err := validateSyntheticIdentity(identity); err != nil {
		return err
	}
	if strings.TrimSpace(username) == "" {
		username = identity.Voucher.Username
	}
	if password == "" {
		password = identity.Voucher.Password
	}
	responder, err := NewACKResponder(ackMode)
	if err != nil {
		return err
	}
	client, err := connectMQTT(broker, identity.PID, username, password)
	if err != nil {
		return err
	}
	defer client.Disconnect(250)

	var sequenceMu sync.Mutex
	sequence := 0
	nextSequence := func() int {
		sequenceMu.Lock()
		defer sequenceMu.Unlock()
		sequence++
		return sequence
	}
	publish := func(frame Frame) error {
		token := client.Publish(frame.Topic, 1, false, []byte(frame.Payload))
		if !token.WaitTimeout(10 * time.Second) {
			return fmt.Errorf("publish timeout on %s", frame.Topic)
		}
		return token.Error()
	}
	errors := make(chan error, 1)
	commandTopic := "devices/command/" + identity.PID + "/+"
	subscribeToken := client.Subscribe(commandTopic, 1, func(_ mqtt.Client, message mqtt.Message) {
		parts := strings.Split(message.Topic(), "/")
		if len(parts) != 4 || parts[0] != "devices" || parts[1] != "command" || parts[2] != identity.PID || parts[3] == "" {
			return
		}
		command, commandErr := newFrame(identity, nextSequence(), "command", message.Topic(), message.Payload(), time.Now().Unix())
		if commandErr != nil {
			select {
			case errors <- commandErr:
			default:
			}
			return
		}
		ack, ackErr := responder.Respond(identity, command, time.Now().Unix(), nextSequence())
		if ackErr != nil {
			select {
			case errors <- ackErr:
			default:
			}
			return
		}
		if publishErr := publish(ack); publishErr != nil {
			select {
			case errors <- publishErr:
			default:
			}
		}
	})
	if !subscribeToken.WaitTimeout(10 * time.Second) {
		return fmt.Errorf("MQTT command subscription timeout")
	}
	if err := subscribeToken.Error(); err != nil {
		return fmt.Errorf("MQTT command subscription failed: %w", err)
	}

	online, err := BuildStatusFrame(identity, true, time.Now().Unix(), nextSequence())
	if err != nil {
		return err
	}
	if err := publish(online); err != nil {
		return err
	}
	telemetry, err := BuildTelemetryFrame(identity, map[string]any{"temperature_1": 25.5}, time.Now().Unix(), nextSequence())
	if err != nil {
		return err
	}
	if err := publish(telemetry); err != nil {
		return err
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			offline, offlineErr := BuildStatusFrame(identity, false, time.Now().Unix(), nextSequence())
			if offlineErr != nil {
				return offlineErr
			}
			return publish(offline)
		case err := <-errors:
			return err
		case <-ticker.C:
			refresh, refreshErr := BuildStatusFrame(identity, true, time.Now().Unix(), nextSequence())
			if refreshErr != nil {
				return refreshErr
			}
			if refreshErr = publish(refresh); refreshErr != nil {
				return refreshErr
			}
		}
	}
}
