package calcfield

import (
	"encoding/json"
	"math"
	"math/rand"
	"strconv"
	"testing"
)

func referenceMarshalDerivedBatch(keys []string, values []interface{}) ([]byte, error) {
	batch := make(map[string]interface{}, len(keys))
	for i, key := range keys {
		batch[key] = values[i]
	}
	return json.Marshal(batch)
}

func TestMarshalDerivedBatchMatchesEncodingJSON(t *testing.T) {
	cases := []struct {
		keys   []string
		values []interface{}
	}{
		{[]string{"b", "a"}, []interface{}{1.5, true}},
		{[]string{"power", "power"}, []interface{}{1.0, 2.0}},
		{[]string{"z", "y", "x"}, []interface{}{"ok", false, 0.0}},
		{[]string{"a<b", "c"}, []interface{}{1.0, 2.0}},
		{[]string{"a", "b"}, []interface{}{"x&y", "中文"}},
		{[]string{"a", "b"}, []interface{}{int64(3), 1.0}},
		{[]string{"tiny", "huge", "neg"}, []interface{}{1e-7, 1e21, -1.25e-9}},
		{[]string{"x", "y"}, []interface{}{123456789012345678.0, 1e20}},
		{[]string{"nan", "b"}, []interface{}{math.NaN(), 1.0}},
		{[]string{"inf", "b"}, []interface{}{math.Inf(-1), 1.0}},
		{[]string{"k1", "k2", "k3", "k4", "k5", "k6", "k7", "k8", "k9", "k10"}, []interface{}{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0}},
	}
	check := func(keys []string, values []interface{}) {
		t.Helper()
		got, gotErr := marshalDerivedBatch(keys, values)
		want, wantErr := referenceMarshalDerivedBatch(keys, values)
		if (gotErr != nil) != (wantErr != nil) {
			t.Fatalf("%v %v: err %v, want %v", keys, values, gotErr, wantErr)
		}
		if string(got) != string(want) {
			t.Fatalf("%v %v:\n got %s\nwant %s", keys, values, got, want)
		}
	}
	for _, tc := range cases {
		check(tc.keys, tc.values)
	}

	r := rand.New(rand.NewSource(7))
	for iter := 0; iter < 5000; iter++ {
		n := 2 + r.Intn(6)
		keys := make([]string, n)
		values := make([]interface{}, n)
		for i := range keys {
			keys[i] = "k" + strconv.Itoa(r.Intn(5))
			switch r.Intn(3) {
			case 0:
				values[i] = math.Float64frombits(r.Uint64())
			case 1:
				values[i] = (r.Float64() - 0.5) * math.Pow(10, float64(r.Intn(50)-25))
			default:
				values[i] = r.Intn(2) == 0
			}
		}
		check(keys, values)
	}
}
