package uplink

import (
	"io"
	"reflect"
	"testing"

	"github.com/sirupsen/logrus"
)

// TestDecodeJSONObjectOrRaw 扁平对象走快路径、嵌套对象走 encoding/json、非对象包装为 {"_raw": ...}。
func TestDecodeJSONObjectOrRaw(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	cases := []struct {
		name    string
		payload string
		want    map[string]interface{}
	}{
		{"flat", `{"t":1.5,"on":true}`, map[string]interface{}{"t": 1.5, "on": true}},
		{"nested", `{"a":{"b":1}}`, map[string]interface{}{"a": map[string]interface{}{"b": float64(1)}}},
		{"array", `[1,2]`, map[string]interface{}{"_raw": parseRawJSONValue([]byte(`[1,2]`))}},
		{"invalid", `not json`, map[string]interface{}{"_raw": parseRawJSONValue([]byte(`not json`))}},
	}
	for _, tc := range cases {
		got := decodeJSONObjectOrRaw(logger, "dev-1", []byte(tc.payload), "wrap")
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %#v want %#v", tc.name, got, tc.want)
		}
	}
}
