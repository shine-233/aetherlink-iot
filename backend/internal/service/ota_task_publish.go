package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/mqtt/publish"
	"aetherlink-iot/backend/pkg/common"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
)

var (
	errOTADeviceOffline          = errors.New("ota device is offline")
	errOTADeviceAlreadyUpgrading = errors.New("ota device already has an active upgrade")
	errOTADispatchStateChanged   = errors.New("ota task detail state changed before dispatch")
	// errOTADispatchCanceled 表示调用方（worker 停机）在 claim 之前取消了本次下发；
	// 行保持 PENDING，不记失败，由租约过期/归还后重新领取。
	errOTADispatchCanceled = errors.New("ota dispatch canceled before claim")
)

// otaPublishAddress 是下发出口；测试替换为假 publisher。
var otaPublishAddress = publish.PublishOtaAddress

const (
	otaDispatchReasonInProgress        = "BROKER_PUBLISH_IN_PROGRESS"
	otaDispatchReasonAccepted          = "BROKER_PUBLISH_ACCEPTED"
	otaDispatchReasonOffline           = "DEVICE_OFFLINE"
	otaDispatchReasonAlreadyUpgrading  = "DEVICE_ALREADY_UPGRADING"
	otaDispatchReasonBrokerUnavailable = "BROKER_UNAVAILABLE"
	otaDispatchReasonPublishTimeout    = "BROKER_PUBLISH_TIMEOUT_OUTCOME_UNKNOWN"
	otaDispatchReasonPublishFailed     = "BROKER_PUBLISH_FAILED"
	otaDispatchReasonPackageInvalid    = "PACKAGE_OR_PAYLOAD_INVALID"
	otaDispatchReasonPrecheckFailed    = "DISPATCH_PRECHECK_FAILED"
)

// otaUpgradePushContext 是一批下发共享的预取数据。devices / packagesByTaskID 只读；
// activeTaskCounts 与 params 缓存会被并发的下发协程修改，必须持 mu 访问。
type otaUpgradePushContext struct {
	devices          map[string]*model.Device
	packagesByTaskID map[string]*model.OtaUpgradePackage
	activeTaskCounts map[string]int64
	paramsByPackage  map[string]*otaUpgradeParamsEntry

	mu sync.Mutex
}

// otaUpgradeParamsEntry 让同一升级包在一批内只下载/校验一次（包括失败结果）：
// 并发协程同时 miss 时，否则会对同一个 URL 重复下载做签名校验。
type otaUpgradeParamsEntry struct {
	once   sync.Once
	params map[string]interface{}
	err    error
}

// pushOTAUpgradeTaskDetails 保留原签名供 rollout 治理执行面复用（otaGovernanceApplier.dispatch），
// 内部改走有界并发、逐项 recover 的分发器。
func pushOTAUpgradeTaskDetails(o *OTA, tasks []*model.OtaUpgradeTaskDetail) {
	o.dispatchOTAUpgradeTaskDetails(context.Background(), tasks, defaultOTADispatchConcurrency)
}

func (o *OTA) PushOTAUpgradePackage(taskDetail *model.OtaUpgradeTaskDetail) error {
	pushContext, err := loadOTAUpgradePushContext([]*model.OtaUpgradeTaskDetail{taskDetail})
	if err != nil {
		return errors.Join(err, recordOTAUpgradePushFailure(nil, taskDetail, err))
	}
	return o.pushOTAUpgradePackageAndRecordFailure(context.Background(), taskDetail, pushContext)
}

func (o *OTA) pushOTAUpgradePackageAndRecordFailure(
	ctx context.Context,
	taskDetail *model.OtaUpgradeTaskDetail,
	pushContext *otaUpgradePushContext,
) error {
	err := o.pushOTAUpgradePackageWithContext(ctx, taskDetail, pushContext)
	if err == nil || errors.Is(err, errOTADispatchCanceled) {
		return err
	}
	return errors.Join(err, recordOTAUpgradePushFailure(pushContext, taskDetail, err))
}

