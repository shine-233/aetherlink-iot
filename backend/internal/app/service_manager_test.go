// 文件用途：覆盖应用编排与服务管理的 Go 测试。
// 核心逻辑：验证配置加载、选项注入、服务启动停止和失败回滚等应用生命周期行为，主要围绕 type mockService、func (m *mockService) Name、func (m *mockService) Start、func (m *mockService) Stop 等声明展开。
// 关键注意事项：测试依赖服务生命周期契约，新增断言需避免引入真实外部服务。
// 重构建议：后续可沉淀统一的 mock service 和配置夹具，降低测试重复度。

package app

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

type mockService struct {
	name      string
	startErr  error
	stopErr   error
	started   int
	stopped   int
	stopTrace *[]string
}

func (m *mockService) Name() string {
	return m.name
}

func (m *mockService) Start() error {
	m.started++
	return m.startErr
}

func (m *mockService) Stop() error {
	m.stopped++
	if m.stopTrace != nil {
		*m.stopTrace = append(*m.stopTrace, m.name)
	}
	return m.stopErr
}

func TestServiceManagerStartsStopsAndWaitsRegisteredServices(t *testing.T) {
	var stopTrace []string
	first := &mockService{name: "first", stopTrace: &stopTrace}
	second := &mockService{name: "second", stopTrace: &stopTrace}
	manager := NewServiceManager()
	manager.RegisterService(first)
	manager.RegisterService(second)

	if err := manager.StartAll(); err != nil {
		t.Fatalf("StartAll returned error: %v", err)
	}
	if !manager.started {
		t.Fatal("StartAll should mark manager started")
	}
	if first.started != 1 || second.started != 1 {
		t.Fatalf("start counts first=%d second=%d, want 1/1", first.started, second.started)
	}
	if err := manager.StartAll(); err == nil {
		t.Fatal("StartAll should reject duplicate start")
	}

	manager.StopAll()
	manager.Wait()

	if manager.started {
		t.Fatal("StopAll should mark manager stopped")
	}
	if first.stopped != 1 || second.stopped != 1 {
		t.Fatalf("stop counts first=%d second=%d, want 1/1", first.stopped, second.stopped)
	}
	if !reflect.DeepEqual(stopTrace, []string{"second", "first"}) {
		t.Fatalf("StopAll order = %#v, want reverse registration order", stopTrace)
	}

	manager.StopAll()
	if first.stopped != 1 || second.stopped != 1 {
		t.Fatalf("StopAll while stopped should not call Stop again: first=%d second=%d", first.stopped, second.stopped)
	}
}

func TestServiceManagerRollsBackStartedServicesWhenLaterStartFails(t *testing.T) {
	var stopTrace []string
	first := &mockService{name: "first", stopTrace: &stopTrace}
	second := &mockService{name: "second", startErr: errors.New("start failed"), stopTrace: &stopTrace}
	third := &mockService{name: "third", stopTrace: &stopTrace}
	manager := NewServiceManager()
	manager.RegisterService(first)
	manager.RegisterService(second)
	manager.RegisterService(third)

	err := manager.StartAll()
	if err == nil {
		t.Fatal("StartAll expected error")
	}
	if first.started != 1 || second.started != 1 || third.started != 0 {
		t.Fatalf("start counts first=%d second=%d third=%d", first.started, second.started, third.started)
	}
	if first.stopped != 1 || second.stopped != 0 || third.stopped != 0 {
		t.Fatalf("rollback stop counts first=%d second=%d third=%d", first.stopped, second.stopped, third.stopped)
	}
	if !reflect.DeepEqual(stopTrace, []string{"first"}) {
		t.Fatalf("rollback stop order = %#v, want only first service", stopTrace)
	}
	if manager.started {
		t.Fatal("StartAll failure should not mark manager started")
	}
}

func TestHTTPServiceDefaultsSetConfigAndNoopStop(t *testing.T) {
	service := NewHTTPService()
	if service.Name() == "" {
		t.Fatal("HTTP service name should not be empty")
	}
	if service.config.Host != "localhost" || service.config.Port != "9999" {
		t.Fatalf("default HTTP config = %+v", service.config)
	}
	if service.config.ReadTimeout != 60*time.Second || service.config.WriteTimeout != 60*time.Second {
		t.Fatalf("default HTTP timeouts = %+v", service.config)
	}

	service.SetConfig("0.0.0.0", "8080", 3*time.Second, 4*time.Second, 5*time.Second)
	if service.config.Host != "0.0.0.0" || service.config.Port != "8080" {
		t.Fatalf("SetConfig host/port = %+v", service.config)
	}
	if service.config.ReadTimeout != 3*time.Second || service.config.WriteTimeout != 4*time.Second || service.config.ShutdownTimeout != 5*time.Second {
		t.Fatalf("SetConfig timeouts = %+v", service.config)
	}
	if err := service.Stop(); err != nil {
		t.Fatalf("Stop without server returned error: %v", err)
	}
}

func TestWithHTTPServiceRegistersService(t *testing.T) {
	app := &Application{ServiceManager: NewServiceManager()}
	if err := WithHTTPService()(app); err != nil {
		t.Fatalf("WithHTTPService returned error: %v", err)
	}
	if len(app.ServiceManager.services) != 1 {
		t.Fatalf("registered services = %d, want 1", len(app.ServiceManager.services))
	}
	if _, ok := app.ServiceManager.services[0].(*HTTPService); !ok {
		t.Fatalf("registered service = %T, want *HTTPService", app.ServiceManager.services[0])
	}
}
