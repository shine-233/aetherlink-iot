package uplink

import (
	"encoding/json"
	"reflect"
	"testing"
)

// The fast path must either decline (ok=false) or match encoding/json exactly.
func TestDecodeFlatJSONObjectMatchesEncodingJSON(t *testing.T) {
	cases := []struct {
		in     string
		wantOK bool
	}{
		{`{"voltage":220.5,"current":3,"online":true,"off":false,"fw":"1.2.3","n":null}`, true},
		{`  { "a" : -1.5e3 , "b":"中文" }  `, true},
		{`{}`, true},
		{`{"a":1,"a":2}`, true},
		{`{"a":"x:y","b":0}`, true},
		{`{"a":{"b":1}}`, false},
		{`{"a":[1,2]}`, false},
		{`{"a":"q\"uote"}`, false},
		{`{"a` + "\\" + `u0041":1}`, false}, // escaped key
		{"{\"a\":\"\xff\"}", false},
		{`{"a":1e999}`, false},
		{`[1,2]`, false},
		{`null`, false},
		{`"str"`, false},
		{`{"a":1,}`, false},
		{`{"a":01}`, false},
		{``, false},
	}
	for _, tc := range cases {
		got, ok := decodeFlatJSONObject([]byte(tc.in))
		if ok != tc.wantOK {
			t.Fatalf("%q: ok=%v, want %v", tc.in, ok, tc.wantOK)
		}
		if !ok {
			continue
		}
		var want map[string]interface{}
		if err := json.Unmarshal([]byte(tc.in), &want); err != nil {
			t.Fatalf("%q: fast path accepted input encoding/json rejects: %v", tc.in, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%q: got %#v, want %#v", tc.in, got, want)
		}
	}
}

func FuzzDecodeFlatJSONObject(f *testing.F) {
	for _, s := range []string{`{"a":1}`, `{"a":"b","c":true,"d":null}`, `{"a":{"b":1}}`, `{"x":-0.5E+2}`} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, in []byte) {
		got, ok := decodeFlatJSONObject(in)
		if !ok {
			return
		}
		var want map[string]interface{}
		if err := json.Unmarshal(in, &want); err != nil {
			t.Fatalf("%q accepted but encoding/json fails: %v", in, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%q: got %#v, want %#v", in, got, want)
		}
	})
}