func (*OTA) pushOTAUpgradePackageWithContext(
	ctx context.Context,
	taskDetail *model.OtaUpgradeTaskDetail,
	pushContext *otaUpgradePushContext,
) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if taskDetail == nil {
		return fmt.Errorf("ota task detail is required")
	}

	device, err := getOTAUpgradePushDevice(taskDetail.DeviceID, pushContext)
	if err != nil {
		return err
	}

	if err := ensureOTAUpgradeTaskPushable(taskDetail, device, pushContext); err != nil {
		return err
	}

	otapackage, err := getOTAUpgradePackageForTask(taskDetail.OtaUpgradeTaskID, pushContext)
	if err != nil {
		return err
	}

	payload, err := buildOTAUpgradePublishPayload(taskDetail.ID, otapackage, pushContext)
	if err != nil {
		return err
	}

	// 停机检查放在 claim 之前：claim 之后行已是 PUSHED/IN_PROGRESS，
	// 此时放弃只会制造一条要等 sweeper 才能收尾的"结果未知"行。
	if ctxErr := ctx.Err(); ctxErr != nil {
		return errors.Join(errOTADispatchCanceled, ctxErr)
	}

	claimed, err := claimOTAUpgradeTaskDetailForPublish(taskDetail)
	if err != nil {
		return err
	}
	if !claimed {
		return errOTADispatchStateChanged
	}

	publishErr := otaPublishAddress(device.DeviceNumber, payload)
	if publishErr != nil {
		reason := otaUpgradePushFailureReason(publishErr)
		affected, statusErr := updateOTAUpgradeTaskDetailIfStatus(
			taskDetail,
			[]int16{model.OtaUpgradeTaskDetailStatusPushed},
			map[string]interface{}{
				"status":             model.OtaUpgradeTaskDetailStatusFailed,
				"status_description": reason,
				"updated_at":         time.Now().UTC(),
			},
		)
		if statusErr == nil && affected > 0 {
			taskDetail.Status = model.OtaUpgradeTaskDetailStatusFailed
			taskDetail.StatusDescription = &reason
		}
		if statusErr == nil && affected == 0 {
			current, loadErr := query.OtaUpgradeTaskDetail.Where(query.OtaUpgradeTaskDetail.ID.Eq(taskDetail.ID)).First()
			if loadErr == nil && (current.Status == model.OtaUpgradeTaskDetailStatusUpgrading || current.Status == model.OtaUpgradeTaskDetailStatusSucceeded) {
				// A device response is stronger evidence than the publisher's late
				// timeout/error result, whose delivery outcome may be ambiguous.
				return nil
			}
		}
		return errors.Join(publishErr, statusErr)
	}

	_, err = updateOTAUpgradeTaskDetailIfStatus(
		taskDetail,
		[]int16{model.OtaUpgradeTaskDetailStatusPushed},
		map[string]interface{}{
			"status_description": otaDispatchReasonAccepted,
			"updated_at":         time.Now().UTC(),
		},
	)

	return err
}

func claimOTAUpgradeTaskDetailForPublish(taskDetail *model.OtaUpgradeTaskDetail) (bool, error) {
	if taskDetail == nil {
		return false, fmt.Errorf("ota task detail is required")
	}
	if taskDetail.Status != model.OtaUpgradeTaskDetailStatusPending && taskDetail.Status != model.OtaUpgradeTaskDetailStatusFailed {
		return false, nil
	}
	zeroStep := int16(0)
	now := time.Now().UTC()
	affected, err := updateOTAUpgradeTaskDetailIfStatus(
		taskDetail,
		[]int16{taskDetail.Status},
		map[string]interface{}{
			"status":               model.OtaUpgradeTaskDetailStatusPushed,
			"status_description":   otaDispatchReasonInProgress,
			"steps":                zeroStep,
			"updated_at":           now,
			"dispatch_lease_token": nil,
			"dispatch_lease_until": nil,
		},
	)
	if err != nil || affected != 1 {
		return false, err
	}
	taskDetail.Status = model.OtaUpgradeTaskDetailStatusPushed
	taskDetail.StatusDescription = stringPointer(otaDispatchReasonInProgress)
	taskDetail.Step = &zeroStep
	taskDetail.UpdatedAt = &now
	return true, nil
}

