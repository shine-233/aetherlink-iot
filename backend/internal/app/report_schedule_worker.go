package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/service"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	defaultReportWorkerInterval         = 15 * time.Second
	defaultReportWorkerLimit            = 20
	defaultReportWorkerConcurrency      = 4
	defaultReportWorkerMaxAttempts      = 3
	defaultReportWorkerLeaseTimeout     = 2 * time.Minute
	defaultReportWorkerOperationTimeout = time.Minute
	defaultReportWorkerShutdownTimeout  = 10 * time.Second
	maxReportWorkerLimit                = 100
	maxReportWorkerConcurrency          = 32
	reportGenerationLeaseExpiredCode    = "generation_lease_expired"
	reportDeliveryAcceptanceUnknownCode = "smtp_acceptance_unknown"
)

type reportGenerationClaim = dal.ReportGenerationClaim
type reportDeliveryClaim = dal.ReportDeliveryClaim

type reportWorkerOperations struct {
	initialize      func(context.Context, int, dal.ReportNextOccurrence) (dal.ReportScheduleInitializationResult, error)
	dispatch        func(context.Context, int, int, dal.ReportNextOccurrence) ([]dal.ReportMaterializationResult, error)
	claimGeneration func(context.Context, int, time.Duration) ([]reportGenerationClaim, error)
	renewGeneration func(context.Context, string, string, time.Duration) (time.Time, error)
	generate        func(context.Context, reportGenerationClaim) error
	reapGeneration  func(context.Context, string) (int64, error)
	claimDelivery   func(context.Context, int, time.Duration) ([]reportDeliveryClaim, error)
	renewDelivery   func(context.Context, string, string, time.Duration) (time.Time, error)
	deliver         func(context.Context, reportDeliveryClaim) error
	reapDelivery    func(context.Context, string) (int64, error)
}

type ReportScheduleWorker struct {
	interval         time.Duration
	limit            int
	concurrency      int
	maxAttempts      int
	leaseTimeout     time.Duration
	operationTimeout time.Duration
	shutdownTimeout  time.Duration
	operations       reportWorkerOperations

	cancel context.CancelFunc
	done   chan struct{}
	mu     sync.Mutex
}

func NewReportScheduleWorker() *ReportScheduleWorker {
	processor := service.NewReportRunProcessor(nil)
	return &ReportScheduleWorker{
		interval: reportWorkerInterval(), limit: reportWorkerLimit(), concurrency: reportWorkerConcurrency(),
		maxAttempts: reportWorkerMaxAttempts(), leaseTimeout: reportWorkerLeaseTimeout(),
		operationTimeout: reportWorkerOperationTimeout(), shutdownTimeout: reportWorkerShutdownTimeout(),
		operations: reportWorkerOperations{
			initialize:      dal.InitializeReportScheduleNextRuns,
			dispatch:        dal.MaterializeDueReportScheduleRuns,
			claimGeneration: dal.ClaimReportGenerations,
			renewGeneration: dal.RenewReportGenerationLease,
			generate:        processor.GenerateClaim,
			reapGeneration:  dal.ReapExpiredReportGenerations,
			claimDelivery:   dal.ClaimReportDeliveries,
			renewDelivery:   dal.RenewReportDeliveryLease,
			deliver:         processor.DeliverClaim,
			reapDelivery:    dal.MarkExpiredReportDeliveriesAmbiguous,
		},
	}
}

func (*ReportScheduleWorker) Name() string { return "report-schedule-worker" }

func (worker *ReportScheduleWorker) Start() error {
	if !reportWorkerEnabled() {
		logrus.Info("report schedule worker disabled")
		return nil
	}
	worker.mu.Lock()
	defer worker.mu.Unlock()
	if worker.cancel != nil {
		select {
		case <-worker.done:
			worker.cancel, worker.done = nil, nil
		default:
			return nil
		}
	}
	if err := worker.validate(); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	worker.cancel = cancel
	worker.done = make(chan struct{})
	go worker.run(ctx, worker.done)
	logrus.WithFields(logrus.Fields{"interval": worker.interval, "limit": worker.limit, "concurrency": worker.concurrency}).Info("report schedule worker started")
	return nil
}

