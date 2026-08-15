package app

import (
	"sync"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/pkg/metrics"

	"github.com/sirupsen/logrus"
)

type TelemetryService struct {
	logger   *logrus.Logger
	stopCh   chan struct{}
	runMu    sync.Mutex
	stopOnce sync.Once
}

func NewTelemetryService() *TelemetryService {
	return &TelemetryService{
		logger: logrus.StandardLogger(),
	}
}

func (s *TelemetryService) Name() string {
	return "telemetry-reporting"
}

func (s *TelemetryService) Start() error {
	if !metrics.TelemetryEnabled() {
		return nil
	}

	s.runMu.Lock()
	defer s.runMu.Unlock()

	if s.stopCh != nil {
		return nil
	}

	stopCh := make(chan struct{})
	s.stopCh = stopCh
	s.stopOnce = sync.Once{}

	go s.run(stopCh)
	return nil
}

func (s *TelemetryService) run(stopCh <-chan struct{}) {
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	select {
	case <-timer.C:
		s.reportTelemetry("startup")
	case <-stopCh:
		return
	}

	ticker := time.NewTicker(metrics.HeartbeatInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.reportTelemetry("heartbeat")
		case <-stopCh:
			return
		}
	}
}

func (s *TelemetryService) Stop() error {
	s.runMu.Lock()
	defer s.runMu.Unlock()

	s.stopOnce.Do(func() {
		if s.stopCh != nil {
			close(s.stopCh)
			s.stopCh = nil
		}
	})
	return nil
}

func WithTelemetry() Option {
	return func(a *Application) error {
		a.RegisterService(NewTelemetryService())
		return nil
	}
}

func (s *TelemetryService) reportTelemetry(trigger string) {
	s.runMu.Lock()
	defer s.runMu.Unlock()

	ins := metrics.NewInstance()
	ins.RefreshRuntimeInfo()
	ins.DeviceCount = dal.GetDevicesCount()
	ins.UserCount = dal.GetUsersCount()

	if err := metrics.ReportTelemetryCycle(ins, trigger); err != nil {
		s.logger.Debugf("telemetry report skipped or failed: %v", err)
		return
	}

	s.logger.Infof("telemetry report sent: trigger=%s", trigger)
}
