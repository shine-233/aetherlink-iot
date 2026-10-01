package mqttdebug

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

// legacySelectSessionMessages is the pre-ring implementation, kept verbatim as
// the behavioural oracle for selectAfter.
func legacySelectSessionMessages(messages []Message, afterSequence int64, limit int) []Message {
	filtered := make([]Message, 0, len(messages))
	for _, message := range messages {
		if message.Sequence > afterSequence {
			filtered = append(filtered, message)
		}
	}
	if len(filtered) <= limit {
		return append([]Message(nil), filtered...)
	}
	if afterSequence <= 0 {
		return append([]Message(nil), filtered[len(filtered)-limit:]...)
	}
	return append([]Message(nil), filtered[:limit]...)
}

// legacyCapture reproduces the old shift-on-full capture buffer.
type legacyCapture struct {
	messages []Message
	capacity int
}

func (capture *legacyCapture) push(message Message) bool {
	if len(capture.messages) >= capture.capacity {
		copy(capture.messages, capture.messages[1:])
		capture.messages[len(capture.messages)-1] = message
		return true
	}
	capture.messages = append(capture.messages, message)
	return false
}

func sequences(messages []Message) []int64 {
	out := make([]int64, len(messages))
	for i, message := range messages {
		out[i] = message.Sequence
	}
	return out
}

func TestMessageRingKeepsNewestInOrderAcrossWraparound(t *testing.T) {
	ring := newMessageRing(5)
	evictions := 0
	for seq := int64(1); seq <= 23; seq++ {
		if ring.push(Message{Sequence: seq}) {
			evictions++
		}
		if want := min(int(seq), 5); ring.Len() != want {
			t.Fatalf("after %d pushes len=%d, want %d", seq, ring.Len(), want)
		}
	}
	if evictions != 18 {
		t.Fatalf("evictions=%d, want 18", evictions)
	}
	got := sequences(ring.selectAfter(0, 100))
	if want := []int64{19, 20, 21, 22, 23}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ring contents=%v, want %v", got, want)
	}
}

func TestMessageRingSelectAfterMatchesLegacySelector(t *testing.T) {
	const capacity = 7
	for pushes := 0; pushes <= 3*capacity+2; pushes++ {
		ring := newMessageRing(capacity)
		legacy := &legacyCapture{capacity: capacity}
		for seq := int64(1); seq <= int64(pushes); seq++ {
			message := Message{Sequence: seq, Payload: fmt.Sprintf("p%d", seq)}
			if ringEvicted, legacyEvicted := ring.push(message), legacy.push(message); ringEvicted != legacyEvicted {
				t.Fatalf("pushes=%d seq=%d eviction mismatch ring=%v legacy=%v", pushes, seq, ringEvicted, legacyEvicted)
			}
		}
		for after := int64(-1); after <= int64(pushes)+1; after++ {
			for _, limit := range []int{1, 2, 3, capacity - 1, capacity, capacity + 5} {
				name := fmt.Sprintf("pushes=%d/after=%d/limit=%d", pushes, after, limit)
				got := ring.selectAfter(after, limit)
				want := legacySelectSessionMessages(legacy.messages, after, limit)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("%s: got %v want %v", name, sequences(got), sequences(want))
				}
			}
		}
	}
}

func TestMessageRingSelectionIsCallerOwnedAndEmptyStaysNullInJSON(t *testing.T) {
	ring := newMessageRing(3)
	for seq := int64(1); seq <= 4; seq++ {
		ring.push(Message{Sequence: seq, Payload: "x"})
	}
	selected := ring.selectAfter(0, 3)
	selected[0].Payload = "mutated"
	if again := ring.selectAfter(0, 3); again[0].Payload != "x" {
		t.Fatalf("selection aliased ring storage: %q", again[0].Payload)
	}

	encoded, err := json.Marshal(Snapshot{Messages: ring.selectAfter(99, 3)})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if value, ok := decoded["messages"]; !ok || value != nil {
		t.Fatalf(`empty selection must encode as "messages": null like before, got %v`, value)
	}
}

func TestMessageRingReleaseDropsStorage(t *testing.T) {
	ring := newMessageRing(4)
	ring.push(Message{Sequence: 1})
	ring.release()
	if ring.Len() != 0 || ring.selectAfter(0, 4) != nil {
		t.Fatal("released ring must be empty")
	}
}

func BenchmarkCaptureAppendAtCapacity(b *testing.B) {
	const capacity = 200
	message := Message{Direction: "inbound", Topic: "devices/telemetry", Payload: `{"t":1}`}
	b.Run("ring", func(b *testing.B) {
		ring := newMessageRing(capacity)
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			message.Sequence = int64(i)
			ring.push(message)
		}
	})
	b.Run("legacy_shift", func(b *testing.B) {
		legacy := &legacyCapture{capacity: capacity}
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			message.Sequence = int64(i)
			legacy.push(message)
		}
	})
}

func BenchmarkSnapshotSelectIncremental(b *testing.B) {
	const capacity = 200
	ring := newMessageRing(capacity)
	legacy := &legacyCapture{capacity: capacity}
	for seq := int64(1); seq <= 3*capacity; seq++ {
		ring.push(Message{Sequence: seq})
		legacy.push(Message{Sequence: seq})
	}
	cursor := int64(3*capacity - 10) // typical poll: a few new messages
	b.Run("ring", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = ring.selectAfter(cursor, capacity)
		}
	})
	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = legacySelectSessionMessages(legacy.messages, cursor, capacity)
		}
	})
}