func (worker *ReportScheduleWorker) Stop() error {
	worker.mu.Lock()
	cancel, done := worker.cancel, worker.done
	worker.mu.Unlock()
	if cancel == nil || done == nil {
		return nil
	}
	cancel()
	timeout := worker.shutdownTimeout
	if timeout <= 0 {
		timeout = defaultReportWorkerShutdownTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		worker.mu.Lock()
		if worker.done == done {
			worker.cancel, worker.done = nil, nil
		}
		worker.mu.Unlock()
		logrus.Info("report schedule worker stopped")
		return nil
	case <-timer.C:
		return fmt.Errorf("report schedule worker stop timed out")
	}
}

func (worker *ReportScheduleWorker) validate() error {
	if worker.interval <= 0 || worker.limit < 1 || worker.concurrency < 1 || worker.maxAttempts < 1 ||
		worker.leaseTimeout <= 0 || worker.operationTimeout <= 0 {
		return fmt.Errorf("report schedule worker configuration is invalid")
	}
	operations := worker.operations
	if operations.initialize == nil || operations.dispatch == nil || operations.claimGeneration == nil ||
		operations.renewGeneration == nil || operations.generate == nil || operations.reapGeneration == nil ||
		operations.claimDelivery == nil || operations.renewDelivery == nil || operations.deliver == nil || operations.reapDelivery == nil {
		return fmt.Errorf("report schedule worker operations are incomplete")
	}
	return nil
}

func (worker *ReportScheduleWorker) run(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	worker.drain(ctx)
	ticker := time.NewTicker(worker.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			worker.drain(ctx)
		}
	}
}

func (worker *ReportScheduleWorker) drain(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	worker.runStep(ctx, "initialize schedules", func(step context.Context) error {
		_, err := worker.operations.initialize(step, worker.limit, service.NextReportScheduleOccurrence)
		return err
	})
	worker.runStep(ctx, "dispatch schedules", func(step context.Context) error {
		_, err := worker.operations.dispatch(step, worker.limit, worker.maxAttempts, service.NextReportScheduleOccurrence)
		return err
	})
	worker.drainGenerations(ctx)
	worker.drainDeliveries(ctx)
	worker.runStep(ctx, "recover generations", func(step context.Context) error {
		_, err := worker.operations.reapGeneration(step, reportGenerationLeaseExpiredCode)
		return err
	})
	worker.runStep(ctx, "recover deliveries", func(step context.Context) error {
		_, err := worker.operations.reapDelivery(step, reportDeliveryAcceptanceUnknownCode)
		return err
	})
}

func (worker *ReportScheduleWorker) runStep(parent context.Context, name string, operation func(context.Context) error) {
	if parent.Err() != nil {
		return
	}
	ctx, cancel := context.WithTimeout(parent, worker.operationTimeout)
	defer cancel()
	if err := operation(ctx); err != nil && parent.Err() == nil {
		logrus.WithError(err).Warn("report worker " + name + " failed")
	}
}

func (worker *ReportScheduleWorker) drainGenerations(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	var claims []reportGenerationClaim
	worker.runStep(ctx, "claim generations", func(step context.Context) error {
		var err error
		claims, err = worker.operations.claimGeneration(step, worker.limit, worker.leaseTimeout)
		return err
	})
	worker.processGenerationClaims(ctx, claims)
}

func (worker *ReportScheduleWorker) drainDeliveries(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	var claims []reportDeliveryClaim
	worker.runStep(ctx, "claim deliveries", func(step context.Context) error {
		var err error
		claims, err = worker.operations.claimDelivery(step, worker.limit, worker.leaseTimeout)
		return err
	})
	worker.processDeliveryClaims(ctx, claims)
}

func (worker *ReportScheduleWorker) processGenerationClaims(ctx context.Context, claims []reportGenerationClaim) {
	worker.processGenerationClaimBatch(ctx, claims)
}

func (worker *ReportScheduleWorker) processGenerationClaimBatch(ctx context.Context, claims []reportGenerationClaim) {
	worker.processClaimBatch(ctx, len(claims), func(index int) {
		claim := claims[index]
		if ctx.Err() != nil || claim.Run == nil {
			return
		}
		worker.processLeasedClaim(ctx, claim.Run.ID, claim.Token, worker.operations.renewGeneration,
			func(operation context.Context) error { return worker.operations.generate(operation, claim) })
	})
}

