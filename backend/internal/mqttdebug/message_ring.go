package mqttdebug

import "sort"

// messageRing is the bounded per-session capture log.
//
// It replaces the old "append, then copy(messages, messages[1:]) once full"
// slice, which cost O(capacity) per captured message after warm-up. Pushing is
// O(1) and never reallocates after the ring reaches capacity. Storage grows
// lazily up to capacity, so idle sessions do not pin capacity*sizeof(Message)
// up front.
//
// Invariant relied on by selectAfter: Sequence values are strictly increasing
// in logical (oldest -> newest) order, because the session assigns them under
// the same lock that pushes.
//
// messageRing is not safe for concurrent use; the owning session's mu guards it.
type messageRing struct {
	buf      []Message
	head     int // physical index of the oldest message once len(buf) == capacity
	capacity int
}

func newMessageRing(capacity int) messageRing {
	if capacity < 1 {
		capacity = 1
	}
	return messageRing{capacity: capacity}
}

// Len reports how many messages are retained.
func (ring *messageRing) Len() int { return len(ring.buf) }

// push appends message as the newest entry. It reports true when the oldest
// entry had to be evicted to make room.
func (ring *messageRing) push(message Message) (evicted bool) {
	if len(ring.buf) < ring.capacity {
		ring.buf = append(ring.buf, message)
		return false
	}
	ring.buf[ring.head] = message
	ring.head++
	if ring.head == ring.capacity {
		ring.head = 0
	}
	return true
}

// at returns the logical i-th message (0 = oldest).
func (ring *messageRing) at(i int) *Message {
	physical := ring.head + i
	if physical >= len(ring.buf) {
		physical -= len(ring.buf)
	}
	return &ring.buf[physical]
}

// selectAfter returns a fresh copy of the retained messages with
// Sequence > afterSequence, keeping the API's historic window semantics:
//
//   - afterSequence <= 0 (initial load): the newest `limit` messages;
//   - afterSequence > 0 (incremental poll): the oldest `limit` messages after
//     the cursor, so a client paging forward never skips entries.
//
// An empty selection returns nil, matching the previous implementation's JSON
// shape (`"messages": null`). Cost is O(log n) to find the cursor plus exactly
// one copy of the selected window.
func (ring *messageRing) selectAfter(afterSequence int64, limit int) []Message {
	total := len(ring.buf)
	if total == 0 || limit <= 0 {
		return nil
	}
	start := sort.Search(total, func(i int) bool { return ring.at(i).Sequence > afterSequence })
	matched := total - start
	if matched == 0 {
		return nil
	}
	if matched > limit {
		if afterSequence <= 0 {
			start = total - limit
		}
		matched = limit
	}
	out := make([]Message, matched)
	ring.copyRange(out, start)
	return out
}

// copyRange copies len(dst) logical messages starting at logical index start
// using at most two contiguous copies.
func (ring *messageRing) copyRange(dst []Message, start int) {
	physical := ring.head + start
	if physical >= len(ring.buf) {
		physical -= len(ring.buf)
	}
	n := copy(dst, ring.buf[physical:])
	if n < len(dst) {
		copy(dst[n:], ring.buf[:len(dst)-n])
	}
}

// release drops the backing storage of a closed session so lingering
// references (timers, in-flight callbacks) do not pin captured payloads.
func (ring *messageRing) release() {
	ring.buf = nil
	ring.head = 0
}
