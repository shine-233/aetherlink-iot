// 文件用途：零分配快路径与旧实现的等价性回归（TopicMatch / ValidUTF8 / ValidTopicName / 变长整数）。
// 方法：保留旧实现的参考副本，对边界用例与随机组合逐一比对结果。
package packets

import (
	"bytes"
	"math/rand"
	"testing"
	"unicode/utf8"
)

func refTopicMatch(topic, filter []byte) bool {
	if len(filter) == 0 || len(topic) == 0 {
		return false
	}
	if (filter[0] == '$' && topic[0] != '$') || (topic[0] == '$' && filter[0] != '$') {
		return false
	}
	tl := bytes.Split(topic, []byte("/"))
	fl := bytes.Split(filter, []byte("/"))
	for i, f := range fl {
		if i == len(fl)-1 && len(f) == 1 && f[0] == '#' {
			return true
		}
		if i >= len(tl) {
			return false
		}
		if !(len(f) == 1 && f[0] == '+') && !bytes.Equal(tl[i], f) {
			return false
		}
	}
	return len(tl) == len(fl)
}

func refValidUTF8(p []byte) bool {
	for len(p) > 0 {
		ru, size := utf8.DecodeRune(p)
		if ru >= 0 && ru <= 0x1f || ru >= 0x7f && ru <= 0x9f || ru == utf8.RuneError || !utf8.ValidRune(ru) {
			return false
		}
		p = p[size:]
	}
	return true
}

func refValidTopicName(mustUTF8 bool, p []byte) bool {
	for len(p) > 0 {
		ru, size := utf8.DecodeRune(p)
		if mustUTF8 && ru == utf8.RuneError {
			return false
		}
		if size == 1 && (p[0] == '+' || p[0] == '#') {
			return false
		}
		p = p[size:]
	}
	return true
}

func TestTopicMatchFastEquivalence(t *testing.T) {
	topics := []string{"a", "a/b", "a/b/c", "a/", "/a", "/", "//", "a//b", "$SYS/x", "$SYS", "b/a", "a/b/c/d", "+", "#"}
	filters := []string{"a", "a/b", "a/+", "+", "#", "a/#", "+/#", "+/+", "/+", "+/", "a/b/#", "$SYS/#", "#/a", "a/+/c", "//", "/#", "+/+/+", "a//b", ""}
	for _, tp := range topics {
		for _, f := range filters {
			got, want := TopicMatch([]byte(tp), []byte(f)), refTopicMatch([]byte(tp), []byte(f))
			if got != want {
				t.Errorf("TopicMatch(%q,%q)=%v want %v", tp, f, got, want)
			}
		}
	}
	r := rand.New(rand.NewSource(1))
	alpha := []byte("ab/+#$")
	gen := func() []byte {
		b := make([]byte, r.Intn(7))
		for i := range b {
			b[i] = alpha[r.Intn(len(alpha))]
		}
		return b
	}
	for i := 0; i < 20000; i++ {
		tp, f := gen(), gen()
		if TopicMatch(tp, f) != refTopicMatch(tp, f) {
			t.Fatalf("TopicMatch(%q,%q) mismatch", tp, f)
		}
	}
}

func TestValidUTF8FastEquivalence(t *testing.T) {
	cases := [][]byte{nil, []byte("abc"), {0x00}, {0x1f}, {0x20}, {0x7e}, {0x7f}, []byte("温度"), {0xc2, 0x80}, {0xc2, 0x9f}, {0xc2, 0xa0}, {0xff}, {0xed, 0xa0, 0x80}, []byte("a+b"), []byte("a#")}
	r := rand.New(rand.NewSource(2))
	for i := 0; i < 20000; i++ {
		b := make([]byte, r.Intn(6))
		r.Read(b)
		cases = append(cases, b)
	}
	for _, c := range cases {
		if ValidUTF8(c) != refValidUTF8(c) {
			t.Fatalf("ValidUTF8(%x) mismatch", c)
		}
		for _, must := range []bool{true, false} {
			if ValidTopicName(must, c) != refValidTopicName(must, c) {
				t.Fatalf("ValidTopicName(%v,%x) mismatch", must, c)
			}
		}
	}
}

func TestRemainLengthRoundTrip(t *testing.T) {
	for _, n := range []int{0, 1, 127, 128, 16383, 16384, 2097151, 2097152, maxRemainLength} {
		b, err := DecodeRemainLength(n)
		if err != nil {
			t.Fatalf("encode %d: %v", n, err)
		}
		got, err := readVarInt(bytes.NewBuffer(b))
		if err != nil || got != n {
			t.Fatalf("roundtrip %d -> %x -> %d (%v)", n, b, got, err)
		}
		var fh bytes.Buffer
		if err := (&FixHeader{PacketType: PUBLISH, RemainLength: n}).Pack(&fh); err != nil || !bytes.Equal(fh.Bytes()[1:], b) {
			t.Fatalf("FixHeader.Pack %d: %x vs %x (%v)", n, fh.Bytes(), b, err)
		}
	}
	if _, err := DecodeRemainLength(maxRemainLength + 1); err == nil {
		t.Fatal("expected ErrMalformed for oversize length")
	}
}