func (worker *ReportScheduleWorker) processDeliveryClaims(ctx context.Context, claims []reportDeliveryClaim) {
	worker.processClaimBatch(ctx, len(claims), func(index int) {
		claim := claims[index]
		if ctx.Err() != nil || claim.Run == nil {
			return
		}
		worker.processLeasedClaim(ctx, claim.Run.ID, claim.Token, worker.operations.renewDelivery,
			func(operation context.Context) error { return worker.operations.deliver(operation, claim) })
	})
}

func (worker *ReportScheduleWorker) processClaimBatch(ctx context.Context, count int, process func(int)) {
	if count == 0 || ctx.Err() != nil {
		return
	}
	concurrency := worker.concurrency
	if concurrency > count {
		concurrency = count
	}
	jobs := make(chan int)
	var wait sync.WaitGroup
	wait.Add(concurrency)
	for index := 0; index < concurrency; index++ {
		go func() {
			defer wait.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-jobs:
					if !ok {
						return
					}
					process(job)
				}
			}
		}()
	}
	for index := 0; index < count; index++ {
		select {
		case jobs <- index:
		case <-ctx.Done():
			close(jobs)
			wait.Wait()
			return
		}
	}
	close(jobs)
	wait.Wait()
}

func (worker *ReportScheduleWorker) processLeasedClaim(
	parent context.Context,
	runID string,
	token string,
	renew func(context.Context, string, string, time.Duration) (time.Time, error),
	operation func(context.Context) error,
) {
	if parent.Err() != nil {
		return
	}
	operationCtx, cancel := context.WithTimeout(parent, worker.operationTimeout)
	defer cancel()
	renewDone := make(chan struct{})
	go func() {
		defer close(renewDone)
		interval := worker.leaseTimeout / 3
		if interval < time.Millisecond {
			interval = time.Millisecond
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-operationCtx.Done():
				return
			case <-ticker.C:
				renewCtx, renewCancel := context.WithTimeout(operationCtx, minDuration(worker.operationTimeout, interval))
				_, err := renew(renewCtx, runID, token, worker.leaseTimeout)
				renewCancel()
				if err != nil {
					cancel()
					if !errors.Is(err, context.Canceled) && !errors.Is(err, dal.ErrReportClaimLost) {
						logrus.WithError(err).Warn("report worker lease renewal failed")
					}
					return
				}
			}
		}
	}()
	err := operation(operationCtx)
	cancel()
	<-renewDone
	if err != nil && !errors.Is(err, dal.ErrReportClaimLost) && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		logrus.WithError(err).Warn("report worker claimed operation failed")
	}
}

func minDuration(left, right time.Duration) time.Duration {
	if left < right {
		return left
	}
	return right
}

func reportWorkerEnabled() bool {
	if !viper.IsSet("reports.worker.enabled") {
		return true
	}
	return viper.GetBool("reports.worker.enabled")
}

func reportWorkerInterval() time.Duration {
	if value := viper.GetDuration("reports.worker.interval"); value > 0 {
		return value
	}
	return defaultReportWorkerInterval
}

func reportWorkerLimit() int {
	return boundedReportWorkerInt(viper.GetInt("reports.worker.limit"), defaultReportWorkerLimit, maxReportWorkerLimit)
}

func reportWorkerConcurrency() int {
	return boundedReportWorkerInt(viper.GetInt("reports.worker.concurrency"), defaultReportWorkerConcurrency, maxReportWorkerConcurrency)
}

func reportWorkerMaxAttempts() int {
	return boundedReportWorkerInt(viper.GetInt("reports.worker.max_attempts"), defaultReportWorkerMaxAttempts, 20)
}

func boundedReportWorkerInt(value, fallback, maximum int) int {
	if value < 1 {
		return fallback
	}
	if value > maximum {
		return maximum
	}
	return value
}

func reportWorkerLeaseTimeout() time.Duration {
	if value := viper.GetDuration("reports.worker.lease_timeout"); value > 0 {
		return value
	}
	return defaultReportWorkerLeaseTimeout
}

func reportWorkerOperationTimeout() time.Duration {
	if value := viper.GetDuration("reports.worker.operation_timeout"); value > 0 {
		return value
	}
	return defaultReportWorkerOperationTimeout
}

func reportWorkerShutdownTimeout() time.Duration {
	if value := viper.GetDuration("reports.worker.shutdown_timeout"); value > 0 {
		return value
	}
	return defaultReportWorkerShutdownTimeout
}

func WithReportScheduleWorker() Option {
	return func(application *Application) error {
		application.RegisterService(NewReportScheduleWorker())
		return nil
	}
}
