package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/internal/storage"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type StorageServiceWrapper struct {
	storage           storage.Storage
	input             *storage.InputQueue
	producerInput     storage.DurableMessageInput
	ctx               context.Context
	cancel            context.CancelFunc
	channelBufferSize int
	stopOnce          sync.Once
	stopErr           error
}

const (
	storageShutdownTimeout   = 30 * time.Second
	storageInputCloseTimeout = 10 * time.Second
)

func (s *StorageServiceWrapper) Name() string {
	return "storage"
}

func (s *StorageServiceWrapper) Start() error {
	if s.input == nil {
		return fmt.Errorf("storage input is not initialized")
	}
	if err := s.storage.Start(s.ctx, s.input.Messages()); err != nil {
		return fmt.Errorf("failed to start storage service: %w", err)
	}
	return nil
}

func (s *StorageServiceWrapper) Stop() error {
	s.stopOnce.Do(func() {
		logrus.Info("stopping storage service")
		deadline := time.Now().Add(storageShutdownTimeout)
		if s.producerInput != nil {
			s.producerInput.StopAccepting()
		}
		if s.input != nil {
			closeCtx, closeCancel := context.WithTimeout(context.Background(), storageInputCloseTimeout)
			if err := s.input.Close(closeCtx); err != nil {
				s.stopErr = errors.Join(s.stopErr, fmt.Errorf("failed to close storage input: %w", err))
			}
			closeCancel()
		}
		if s.producerInput != nil {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				remaining = time.Second
			}
			waitCtx, waitCancel := context.WithTimeout(context.Background(), remaining)
			if err := s.producerInput.Wait(waitCtx); err != nil {
				s.stopErr = errors.Join(s.stopErr, fmt.Errorf("failed to drain storage durability fallbacks: %w", err))
			}
			waitCancel()
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			remaining = time.Second
		}
		if err := s.storage.Stop(remaining); err != nil {
			s.stopErr = errors.Join(s.stopErr, fmt.Errorf("failed to stop storage service: %w", err))
		}
		if s.cancel != nil {
			s.cancel()
		}
		logrus.Info("storage service stopped")
	})
	return s.stopErr
}