func updateOTAUpgradeTaskDetailIfStatus(
	taskDetail *model.OtaUpgradeTaskDetail,
	fromStatuses []int16,
	updates map[string]interface{},
) (int64, error) {
	if taskDetail == nil || strings.TrimSpace(taskDetail.ID) == "" || len(fromStatuses) == 0 {
		return 0, fmt.Errorf("ota task detail transition is incomplete")
	}
	result := global.DB.Model(&model.OtaUpgradeTaskDetail{}).
		Where("id = ? AND status IN ?", taskDetail.ID, fromStatuses).
		Updates(updates)
	return result.RowsAffected, result.Error
}

func recordOTAUpgradePushFailure(
	pushContext *otaUpgradePushContext,
	taskDetail *model.OtaUpgradeTaskDetail,
	cause error,
) error {
	if taskDetail == nil {
		return nil
	}
	if taskDetail.Status != model.OtaUpgradeTaskDetailStatusPending && taskDetail.Status != model.OtaUpgradeTaskDetailStatusFailed {
		return nil
	}
	return failOTAUpgradeTaskDetail(pushContext, taskDetail, otaUpgradePushFailureReason(cause))
}

func otaUpgradePushFailureReason(err error) string {
	switch {
	case errors.Is(err, errOTADeviceOffline):
		return otaDispatchReasonOffline
	case errors.Is(err, errOTADeviceAlreadyUpgrading):
		return otaDispatchReasonAlreadyUpgrading
	case errors.Is(err, publish.ErrPublishTimeout):
		return otaDispatchReasonPublishTimeout
	case errors.Is(err, publish.ErrPublisherUnavailable):
		return otaDispatchReasonBrokerUnavailable
	case err != nil && (strings.Contains(strings.ToLower(err.Error()), "package") || strings.Contains(strings.ToLower(err.Error()), "payload")):
		return otaDispatchReasonPackageInvalid
	case err != nil && strings.Contains(strings.ToLower(err.Error()), "publish"):
		return otaDispatchReasonPublishFailed
	default:
		return otaDispatchReasonPrecheckFailed
	}
}

func logOTAUpgradePushFailure(taskDetail *model.OtaUpgradeTaskDetail, err error) {
	fields := logrus.Fields{"reason": otaUpgradePushFailureReason(err)}
	if taskDetail != nil {
		fields["ota_task_detail_id"] = taskDetail.ID
		fields["ota_task_id"] = taskDetail.OtaUpgradeTaskID
	}
	logrus.WithFields(fields).Warn("OTA task detail dispatch failed")
}

func stringPointer(value string) *string {
	return &value
}

func loadOTAUpgradePushContext(tasks []*model.OtaUpgradeTaskDetail) (*otaUpgradePushContext, error) {
	deviceIDs := uniqueOTAUpgradeTaskDeviceIDs(tasks)
	taskIDs := uniqueOTAUpgradeTaskIDs(tasks)
	devices, err := loadOTAUpgradePushDevices(deviceIDs)
	if err != nil {
		return nil, err
	}
	packagesByTaskID, err := loadOTAUpgradePackagesForTasks(taskIDs)
	if err != nil {
		return nil, err
	}
	activeTaskCounts, err := loadOTAUpgradeActiveTaskCounts(deviceIDs)
	if err != nil {
		return nil, err
	}
	return &otaUpgradePushContext{
		devices:          devices,
		packagesByTaskID: packagesByTaskID,
		activeTaskCounts: activeTaskCounts,
		paramsByPackage:  map[string]*otaUpgradeParamsEntry{},
	}, nil
}

func uniqueOTAUpgradeTaskDeviceIDs(tasks []*model.OtaUpgradeTaskDetail) []string {
	ids := make([]string, 0, len(tasks))
	seen := make(map[string]struct{}, len(tasks))
	for _, taskDetail := range tasks {
		if taskDetail == nil || taskDetail.DeviceID == "" {
			continue
		}
		if _, ok := seen[taskDetail.DeviceID]; ok {
			continue
		}
		seen[taskDetail.DeviceID] = struct{}{}
		ids = append(ids, taskDetail.DeviceID)
	}
	return ids
}

