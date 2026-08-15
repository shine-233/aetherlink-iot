package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/go-basic/uuid"
)

// Version 2 separates occurrence identity from the payload fingerprint. This
// prevents equal same-millisecond events from being collapsed while preserving
// idempotent replay when a protocol source identity is present.
const attributeEventEnvelopeVersion = 2

// attributeEventEnvelope is the only representation written to the
// attribute/event spool. Payload is canonical JSON: a key-sorted point array
// for attributes, or an identify/data object for events.
type attributeEventEnvelope struct {
	Version     int             `json:"version"`
	Identity    string          `json:"message_id"`
	Fingerprint string          `json:"fingerprint"`
	DeviceID    string          `json:"device_id"`
	TenantID    string          `json:"tenant_id"`
	Kind        DataType        `json:"kind"`
	Timestamp   int64           `json:"timestamp"`
	Payload     json.RawMessage `json:"payload"`
}

type attributeEventFingerprintMaterial struct {
	Version  int             `json:"version"`
	Identity string          `json:"message_id"`
	DeviceID string          `json:"device_id"`
	TenantID string          `json:"tenant_id"`
	Kind     DataType        `json:"kind"`
	Payload  json.RawMessage `json:"payload"`
}

type canonicalAttributePoint struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

type canonicalEventPayload struct {
	Identify string          `json:"identify"`
	Data     json.RawMessage `json:"data"`
}

func buildAttributeEventEnvelope(msg *Message) (attributeEventEnvelope, error) {
	if msg == nil {
		return attributeEventEnvelope{}, fmt.Errorf("attribute/event message is nil")
	}
	if strings.TrimSpace(msg.DeviceID) == "" {
		return attributeEventEnvelope{}, fmt.Errorf("attribute/event device_id is empty")
	}
	if strings.TrimSpace(msg.TenantID) == "" {
		return attributeEventEnvelope{}, fmt.Errorf("attribute/event tenant_id is empty")
	}
	if msg.Timestamp <= 0 {
		return attributeEventEnvelope{}, fmt.Errorf("attribute/event timestamp must be positive")
	}

	var payload json.RawMessage
	var err error
	switch msg.DataType {
	case DataTypeAttribute:
		payload, err = canonicalAttributePayload(msg.Data)
	case DataTypeEvent:
		payload, err = canonicalEventDataPayload(msg.Data)
	case DataTypeTelemetry:
		return attributeEventEnvelope{}, fmt.Errorf("telemetry is not accepted by attribute/event durable input")
	default:
		return attributeEventEnvelope{}, fmt.Errorf("unsupported attribute/event data type %q", msg.DataType)
	}
	if err != nil {
		return attributeEventEnvelope{}, err
	}

	messageID, err := resolveAttributeEventMessageID(msg)
	if err != nil {
		return attributeEventEnvelope{}, err
	}
	envelope := attributeEventEnvelope{
		Version:   attributeEventEnvelopeVersion,
		Identity:  messageID,
		DeviceID:  strings.TrimSpace(msg.DeviceID),
		TenantID:  strings.TrimSpace(msg.TenantID),
		Kind:      msg.DataType,
		Timestamp: msg.Timestamp,
		Payload:   payload,
	}
	identity, fingerprint, err := attributeEventEnvelopeIdentity(envelope)
	if err != nil {
		return attributeEventEnvelope{}, err
	}
	envelope.Identity = identity
	envelope.Fingerprint = fingerprint
	return envelope, nil
}

func resolveAttributeEventMessageID(msg *Message) (string, error) {
	if msg == nil {
		return "", fmt.Errorf("attribute/event message is nil")
	}
	if existing := strings.ToLower(strings.TrimSpace(msg.MessageID)); existing != "" {
		if !isUUIDShapedAttributeEventID(existing) {
			return "", fmt.Errorf("attribute/event message_id must be UUID-shaped")
		}
		msg.MessageID = existing
		return existing, nil
	}

	var identity string
	if source := strings.TrimSpace(msg.SourceMessageID); source != "" {
		// Scope an opaque wire identity by target device and kind. One gateway
		// message can contain several child-device envelopes without collisions,
		// while an MQTT retransmission for the same target remains idempotent.
		sum := sha256.Sum256([]byte(source + "\x00" + strings.TrimSpace(msg.DeviceID) + "\x00" + string(msg.DataType)))
		identity = uuidShapedAttributeEventDigest(sum[:])
	} else {
		// No protocol identity means two equal events in the same millisecond may
		// still be two real occurrences. Generate once and write it back to the
		// message; never collapse them by timestamp/payload equality.
		identity = strings.ToLower(uuid.New())
	}
	if !isUUIDShapedAttributeEventID(identity) {
		return "", fmt.Errorf("failed to create UUID-shaped attribute/event message_id")
	}
	msg.MessageID = identity
	return identity, nil
}

