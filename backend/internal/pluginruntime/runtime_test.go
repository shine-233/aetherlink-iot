package pluginruntime

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeRuntime struct{}

func (*fakeRuntime) GetPluginForm(string, string, string, string) (interface{}, error) {
	return nil, nil
}

func (*fakeRuntime) Notify(string, string, string) ([]byte, error) {
	return nil, nil
}

func (*fakeRuntime) ListServiceAccessDevices(string, string, int, int) (*DevicePage, error) {
	return nil, nil
}

func (*fakeRuntime) DisconnectDevice(string, string) error {
	return nil
}

func TestRemoteHTTPRuntimeGetPluginForm(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/form/config" {
			t.Errorf("path = %s", r.URL.Path)
		}
		query := r.URL.Query()
		if query.Get("protocol_type") != "CUSTOM" || query.Get("device_type") != "1" || query.Get("form_type") != "CFG" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"code":200,"data":[{"dataKey":"host"}]}`))
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	form, err := (remoteHTTPRuntime{}).GetPluginForm(host, "CUSTOM", "1", "CFG")
	if err != nil {
		t.Fatalf("GetPluginForm returned error: %v", err)
	}
	fields, ok := form.([]interface{})
	if !ok || len(fields) != 1 {
		t.Fatalf("form = %#v, want one field", form)
	}
}

func TestDefaultRuntimeIsDisabled(t *testing.T) {
	runtime := Current()
	sensitiveValues := []string{"plugin.internal", "voucher-secret", "message-secret", "device-secret"}

	_, formErr := runtime.GetPluginForm(sensitiveValues[0], "service-secret", "device-type-secret", "form-secret")
	_, notifyErr := runtime.Notify(sensitiveValues[0], "message-type-secret", sensitiveValues[2])
	_, listErr := runtime.ListServiceAccessDevices(sensitiveValues[0], sensitiveValues[1], 10, 1)
	disconnectErr := runtime.DisconnectDevice(sensitiveValues[0], sensitiveValues[3])

	for name, err := range map[string]error{
		"GetPluginForm":            formErr,
		"Notify":                   notifyErr,
		"ListServiceAccessDevices": listErr,
		"DisconnectDevice":         disconnectErr,
	} {
		if !IsDisabled(err) {
			t.Errorf("%s error = %v, want ErrDisabled", name, err)
		}
		for _, sensitive := range sensitiveValues {
			if strings.Contains(err.Error(), sensitive) {
				t.Errorf("%s error leaked sensitive input %q: %v", name, sensitive, err)
			}
		}
	}
}

func TestSetAndRestoreRuntime(t *testing.T) {
	original := Current()
	fake := &fakeRuntime{}
	restore := Set(fake)
	if Current() != fake {
		t.Fatal("Current did not return the injected runtime")
	}
	restore()
	if Current() != original {
		t.Fatal("restore did not reinstate the previous runtime")
	}
}

func TestSetNilInstallsDisabledRuntime(t *testing.T) {
	restore := Set(&fakeRuntime{})
	defer restore()

	restoreDisabled := Set(nil)
	if _, err := Current().GetPluginForm("host-secret", "service", "device", "form"); !IsDisabled(err) {
		t.Fatalf("GetPluginForm error = %v, want ErrDisabled", err)
	}
	restoreDisabled()
	if _, ok := Current().(*fakeRuntime); !ok {
		t.Fatalf("restore did not reinstate fake runtime: %T", Current())
	}
}

func TestRemoteHTTPFactory(t *testing.T) {
	var runtime Runtime = RemoteHTTP()
	if runtime == nil {
		t.Fatal("RemoteHTTP returned nil")
	}
	if _, ok := runtime.(remoteHTTPRuntime); !ok {
		t.Fatalf("RemoteHTTP returned %T", runtime)
	}
}

func TestRemoteHTTPRuntimeNotify(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/notify/event" {
			t.Errorf("path = %s", r.URL.Path)
		}
		var payload struct {
			MessageType string `json:"message_type"`
			Message     string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.MessageType != "1" || payload.Message != `{"service_access_id":"access-1"}` {
			t.Errorf("unexpected payload: %+v", payload)
		}
		_, _ = w.Write([]byte(`{"code":200}`))
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	body, err := (remoteHTTPRuntime{}).Notify(host, "1", `{"service_access_id":"access-1"}`)
	if err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}
	if string(body) != `{"code":200}` {
		t.Fatalf("body = %s", body)
	}
}

func TestRemoteHTTPRuntimeNotifyRejectsNonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	if _, err := (remoteHTTPRuntime{}).Notify(host, "1", "message"); err == nil {
		t.Fatal("Notify accepted a non-200 response")
	}
}

func TestRemoteHTTPRuntimeListServiceAccessDevices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/plugin/device/list" {
			t.Errorf("path = %s", r.URL.Path)
		}
		query := r.URL.Query()
		if query.Get("voucher") != "voucher-1" || query.Get("page_size") != "25" || query.Get("page") != "2" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"message":"ok","data":{"total":1,"list":[{"device_name":"Meter","device_number":"meter-1","description":"Main meter","is_bind":true,"device_config_id":"config-1"}]}}`))
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	page, err := (remoteHTTPRuntime{}).ListServiceAccessDevices(host, "voucher-1", 25, 2)
	if err != nil {
		t.Fatalf("ListServiceAccessDevices returned error: %v", err)
	}
	if page.Total != 1 || len(page.List) != 1 {
		t.Fatalf("unexpected page: %+v", page)
	}
	device := page.List[0]
	if device.DeviceName != "Meter" || device.DeviceNumber != "meter-1" || device.Description != "Main meter" || !device.IsBind || device.DeviceConfigID != "config-1" {
		t.Fatalf("unexpected device mapping: %+v", device)
	}
}