func uniqueOTAUpgradeTaskIDs(tasks []*model.OtaUpgradeTaskDetail) []string {
	ids := make([]string, 0, len(tasks))
	seen := make(map[string]struct{}, len(tasks))
	for _, taskDetail := range tasks {
		if taskDetail == nil || taskDetail.OtaUpgradeTaskID == "" {
			continue
		}
		if _, ok := seen[taskDetail.OtaUpgradeTaskID]; ok {
			continue
		}
		seen[taskDetail.OtaUpgradeTaskID] = struct{}{}
		ids = append(ids, taskDetail.OtaUpgradeTaskID)
	}
	return ids
}

func loadOTAUpgradePushDevice(deviceID string) (*model.Device, error) {
	return query.Device.Where(query.Device.ID.Eq(deviceID)).First()
}

func loadOTAUpgradePushDevices(deviceIDs []string) (map[string]*model.Device, error) {
	devicesByID := make(map[string]*model.Device, len(deviceIDs))
	if len(deviceIDs) == 0 {
		return devicesByID, nil
	}
	devices, err := query.Device.Where(query.Device.ID.In(deviceIDs...)).Find()
	if err != nil {
		return nil, err
	}
	for _, device := range devices {
		if device != nil {
			devicesByID[device.ID] = device
		}
	}
	return devicesByID, nil
}

func getOTAUpgradePushDevice(deviceID string, pushContext *otaUpgradePushContext) (*model.Device, error) {
	if pushContext != nil && pushContext.devices != nil {
		if device := pushContext.devices[deviceID]; device != nil {
			return device, nil
		}
		return nil, fmt.Errorf("ota push device not found: %s", deviceID)
	}
	return loadOTAUpgradePushDevice(deviceID)
}

func ensureOTAUpgradeTaskPushable(
	taskDetail *model.OtaUpgradeTaskDetail,
	device *model.Device,
	pushContext *otaUpgradePushContext,
) error {
	if device.IsOnline != 1 {
		return fmt.Errorf("%w: %s", errOTADeviceOffline, device.ID)
	}

	hasOtherActiveTask, err := hasOtherActiveOTAUpgradeTask(taskDetail, pushContext)
	if err != nil {
		return err
	}
	if hasOtherActiveTask {
		return fmt.Errorf("%w: %s", errOTADeviceAlreadyUpgrading, device.ID)
	}
	return nil
}

func failOTAUpgradeTaskDetail(
	pushContext *otaUpgradePushContext,
	taskDetail *model.OtaUpgradeTaskDetail,
	description string,
) error {
	wasActive := taskDetail != nil && taskDetail.Status < model.OtaUpgradeTaskDetailStatusSucceeded
	affected, err := updateOTAUpgradeTaskDetailIfStatus(
		taskDetail,
		[]int16{model.OtaUpgradeTaskDetailStatusPending, model.OtaUpgradeTaskDetailStatusFailed},
		map[string]interface{}{
			"status":               model.OtaUpgradeTaskDetailStatusFailed,
			"status_description":   description,
			"updated_at":           time.Now().UTC(),
			"dispatch_lease_token": nil,
			"dispatch_lease_until": nil,
		},
	)
	if err != nil {
		return err
	}
	if affected > 0 {
		taskDetail.Status = model.OtaUpgradeTaskDetailStatusFailed
		taskDetail.StatusDescription = stringPointer(description)
	}
	if pushContext != nil && pushContext.activeTaskCounts != nil && wasActive && affected > 0 {
		pushContext.mu.Lock()
		pushContext.activeTaskCounts[taskDetail.DeviceID]--
		pushContext.mu.Unlock()
	}
	return nil
}

