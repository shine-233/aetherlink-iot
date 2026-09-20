package app

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"

	"github.com/spf13/viper"
)

func testReportWorkerOperations(calls chan<- string) reportWorkerOperations {
	record := func(value string) {
		select {
		case calls <- value:
		default:
		}
	}
	return reportWorkerOperations{
		initialize: func(context.Context, int, dal.ReportNextOccurrence) (dal.ReportScheduleInitializationResult, error) {
			record("initialize")
			return dal.ReportScheduleInitializationResult{}, nil
		},
		dispatch: func(context.Context, int, int, dal.ReportNextOccurrence) ([]dal.ReportMaterializationResult, error) {
			record("dispatch")
			return nil, nil
		},
		claimGeneration: func(context.Context, int, time.Duration) ([]reportGenerationClaim, error) {
			record("claim-generation")
			return nil, nil
		},
		renewGeneration: func(context.Context, string, string, time.Duration) (time.Time, error) { return time.Now(), nil },
		generate:        func(context.Context, reportGenerationClaim) error { return nil },
		reapGeneration:  func(context.Context, string) (int64, error) { record("reap-generation"); return 0, nil },
		claimDelivery: func(context.Context, int, time.Duration) ([]reportDeliveryClaim, error) {
			record("claim-delivery")
			return nil, nil
		},
		renewDelivery: func(context.Context, string, string, time.Duration) (time.Time, error) { return time.Now(), nil },
		deliver:       func(context.Context, reportDeliveryClaim) error { return nil },
		reapDelivery:  func(context.Context, string) (int64, error) { record("reap-delivery"); return 0, nil },
	}
}

func TestReportScheduleWorkerConfigDefaultsAndOverrides(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	if !reportWorkerEnabled() || reportWorkerInterval() != defaultReportWorkerInterval || reportWorkerLimit() != defaultReportWorkerLimit ||
		reportWorkerConcurrency() != defaultReportWorkerConcurrency || reportWorkerMaxAttempts() != defaultReportWorkerMaxAttempts ||
		reportWorkerLeaseTimeout() != defaultReportWorkerLeaseTimeout || reportWorkerOperationTimeout() != defaultReportWorkerOperationTimeout ||
		reportWorkerShutdownTimeout() != defaultReportWorkerShutdownTimeout {
		t.Fatal("report worker defaults do not match the production contract")
	}
	viper.Set("reports.worker.enabled", false)
	viper.Set("reports.worker.interval", "45s")
	viper.Set("reports.worker.limit", 12)
	viper.Set("reports.worker.concurrency", 7)
	viper.Set("reports.worker.max_attempts", 5)
	viper.Set("reports.worker.lease_timeout", "3m")
	viper.Set("reports.worker.operation_timeout", "2m")
	viper.Set("reports.worker.shutdown_timeout", "7s")
	if reportWorkerEnabled() || reportWorkerInterval() != 45*time.Second || reportWorkerLimit() != 12 ||
		reportWorkerConcurrency() != 7 || reportWorkerMaxAttempts() != 5 || reportWorkerLeaseTimeout() != 3*time.Minute ||
		reportWorkerOperationTimeout() != 2*time.Minute || reportWorkerShutdownTimeout() != 7*time.Second {
		t.Fatal("report worker did not apply configuration overrides")
	}
}

func TestReportScheduleWorkerConfigurationBounds(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("reports.worker.limit", maxReportWorkerLimit+1)
	viper.Set("reports.worker.concurrency", maxReportWorkerConcurrency+1)
	viper.Set("reports.worker.max_attempts", 21)
	if reportWorkerLimit() != maxReportWorkerLimit || reportWorkerConcurrency() != maxReportWorkerConcurrency || reportWorkerMaxAttempts() != 20 {
		t.Fatal("report worker did not cap oversized configuration")
	}
}

func TestWithReportScheduleWorkerRegistersLifecycleService(t *testing.T) {
	application := &Application{ServiceManager: NewServiceManager()}
	if err := WithReportScheduleWorker()(application); err != nil {
		t.Fatal(err)
	}
	if len(application.ServiceManager.services) != 1 {
		t.Fatalf("registered services = %d, want 1", len(application.ServiceManager.services))
	}
	worker, ok := application.ServiceManager.services[0].(*ReportScheduleWorker)
	if !ok || worker.Name() != "report-schedule-worker" {
		t.Fatalf("registered service = %T", application.ServiceManager.services[0])
	}
}

