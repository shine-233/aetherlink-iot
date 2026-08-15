package app

import (
	"aetherlink-iot/backend/initialize/croninit"

	"github.com/sirupsen/logrus"
)

type CronService struct {
	initialized bool
}

func NewCronService() *CronService {
	return &CronService{}
}

func (s *CronService) Name() string {
	return "cron"
}

func (s *CronService) Start() error {
	logrus.Info("starting cron service")
	croninit.CronInit()
	s.initialized = true
	logrus.Info("cron service started")
	return nil
}

func (s *CronService) Stop() error {
	if !s.initialized {
		return nil
	}

	logrus.Info("stopping cron service")
	croninit.Stop()
	s.initialized = false
	logrus.Info("cron service stopped")
	return nil
}

func WithCronService() Option {
	return func(app *Application) error {
		app.RegisterService(NewCronService())
		return nil
	}
}