func hasOtherActiveOTAUpgradeTask(
	taskDetail *model.OtaUpgradeTaskDetail,
	pushContext *otaUpgradePushContext,
) (bool, error) {
	if pushContext != nil && pushContext.activeTaskCounts != nil {
		currentDetailActive := taskDetail.Status < model.OtaUpgradeTaskDetailStatusSucceeded
		pushContext.mu.Lock()
		count := pushContext.activeTaskCounts[taskDetail.DeviceID]
		pushContext.mu.Unlock()
		if currentDetailActive {
			count--
		}
		return count > 0, nil
	}

	count, err := query.OtaUpgradeTaskDetail.Where(
		query.OtaUpgradeTaskDetail.DeviceID.Eq(taskDetail.DeviceID),
		query.OtaUpgradeTaskDetail.Status.Lt(model.OtaUpgradeTaskDetailStatusSucceeded),
		query.OtaUpgradeTaskDetail.ID.Neq(taskDetail.ID),
	).Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

type otaUpgradeActiveTaskCount struct {
	DeviceID string `gorm:"column:device_id"`
	Count    int64  `gorm:"column:count"`
}

func loadOTAUpgradeActiveTaskCounts(deviceIDs []string) (map[string]int64, error) {
	counts := make(map[string]int64, len(deviceIDs))
	if len(deviceIDs) == 0 {
		return counts, nil
	}

	rows := make([]otaUpgradeActiveTaskCount, 0, len(deviceIDs))
	err := global.DB.
		Raw(
			`SELECT device_id, COUNT(*) AS count
			 FROM ota_upgrade_task_details
			 WHERE device_id IN ? AND status < ?
			 GROUP BY device_id`,
			deviceIDs,
			model.OtaUpgradeTaskDetailStatusSucceeded,
		).
		Scan(&rows).
		Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts[row.DeviceID] = row.Count
	}
	return counts, nil
}

func loadOTAUpgradePackageForTask(taskID string) (*model.OtaUpgradePackage, error) {
	task, err := query.OtaUpgradeTask.Where(query.OtaUpgradeTask.ID.Eq(taskID)).First()
	if err != nil {
		return nil, err
	}
	return query.OtaUpgradePackage.Where(query.OtaUpgradePackage.ID.Eq(task.OtaUpgradePackageID)).First()
}

func loadOTAUpgradePackagesForTasks(taskIDs []string) (map[string]*model.OtaUpgradePackage, error) {
	packagesByTaskID := make(map[string]*model.OtaUpgradePackage, len(taskIDs))
	if len(taskIDs) == 0 {
		return packagesByTaskID, nil
	}

	tasks, err := query.OtaUpgradeTask.Where(query.OtaUpgradeTask.ID.In(taskIDs...)).Find()
	if err != nil {
		return nil, err
	}

	packageIDs := make([]string, 0, len(tasks))
	packageIDByTaskID := make(map[string]string, len(tasks))
	seenPackageIDs := make(map[string]struct{}, len(tasks))
	for _, task := range tasks {
		if task == nil || task.OtaUpgradePackageID == "" {
			continue
		}
		packageIDByTaskID[task.ID] = task.OtaUpgradePackageID
		if _, ok := seenPackageIDs[task.OtaUpgradePackageID]; ok {
			continue
		}
		seenPackageIDs[task.OtaUpgradePackageID] = struct{}{}
		packageIDs = append(packageIDs, task.OtaUpgradePackageID)
	}
	if len(packageIDs) == 0 {
		return packagesByTaskID, nil
	}

	packages, err := query.OtaUpgradePackage.Where(query.OtaUpgradePackage.ID.In(packageIDs...)).Find()
	if err != nil {
		return nil, err
	}
	packagesByID := make(map[string]*model.OtaUpgradePackage, len(packages))
	for _, otaPackage := range packages {
		if otaPackage != nil {
			packagesByID[otaPackage.ID] = otaPackage
		}
	}
	for taskID, packageID := range packageIDByTaskID {
		if otaPackage := packagesByID[packageID]; otaPackage != nil {
			packagesByTaskID[taskID] = otaPackage
		}
	}
	return packagesByTaskID, nil
}

func getOTAUpgradePackageForTask(
	taskID string,
	pushContext *otaUpgradePushContext,
) (*model.OtaUpgradePackage, error) {
	if pushContext != nil && pushContext.packagesByTaskID != nil {
		if otaPackage := pushContext.packagesByTaskID[taskID]; otaPackage != nil {
			return otaPackage, nil
		}
		return nil, fmt.Errorf("ota package not found for task: %s", taskID)
	}
	return loadOTAUpgradePackageForTask(taskID)
}

