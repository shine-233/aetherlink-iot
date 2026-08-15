package app

import (
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Service is the lifecycle contract for app-managed components.
type Service interface {
	Name() string
	Start() error
	Stop() error
}

// ServiceManager starts services in registration order and stops them in reverse.
type ServiceManager struct {
	services []Service
	wg       sync.WaitGroup
	mu       sync.Mutex
	started  bool
}

const serviceStopTimeout = 30 * time.Second

func NewServiceManager() *ServiceManager {
	return &ServiceManager{
		services: make([]Service, 0),
	}
}

func (m *ServiceManager) RegisterService(service Service) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.services = append(m.services, service)
	logrus.Infof("service %s registered", service.Name())
}

func (m *ServiceManager) StartAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return fmt.Errorf("services already started")
	}

	var started []Service
	for _, service := range m.services {
		logrus.Infof("starting service: %s", service.Name())
		if err := service.Start(); err != nil {
			logrus.Errorf("failed to start service %s: %v; rolling back started services", service.Name(), err)
			for i := len(started) - 1; i >= 0; i-- {
				if stopErr := started[i].Stop(); stopErr != nil {
					logrus.Errorf("failed to roll back service %s: %v", started[i].Name(), stopErr)
				}
				m.wg.Done()
			}
			return fmt.Errorf("failed to start service %s: %w", service.Name(), err)
		}
		m.wg.Add(1)
		started = append(started, service)
	}

	m.started = true
	return nil
}

func (m *ServiceManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	for i := len(m.services) - 1; i >= 0; i-- {
		service := m.services[i]
		logrus.Infof("stopping service: %s", service.Name())

		done := make(chan error, 1)
		go func(s Service) {
			done <- s.Stop()
			m.wg.Done()
		}(service)

		timer := time.NewTimer(serviceStopTimeout)
		select {
		case err := <-done:
			timer.Stop()
			if err != nil {
				logrus.Errorf("failed to stop service %s: %v", service.Name(), err)
			} else {
				logrus.Infof("service %s stopped", service.Name())
			}
		case <-timer.C:
			logrus.Warnf("stopping service %s timed out", service.Name())
		}
	}

	m.started = false
}

func (m *ServiceManager) Wait() {
	m.wg.Wait()
}