func TestRemoteHTTPRuntimeListNormalizesNilList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":200,"message":"ok","data":{"total":0,"list":null}}`))
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	page, err := (remoteHTTPRuntime{}).ListServiceAccessDevices(host, "voucher-1", 10, 1)
	if err != nil {
		t.Fatalf("ListServiceAccessDevices returned error: %v", err)
	}
	if page.List == nil || len(page.List) != 0 {
		t.Fatalf("list was not normalized: %#v", page.List)
	}
}

func TestRemoteHTTPRuntimeDisconnectDevice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/device/disconnect" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		var payload struct {
			DeviceID string `json:"device_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.DeviceID != "device-1" {
			t.Errorf("device_id = %s", payload.DeviceID)
		}
		_, _ = w.Write([]byte(`{"code":200,"message":"ok"}`))
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	if err := (remoteHTTPRuntime{}).DisconnectDevice(host, "device-1"); err != nil {
		t.Fatalf("DisconnectDevice returned error: %v", err)
	}
}

func TestRemoteHTTPRuntimeDisconnectDeviceErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{name: "business error", statusCode: http.StatusOK, body: `{"code":500,"message":"disconnect failed"}`},
		{name: "http error", statusCode: http.StatusBadGateway, body: `{"code":200,"message":"ok"}`},
		{name: "invalid json", statusCode: http.StatusOK, body: `{`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.statusCode)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()

			host := strings.TrimPrefix(server.URL, "http://")
			if err := (remoteHTTPRuntime{}).DisconnectDevice(host, "device-1"); err == nil {
				t.Fatal("DisconnectDevice accepted an invalid response")
			}
		})
	}
}

func TestRemoteHTTPRuntimeDisconnectDeviceTransportError(t *testing.T) {
	err := (remoteHTTPRuntime{}).DisconnectDevice("127.0.0.1:0", "device-1")
	if err == nil || errors.Is(err, nil) {
		t.Fatal("DisconnectDevice accepted a transport failure")
	}
}