func isUUIDShapedAttributeEventID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, char := range value {
		switch index {
		case 8, 13, 18, 23:
			if char != '-' {
				return false
			}
		default:
			if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
				return false
			}
		}
	}
	return true
}

func canonicalAttributePayload(data interface{}) (json.RawMessage, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal attribute data: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var decoded []canonicalAttributePoint
	if err := decoder.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("invalid attribute data format: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return nil, fmt.Errorf("invalid attribute data format: %w", err)
	}
	if len(decoded) == 0 {
		return nil, fmt.Errorf("attribute payload has no points")
	}
	seen := make(map[string]struct{}, len(decoded))
	for index := range decoded {
		decoded[index].Key = strings.TrimSpace(decoded[index].Key)
		if decoded[index].Key == "" {
			return nil, fmt.Errorf("attribute point %d has an empty key", index)
		}
		if _, duplicate := seen[decoded[index].Key]; duplicate {
			return nil, fmt.Errorf("attribute payload contains duplicate key %q", decoded[index].Key)
		}
		seen[decoded[index].Key] = struct{}{}
		canonicalValue, err := canonicalizeRawJSON(decoded[index].Value)
		if err != nil {
			return nil, fmt.Errorf("canonicalize attribute point %d (%q): %w", index, decoded[index].Key, err)
		}
		decoded[index].Value = canonicalValue
	}
	sortCanonicalAttributePoints(decoded)
	payload, err := json.Marshal(decoded)
	if err != nil {
		return nil, fmt.Errorf("marshal canonical attribute payload: %w", err)
	}
	return payload, nil
}

func sortCanonicalAttributePoints(points []canonicalAttributePoint) {
	for index := 1; index < len(points); index++ {
		for current := index; current > 0 && points[current].Key < points[current-1].Key; current-- {
			points[current], points[current-1] = points[current-1], points[current]
		}
	}
}

func canonicalEventDataPayload(data interface{}) (json.RawMessage, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal event data: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var decoded canonicalEventPayload
	if err := decoder.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("invalid event data format: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return nil, fmt.Errorf("invalid event data format: %w", err)
	}
	if strings.TrimSpace(decoded.Identify) == "" {
		return nil, fmt.Errorf("event identify is empty")
	}
	decoded.Identify = strings.TrimSpace(decoded.Identify)
	decoded.Data, err = canonicalizeRawJSON(decoded.Data)
	if err != nil {
		return nil, fmt.Errorf("canonicalize event data: %w", err)
	}
	payload, err := json.Marshal(decoded)
	if err != nil {
		return nil, fmt.Errorf("marshal canonical event payload: %w", err)
	}
	return payload, nil
}