func buildOTAUpgradePublishPayload(
	taskDetailID string,
	otapackage *model.OtaUpgradePackage,
	pushContext *otaUpgradePushContext,
) ([]byte, error) {
	params, err := getOTAUpgradeMessageParams(otapackage, pushContext)
	if err != nil {
		return nil, err
	}

	randNum, err := common.GetRandomNineDigits()
	if err != nil {
		return nil, err
	}
	payload, jsonErr := buildOTAUpgradeMessagePayload(randNum, params)
	if jsonErr != nil {
		logrus.Error("failed to marshal OTA upgrade payload")
		return nil, jsonErr
	}
	return payload, nil
}

func getOTAUpgradeMessageParams(
	otapackage *model.OtaUpgradePackage,
	pushContext *otaUpgradePushContext,
) (map[string]interface{}, error) {
	if pushContext == nil || pushContext.paramsByPackage == nil || otapackage == nil {
		return buildOTAUpgradeMessageParams(otapackage)
	}
	pushContext.mu.Lock()
	entry := pushContext.paramsByPackage[otapackage.ID]
	if entry == nil {
		entry = &otaUpgradeParamsEntry{}
		pushContext.paramsByPackage[otapackage.ID] = entry
	}
	pushContext.mu.Unlock()
	entry.once.Do(func() {
		entry.params, entry.err = buildOTAUpgradeMessageParams(otapackage)
	})
	return entry.params, entry.err
}

func buildOTAUpgradeMessagePayload(messageID string, params map[string]interface{}) ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":     messageID,
		"code":   "200",
		"params": params,
	})
}

func verifyOTAPackageIntegrity(otapackage *model.OtaUpgradePackage) error {
	if otapackage == nil {
		return fmt.Errorf("ota package is required")
	}
	if otapackage.PackageURL == nil || strings.TrimSpace(*otapackage.PackageURL) == "" {
		return fmt.Errorf("ota package url is required")
	}
	if otapackage.SignatureType == nil || strings.TrimSpace(*otapackage.SignatureType) == "" {
		return fmt.Errorf("ota package signature type is required")
	}
	signatureType := strings.ToUpper(strings.TrimSpace(*otapackage.SignatureType))
	if signatureType != "MD5" && signatureType != "SHA256" {
		return fmt.Errorf("unsupported ota package signature type: %s", signatureType)
	}
	if otapackage.Signature == nil || strings.TrimSpace(*otapackage.Signature) == "" {
		return fmt.Errorf("ota package signature is required")
	}

	actual, err := signOTAPackageFromURL(*otapackage.PackageURL, signatureType)
	if err != nil {
		return fmt.Errorf("verify ota package integrity: %w", err)
	}
	if !strings.EqualFold(actual, strings.TrimSpace(*otapackage.Signature)) {
		return fmt.Errorf("ota package integrity verification failed")
	}
	return nil
}

func buildOTAUpgradeMessageParams(otapackage *model.OtaUpgradePackage) (map[string]interface{}, error) {
	if otapackage == nil {
		return nil, fmt.Errorf("ota package is required")
	}
	if otapackage.PackageURL == nil || strings.TrimSpace(*otapackage.PackageURL) == "" {
		return nil, fmt.Errorf("ota package url is required")
	}
	if err := verifyOTAPackageIntegrity(otapackage); err != nil {
		return nil, err
	}

	extData := map[string]interface{}{}
	if otapackage.AdditionalInfo != nil && strings.TrimSpace(*otapackage.AdditionalInfo) != "" {
		if err := json.Unmarshal([]byte(*otapackage.AdditionalInfo), &extData); err != nil {
			return nil, fmt.Errorf("ota additional info is not valid JSON: %w", err)
		}
	}

	packageURL := global.OtaAddress + strings.TrimPrefix(*otapackage.PackageURL, ".")
	signature := ""
	if otapackage.Signature != nil {
		signature = *otapackage.Signature
	}

	return map[string]interface{}{
		"version":      otapackage.Version,
		"size":         "0",
		"url":          packageURL,
		"firmware_url": packageURL,
		"signMethod":   otapackage.SignatureType,
		"sign_method":  otapackage.SignatureType,
		"sign":         signature,
		"signature":    signature,
		"module":       otapackage.Module,
		"extData":      extData,
		"ext_data":     extData,
	}, nil
}