func TestReportScheduleWorkerRunsImmediateFullDrain(t *testing.T) {
	calls := make(chan string, 12)
	worker := &ReportScheduleWorker{interval: time.Hour, limit: 5, concurrency: 2, maxAttempts: 3, leaseTimeout: time.Second, operationTimeout: time.Second, shutdownTimeout: time.Second, operations: testReportWorkerOperations(calls)}
	if err := worker.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = worker.Stop() })
	want := []string{"initialize", "dispatch", "claim-generation", "claim-delivery", "reap-generation", "reap-delivery"}
	for _, expected := range want {
		select {
		case got := <-calls:
			if got != expected {
				t.Fatalf("drain step = %q, want %q", got, expected)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("timed out waiting for %s", expected)
		}
	}
	if err := worker.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestReportScheduleWorkerCancelsOperationOnStop(t *testing.T) {
	operations := testReportWorkerOperations(make(chan string, 10))
	operations.claimGeneration = func(context.Context, int, time.Duration) ([]reportGenerationClaim, error) {
		return []reportGenerationClaim{{Run: &model.ReportScheduleRun{ID: "run-1"}, Token: "token"}}, nil
	}
	started := make(chan struct{})
	operationErr := make(chan error, 1)
	operations.generate = func(ctx context.Context, _ reportGenerationClaim) error {
		defer close(operationErr)
		close(started)
		<-ctx.Done()
		operationErr <- ctx.Err()
		return ctx.Err()
	}
	worker := &ReportScheduleWorker{interval: time.Hour, limit: 1, concurrency: 1, maxAttempts: 1, leaseTimeout: time.Second, operationTimeout: time.Second, shutdownTimeout: time.Second, operations: operations}
	if err := worker.Start(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("generation did not start")
	}
	if err := worker.Stop(); err != nil {
		t.Fatal(err)
	}
	select {
	case err, ok := <-operationErr:
		if !ok {
			t.Fatal("Stop returned before operation completed")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("operation error = %v, want context canceled", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("operation did not observe Stop cancellation")
	}
}

func TestReportScheduleWorkerOperationDeadlineApplies(t *testing.T) {
	operations := testReportWorkerOperations(make(chan string, 10))
	operations.claimGeneration = func(context.Context, int, time.Duration) ([]reportGenerationClaim, error) {
		return []reportGenerationClaim{{Run: &model.ReportScheduleRun{ID: "run-1"}, Token: "token"}}, nil
	}
	operationErr := make(chan error, 1)
	operations.generate = func(ctx context.Context, _ reportGenerationClaim) error {
		<-ctx.Done()
		operationErr <- ctx.Err()
		return ctx.Err()
	}
	worker := &ReportScheduleWorker{interval: time.Hour, limit: 1, concurrency: 1, maxAttempts: 1, leaseTimeout: time.Second, operationTimeout: 20 * time.Millisecond, shutdownTimeout: time.Second, operations: operations}
	if err := worker.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = worker.Stop() })
	select {
	case err := <-operationErr:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("operation error = %v, want deadline exceeded", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("operation deadline did not apply")
	}
}

func TestReportScheduleWorkerStopsClaimsBeforeWaitingForInFlightOperation(t *testing.T) {
	calls := make(chan string, 20)
	operations := testReportWorkerOperations(calls)
	var claimCalls atomic.Int32
	operations.claimGeneration = func(context.Context, int, time.Duration) ([]reportGenerationClaim, error) {
		claimCalls.Add(1)
		return []reportGenerationClaim{{Run: &model.ReportScheduleRun{ID: "run-1"}, Token: "token"}}, nil
	}
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	operations.generate = func(context.Context, reportGenerationClaim) error {
		once.Do(func() { close(started) })
		<-release
		return nil
	}
	worker := &ReportScheduleWorker{interval: time.Millisecond, limit: 5, concurrency: 1, maxAttempts: 3, leaseTimeout: time.Second, operationTimeout: time.Second, shutdownTimeout: time.Second, operations: operations}
	if err := worker.Start(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("generation did not start")
	}
	stopDone := make(chan error, 1)
	go func() { stopDone <- worker.Stop() }()
	time.Sleep(20 * time.Millisecond)
	if got := claimCalls.Load(); got != 1 {
		t.Fatalf("claim calls after cancellation = %d, want 1", got)
	}
	close(release)
	select {
	case err := <-stopDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Stop did not drain the in-flight operation")
	}
}

func TestReportScheduleWorkerClaimLossCancelsOperation(t *testing.T) {
	operations := testReportWorkerOperations(make(chan string, 10))
	operations.claimGeneration = func(context.Context, int, time.Duration) ([]reportGenerationClaim, error) {
		return []reportGenerationClaim{{Run: &model.ReportScheduleRun{ID: "run-1"}, Token: "token"}}, nil
	}
	started := make(chan struct{})
	operationErr := make(chan error, 1)
	operations.generate = func(ctx context.Context, _ reportGenerationClaim) error {
		defer close(operationErr)
		close(started)
		<-ctx.Done()
		operationErr <- ctx.Err()
		return ctx.Err()
	}
	renewed := make(chan struct{}, 1)
	operations.renewGeneration = func(context.Context, string, string, time.Duration) (time.Time, error) {
		renewed <- struct{}{}
		return time.Time{}, dal.ErrReportClaimLost
	}
	worker := &ReportScheduleWorker{interval: time.Hour, limit: 1, concurrency: 1, maxAttempts: 1, leaseTimeout: 9 * time.Millisecond, operationTimeout: time.Second, shutdownTimeout: time.Second, operations: operations}
	if err := worker.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = worker.Stop() })
	select {
	case <-started:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("generation did not start")
	}
	select {
	case <-renewed:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("lease was not renewed")
	}
	select {
	case err, ok := <-operationErr:
		if !ok {
			t.Fatal("renewer returned before operation completed")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("operation error = %v, want context canceled", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("claim loss did not cancel operation")
	}
	select {
	case _, ok := <-operationErr:
		if ok {
			t.Fatal("operation completion channel remained open")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("operation did not finish after claim loss")
	}
}

func TestReportScheduleWorkerRenewerStopsAfterStopTimeout(t *testing.T) {
	operations := testReportWorkerOperations(make(chan string, 10))
	operations.claimGeneration = func(context.Context, int, time.Duration) ([]reportGenerationClaim, error) {
		return []reportGenerationClaim{{Run: &model.ReportScheduleRun{ID: "run-1"}, Token: "token"}}, nil
	}
	started, release := make(chan struct{}), make(chan struct{})
	operations.generate = func(context.Context, reportGenerationClaim) error {
		close(started)
		<-release
		return nil
	}
	var renewCalls atomic.Int32
	firstRenewal := make(chan struct{}, 1)
	operations.renewGeneration = func(context.Context, string, string, time.Duration) (time.Time, error) {
		renewCalls.Add(1)
		select {
		case firstRenewal <- struct{}{}:
		default:
		}
		return time.Now(), nil
	}
	worker := &ReportScheduleWorker{interval: time.Hour, limit: 1, concurrency: 1, maxAttempts: 1, leaseTimeout: 9 * time.Millisecond, operationTimeout: time.Second, shutdownTimeout: 20 * time.Millisecond, operations: operations}
	if err := worker.Start(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("generation did not start")
	}
	select {
	case <-firstRenewal:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("lease was not renewed")
	}
	if err := worker.Stop(); err == nil {
		t.Fatal("Stop should report its shutdown timeout")
	}
	afterStop := renewCalls.Load()
	time.Sleep(30 * time.Millisecond)
	if got := renewCalls.Load(); got != afterStop {
		t.Fatalf("renew calls after Stop timeout = %d, want %d", got, afterStop)
	}
	close(release)
	deadline := time.After(500 * time.Millisecond)
	for {
		if err := worker.Stop(); err == nil {
			break
		}
		select {
		case <-deadline:
			t.Fatal("worker did not finish after blocked operation was released")
		case <-time.After(time.Millisecond):
		}
	}
}

func TestReportScheduleWorkerStopIsBounded(t *testing.T) {
	calls := make(chan string, 10)
	operations := testReportWorkerOperations(calls)
	operations.claimGeneration = func(context.Context, int, time.Duration) ([]reportGenerationClaim, error) {
		return []reportGenerationClaim{{Run: &model.ReportScheduleRun{ID: "run-1"}, Token: "token"}}, nil
	}
	started, release := make(chan struct{}), make(chan struct{})
	operations.generate = func(context.Context, reportGenerationClaim) error { close(started); <-release; return nil }
	worker := &ReportScheduleWorker{interval: time.Hour, limit: 1, concurrency: 1, maxAttempts: 1, leaseTimeout: time.Second, operationTimeout: time.Second, shutdownTimeout: 20 * time.Millisecond, operations: operations}
	if err := worker.Start(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("generation did not start")
	}
	before := time.Now()
	if err := worker.Stop(); err == nil {
		t.Fatal("Stop should report its shutdown timeout")
	}
	if elapsed := time.Since(before); elapsed > 200*time.Millisecond {
		t.Fatalf("Stop took %s", elapsed)
	}
	close(release)
	deadline := time.After(500 * time.Millisecond)
	for {
		if err := worker.Stop(); err == nil {
			break
		}
		select {
		case <-deadline:
			t.Fatal("worker did not finish after blocked operation was released")
		case <-time.After(time.Millisecond):
		}
	}
}