func canonicalizeRawJSON(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("JSON value is empty")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if err := requireJSONEOF(decoder); err != nil {
		return nil, err
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return canonical, nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra interface{}
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

func validateAttributeEventEnvelope(envelope attributeEventEnvelope) (attributeEventEnvelope, error) {
	if envelope.Version != attributeEventEnvelopeVersion {
		return attributeEventEnvelope{}, fmt.Errorf("unsupported attribute/event envelope version %d", envelope.Version)
	}
	if strings.TrimSpace(envelope.DeviceID) == "" || strings.TrimSpace(envelope.TenantID) == "" {
		return attributeEventEnvelope{}, fmt.Errorf("attribute/event envelope is missing device or tenant identity")
	}
	if envelope.Timestamp <= 0 {
		return attributeEventEnvelope{}, fmt.Errorf("attribute/event envelope timestamp must be positive")
	}

	var canonicalPayload json.RawMessage
	var err error
	switch envelope.Kind {
	case DataTypeAttribute:
		var points []canonicalAttributePoint
		if unmarshalErr := json.Unmarshal(envelope.Payload, &points); unmarshalErr != nil {
			return attributeEventEnvelope{}, fmt.Errorf("decode attribute envelope payload: %w", unmarshalErr)
		}
		generic := make([]AttributeDataPoint, 0, len(points))
		for _, point := range points {
			generic = append(generic, AttributeDataPoint{Key: point.Key, Value: point.Value})
		}
		canonicalPayload, err = canonicalAttributePayload(generic)
	case DataTypeEvent:
		var payload canonicalEventPayload
		if unmarshalErr := json.Unmarshal(envelope.Payload, &payload); unmarshalErr != nil {
			return attributeEventEnvelope{}, fmt.Errorf("decode event envelope payload: %w", unmarshalErr)
		}
		canonicalPayload, err = canonicalEventDataPayload(EventData{Identify: payload.Identify, Data: payload.Data})
	default:
		return attributeEventEnvelope{}, fmt.Errorf("unsupported attribute/event envelope kind %q", envelope.Kind)
	}
	if err != nil {
		return attributeEventEnvelope{}, err
	}
	if !bytes.Equal(canonicalPayload, envelope.Payload) {
		return attributeEventEnvelope{}, fmt.Errorf("attribute/event envelope payload is not canonical")
	}
	wantIdentity, wantFingerprint, err := attributeEventEnvelopeIdentity(envelope)
	if err != nil {
		return attributeEventEnvelope{}, err
	}
	if !strings.EqualFold(envelope.Identity, wantIdentity) ||
		!strings.EqualFold(envelope.Fingerprint, wantFingerprint) {
		return attributeEventEnvelope{}, fmt.Errorf("attribute/event envelope identity collision or corruption")
	}
	return envelope, nil
}

func unmarshalAttributeEventEnvelope(payload []byte) (attributeEventEnvelope, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var envelope attributeEventEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return attributeEventEnvelope{}, err
	}
	if err := requireJSONEOF(decoder); err != nil {
		return attributeEventEnvelope{}, err
	}
	return validateAttributeEventEnvelope(envelope)
}

func messageFromAttributeEventEnvelope(envelope attributeEventEnvelope) (*Message, error) {
	validated, err := validateAttributeEventEnvelope(envelope)
	if err != nil {
		return nil, err
	}
	message := &Message{
		MessageID: validated.Identity,
		DeviceID:  validated.DeviceID,
		TenantID:  validated.TenantID,
		DataType:  validated.Kind,
		Timestamp: validated.Timestamp,
	}
	switch validated.Kind {
	case DataTypeAttribute:
		var canonical []canonicalAttributePoint
		if err := json.Unmarshal(validated.Payload, &canonical); err != nil {
			return nil, err
		}
		points := make([]AttributeDataPoint, 0, len(canonical))
		for _, point := range canonical {
			value, err := decodeCanonicalAttributeValue(point.Value)
			if err != nil {
				return nil, err
			}
			points = append(points, AttributeDataPoint{Key: point.Key, Value: value})
		}
		message.Data = points
	case DataTypeEvent:
		var payload canonicalEventPayload
		if err := json.Unmarshal(validated.Payload, &payload); err != nil {
			return nil, err
		}
		message.Data = EventData{
			Identify: payload.Identify,
			Data:     append(json.RawMessage(nil), payload.Data...),
		}
	default:
		return nil, fmt.Errorf("unsupported attribute/event envelope kind %q", validated.Kind)
	}
	return message, nil
}

func attributeEventEnvelopeIdentity(envelope attributeEventEnvelope) (string, string, error) {
	identity := strings.ToLower(strings.TrimSpace(envelope.Identity))
	if !isUUIDShapedAttributeEventID(identity) {
		return "", "", fmt.Errorf("attribute/event envelope message_id must be UUID-shaped")
	}
	// Timestamp is deliberately excluded. MQTT retries reuse the trusted source
	// identity but acquire a new adapter receive time; the first durable receipt
	// owns that time. Payload or target changes still alter the full fingerprint
	// and are rejected as an identity collision.
	material := attributeEventFingerprintMaterial{
		Version:  envelope.Version,
		Identity: identity,
		DeviceID: envelope.DeviceID,
		TenantID: envelope.TenantID,
		Kind:     envelope.Kind,
		Payload:  envelope.Payload,
	}
	canonical, err := json.Marshal(material)
	if err != nil {
		return "", "", fmt.Errorf("marshal attribute/event identity material: %w", err)
	}
	sum := sha256.Sum256(canonical)
	return identity, hex.EncodeToString(sum[:]), nil
}

func uuidShapedAttributeEventDigest(digest []byte) string {
	value := make([]byte, 16)
	copy(value, digest)
	value[6] = (value[6] & 0x0f) | 0x50
	value[8] = (value[8] & 0x3f) | 0x80
	hexValue := hex.EncodeToString(value)
	return fmt.Sprintf(
		"%s-%s-%s-%s-%s",
		hexValue[0:8],
		hexValue[8:12],
		hexValue[12:16],
		hexValue[16:20],
		hexValue[20:32],
	)
}

func deterministicAttributeEventRowID(scope, envelopeIdentity, key string) string {
	sum := sha256.Sum256([]byte(scope + "\x00" + envelopeIdentity + "\x00" + key))
	value := append([]byte(nil), sum[:16]...)
	// Mark the derived ID as an RFC 4122-shaped version-5/variant-1 UUID while
	// retaining SHA-256 as the identity source.
	value[6] = (value[6] & 0x0f) | 0x50
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf(
		"%x-%x-%x-%x-%x",
		value[0:4],
		value[4:6],
		value[6:8],
		value[8:10],
		value[10:16],
	)
}

func decodeCanonicalAttributeValue(raw json.RawMessage) (interface{}, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if err := requireJSONEOF(decoder); err != nil {
		return nil, err
	}
	if number, ok := value.(json.Number); ok {
		converted, err := number.Float64()
		if err != nil {
			return nil, err
		}
		return converted, nil
	}
	return value, nil
}
