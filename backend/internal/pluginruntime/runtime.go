// Package pluginruntime defines the platform-side boundary for protocol and
// service plugin runtime operations. Callers depend on this package instead of
// binding business logic directly to an external HTTP adapter.
package pluginruntime

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"aetherlink-iot/backend/third_party/others/http_client"
)

// DeviceData is one device advertised by a service plugin.
type DeviceData struct {
	DeviceName     string `json:"device_name"`
	DeviceNumber   string `json:"device_number"`
	Description    string `json:"description"`
	IsBind         bool   `json:"is_bind"`
	DeviceConfigID string `json:"device_config_id"`
}

// DevicePage is a page of devices advertised by a service plugin.
type DevicePage struct {
	Total int          `json:"total"`
	List  []DeviceData `json:"list"`
}

// Runtime is the replaceable boundary for operations currently supplied by a
// protocol or service plugin process.
type Runtime interface {
	GetPluginForm(host, serviceIdentifier, deviceType, formType string) (interface{}, error)
	Notify(host, messageType, message string) ([]byte, error)
	ListServiceAccessDevices(host, voucher string, pageSize, page int) (*DevicePage, error)
	DisconnectDevice(host, deviceID string) error
}

var ErrDisabled = errors.New("plugin runtime disabled")

// IsDisabled reports whether an operation was blocked because the optional
// external plugin runtime is not enabled.
func IsDisabled(err error) bool {
	return errors.Is(err, ErrDisabled)
}

type disabledRuntime struct{}

func (disabledRuntime) GetPluginForm(string, string, string, string) (interface{}, error) {
	return nil, ErrDisabled
}

func (disabledRuntime) Notify(string, string, string) ([]byte, error) {
	return nil, ErrDisabled
}

func (disabledRuntime) ListServiceAccessDevices(string, string, int, int) (*DevicePage, error) {
	return nil, ErrDisabled
}

func (disabledRuntime) DisconnectDevice(string, string) error {
	return ErrDisabled
}

type remoteHTTPRuntime struct{}

// RemoteHTTP enables the optional external HTTP plugin runtime explicitly.
func RemoteHTTP() Runtime {
	return remoteHTTPRuntime{}
}

func (remoteHTTPRuntime) GetPluginForm(host, serviceIdentifier, deviceType, formType string) (interface{}, error) {
	return http_client.GetPluginFromConfigV2(host, serviceIdentifier, deviceType, formType)
}

func (remoteHTTPRuntime) Notify(host, messageType, message string) ([]byte, error) {
	return http_client.Notification(messageType, message, host)
}

func (remoteHTTPRuntime) ListServiceAccessDevices(host, voucher string, pageSize, page int) (*DevicePage, error) {
	data, err := http_client.GetServiceAccessDeviceList(host, voucher, fmt.Sprint(pageSize), fmt.Sprint(page))
	if err != nil {
		return nil, err
	}
	result := &DevicePage{Total: data.Total, List: make([]DeviceData, len(data.List))}
	for index, device := range data.List {
		result.List[index] = DeviceData{
			DeviceName: device.DeviceName, DeviceNumber: device.DeviceNumber,
			Description: device.Description, IsBind: device.IsBind,
			DeviceConfigID: device.DeviceConfigID,
		}
	}
	return result, nil
}

func (remoteHTTPRuntime) DisconnectDevice(host, deviceID string) error {
	payload, err := json.Marshal(struct {
		DeviceID string `json:"device_id"`
	}{DeviceID: deviceID})
	if err != nil {
		return err
	}
	response, err := http_client.DisconnectDevice(payload, host)
	if err != nil {
		return err
	}
	if response == nil {
		return fmt.Errorf("plugin disconnect returned no response")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("plugin disconnect failed with HTTP status %d", response.StatusCode)
	}
	var result http_client.RspData
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode plugin disconnect response: %w", err)
	}
	if result.Code != http.StatusOK {
		return fmt.Errorf("plugin disconnect failed: %s", result.Message)
	}
	return nil
}

var runtimeState = struct {
	sync.RWMutex
	value Runtime
}{value: disabledRuntime{}}

// Current returns the configured plugin runtime.
func Current() Runtime {
	runtimeState.RLock()
	defer runtimeState.RUnlock()
	return runtimeState.value
}

// Set replaces the runtime and returns a restore function. It is intended for
// application composition and tests; nil installs the disabled local default.
func Set(runtime Runtime) func() {
	if runtime == nil {
		runtime = disabledRuntime{}
	}
	runtimeState.Lock()
	previous := runtimeState.value
	runtimeState.value = runtime
	runtimeState.Unlock()
	return func() {
		runtimeState.Lock()
		runtimeState.value = previous
		runtimeState.Unlock()
	}
}