func WithStorageService() Option {
	return func(a *Application) error {
		config := storage.DefaultConfig()

		if viper.IsSet("storage.channel_buffer_size") {
			config.ChannelBufferSize = viper.GetInt("storage.channel_buffer_size")
		}
		if viper.IsSet("storage.telemetry_batch_size") {
			config.TelemetryBatchSize = viper.GetInt("storage.telemetry_batch_size")
		}
		if viper.IsSet("storage.telemetry_flush_interval") {
			config.TelemetryFlushInterval = viper.GetInt("storage.telemetry_flush_interval")
		}
		if viper.IsSet("storage.enable_metrics") {
			config.EnableMetrics = viper.GetBool("storage.enable_metrics")
		}
		if viper.IsSet("storage.telemetry_spool.enabled") {
			config.TelemetrySpoolEnabled = viper.GetBool("storage.telemetry_spool.enabled")
		}
		if viper.IsSet("storage.telemetry_spool.directory") {
			config.TelemetrySpoolDirectory = viper.GetString("storage.telemetry_spool.directory")
		}
		if viper.IsSet("storage.telemetry_spool.max_bytes") {
			config.TelemetrySpoolMaxBytes = viper.GetInt64("storage.telemetry_spool.max_bytes")
		}
		if viper.IsSet("storage.telemetry_spool.max_records") {
			config.TelemetrySpoolMaxRecords = viper.GetInt("storage.telemetry_spool.max_records")
		}
		if viper.IsSet("storage.telemetry_spool.max_record_bytes") {
			config.TelemetrySpoolMaxRecordBytes = viper.GetInt64("storage.telemetry_spool.max_record_bytes")
		}
		if viper.IsSet("storage.telemetry_spool.replay_interval") {
			config.TelemetrySpoolReplayInterval = viper.GetDuration("storage.telemetry_spool.replay_interval")
		}
		if viper.IsSet("storage.telemetry_spool.replay_batch_size") {
			config.TelemetrySpoolReplayBatchSize = viper.GetInt("storage.telemetry_spool.replay_batch_size")
		}
		if viper.IsSet("storage.telemetry_spool.replay_timeout") {
			config.TelemetrySpoolReplayTimeout = viper.GetDuration("storage.telemetry_spool.replay_timeout")
		}
		if viper.IsSet("storage.telemetry_write_ahead_spool_enabled") {
			config.TelemetryWriteAheadSpoolEnabled = viper.GetBool("storage.telemetry_write_ahead_spool_enabled")
		}
		if viper.IsSet("storage.attribute_event_spool.enabled") {
			config.AttributeEventSpoolEnabled = viper.GetBool("storage.attribute_event_spool.enabled")
		}
		if viper.IsSet("storage.attribute_event_spool.directory") {
			config.AttributeEventSpoolDirectory = viper.GetString("storage.attribute_event_spool.directory")
		}
		if viper.IsSet("storage.attribute_event_spool.max_bytes") {
			config.AttributeEventSpoolMaxBytes = viper.GetInt64("storage.attribute_event_spool.max_bytes")
		}
		if viper.IsSet("storage.attribute_event_spool.max_records") {
			config.AttributeEventSpoolMaxRecords = viper.GetInt("storage.attribute_event_spool.max_records")
		}
		if viper.IsSet("storage.attribute_event_spool.max_record_bytes") {
			config.AttributeEventSpoolMaxRecordBytes = viper.GetInt64("storage.attribute_event_spool.max_record_bytes")
		}
		if viper.IsSet("storage.attribute_event_spool.replay_interval") {
			config.AttributeEventSpoolReplayInterval = viper.GetDuration("storage.attribute_event_spool.replay_interval")
		}
		if viper.IsSet("storage.attribute_event_spool.replay_batch_size") {
			config.AttributeEventSpoolReplayBatchSize = viper.GetInt("storage.attribute_event_spool.replay_batch_size")
		}
		if viper.IsSet("storage.attribute_event_spool.replay_timeout") {
			config.AttributeEventSpoolReplayTimeout = viper.GetDuration("storage.attribute_event_spool.replay_timeout")
		}

		logrus.WithFields(logrus.Fields{
			"buffer":                                 config.ChannelBufferSize,
			"batch":                                  config.TelemetryBatchSize,
			"flush_ms":                               config.TelemetryFlushInterval,
			"metrics":                                config.EnableMetrics,
			"telemetry_spool_enabled":                config.TelemetrySpoolEnabled,
			"telemetry_spool_max_bytes":              config.TelemetrySpoolMaxBytes,
			"telemetry_spool_max_records":            config.TelemetrySpoolMaxRecords,
			"telemetry_spool_max_record_bytes":       config.TelemetrySpoolMaxRecordBytes,
			"telemetry_spool_replay_batch":           config.TelemetrySpoolReplayBatchSize,
			"telemetry_spool_replay_timeout":         config.TelemetrySpoolReplayTimeout,
			"telemetry_write_ahead_enabled":          config.TelemetryWriteAheadSpoolEnabled,
			"attribute_event_spool_enabled":          config.AttributeEventSpoolEnabled,
			"attribute_event_spool_max_bytes":        config.AttributeEventSpoolMaxBytes,
			"attribute_event_spool_max_records":      config.AttributeEventSpoolMaxRecords,
			"attribute_event_spool_max_record_bytes": config.AttributeEventSpoolMaxRecordBytes,
			"attribute_event_spool_replay_batch":     config.AttributeEventSpoolReplayBatchSize,
			"attribute_event_spool_replay_timeout":   config.AttributeEventSpoolReplayTimeout,
		}).Info("storage config loaded")
		logrus.Debugf(
			"storage telemetry spool directory=%s replay_interval=%s",
			config.TelemetrySpoolDirectory,
			config.TelemetrySpoolReplayInterval,
		)
		logrus.Debugf(
			"storage attribute/event spool directory=%s replay_interval=%s",
			config.AttributeEventSpoolDirectory,
			config.AttributeEventSpoolReplayInterval,
		)

		inputQueue := storage.NewInputQueue(config.ChannelBufferSize)
		storageService := storage.New(a.DB, a.Logger, config)
		if operator, ok := storageService.(storage.AttributeEventDeadLetterOperator); ok {
			service.SetAttributeEventDeadLetterOperator(operator)
		}
		producerInput := storage.NewDurableMessageEnqueuer(inputQueue, storageService)
		ctx, cancel := context.WithCancel(context.Background())

		wrapper := &StorageServiceWrapper{
			storage:           storageService,
			input:             inputQueue,
			producerInput:     producerInput,
			ctx:               ctx,
			cancel:            cancel,
			channelBufferSize: config.ChannelBufferSize,
		}

		a.RegisterService(wrapper)
		a.storageService = storageService
		a.storageInput = producerInput

		logrus.Info("storage service registered")
		return nil
	}
}

func (a *Application) GetStorageService() storage.Storage {
	return a.storageService
}

func (a *Application) GetStorageInput() storage.DurableMessageInput {
	if a.storageInput == nil {
		return nil
	}
	return a.storageInput
}
