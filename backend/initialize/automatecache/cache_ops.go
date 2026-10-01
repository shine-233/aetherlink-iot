package automatecache

import (
	"context"

	"aetherlink-iot/backend/internal/model"

	"github.com/sirupsen/logrus"
)

var allDimensions = []KeyDimension{dimensionMultiple, dimensionOne}

// DeleteCacheBySceneAutomationId 按场景 ID 清理两种设备维度下的全部自动化缓存。
func (c *Cache) DeleteCacheBySceneAutomationId(sceneAutomationID string) error {
	for _, dim := range allDimensions {
		if err := c.deleteBySceneAutomationID(dim, sceneAutomationID); err != nil {
			return err
		}
	}
	return nil
}

// deleteBySceneAutomationID 在给定维度下回收动作、条件组和设备一级缓存。
func (c *Cache) deleteBySceneAutomationID(dim KeyDimension, sceneAutomationID string) error {
	var (
		action     AutomateActionInfo
		deleteKeys []string
		deviceIDs  []string
	)
	actionKey := keyAction(dim, sceneAutomationID)
	deleteKeys = append(deleteKeys, actionKey)
	resultInt, err := scan(c.get(actionKey), &action)
	if err != nil {
		return err
	}
	// 根据动作缓存里记录的条件组反查此场景关联的设备。
	if resultInt == ResultOK {
		for _, groupID := range action.GroupIds {
			groupKey := keyGroup(dim, groupID)
			deleteKeys = append(deleteKeys, groupKey)
			var groupInfos DTConditions
			if err := c.get(groupKey).Scan(&groupInfos); err != nil {
				continue
			}
			for _, v := range groupInfos {
				if v.TriggerSource != nil && v.TriggerConditionType == dim.GetDeviceTriggerConditionType() {
					deviceIDs = append(deviceIDs, *v.TriggerSource)
				}
			}
		}
	}
	if err := c.pruneDeviceCaches(dim, sceneAutomationID, deviceIDs, &deleteKeys); err != nil {
		return err
	}
	// 反查链路可能断（条件组缓存已过期），再用 SCAN 兜底收集残留键。
	if err := c.scanGroupKeysForScene(dim, sceneAutomationID, &deleteKeys); err != nil {
		return err
	}
	if err := c.scanDeviceKeysForScene(dim, sceneAutomationID, &deleteKeys); err != nil {
		return err
	}
	return c.client.Del(context.Background(), deleteKeys...).Err()
}

// pruneDeviceCaches 从设备一级缓存中移除目标场景引用，空列表的键收集待删。
func (c *Cache) pruneDeviceCaches(dim KeyDimension, sceneAutomationID string, ids []string, deleteKeys *[]string) error {
	for _, deviceID := range ids {
		baseKey := keyBase(dim, deviceID)
		var infos AutomateDeviceInfos
		if err := c.get(baseKey).Scan(&infos); err != nil {
			continue
		}
		if err := c.pruneDeviceKey(baseKey, infos, sceneAutomationID, deleteKeys); err != nil {
			return err
		}
	}
	return nil
}

func (c *Cache) pruneDeviceKey(key string, infos AutomateDeviceInfos, sceneAutomationID string, deleteKeys *[]string) error {
	updated, removed := RemoveSceneAutomationFromDeviceInfos(infos, sceneAutomationID)
	if !removed {
		return nil
	}
	if len(updated) == 0 {
		*deleteKeys = append(*deleteKeys, key)
		return nil
	}
	return c.set(key, updated)
}

func (c *Cache) scanGroupKeysForScene(dim KeyDimension, sceneAutomationID string, deleteKeys *[]string) error {
	return c.scanKeys(keyGroup(dim, "*"), func(key string) error {
		var groupInfos DTConditions
		if err := c.get(key).Scan(&groupInfos); err != nil {
			return nil
		}
		for _, condition := range groupInfos {
			if condition.SceneAutomationID == sceneAutomationID {
				*deleteKeys = append(*deleteKeys, key)
				break
			}
		}
		return nil
	})
}

func (c *Cache) scanDeviceKeysForScene(dim KeyDimension, sceneAutomationID string, deleteKeys *[]string) error {
	return c.scanKeys(keyBase(dim, "*"), func(key string) error {
		var infos AutomateDeviceInfos
		if err := c.get(key).Scan(&infos); err != nil {
			return nil
		}
		return c.pruneDeviceKey(key, infos, sceneAutomationID, deleteKeys)
	})
}

// RemoveSceneAutomationFromDeviceInfos 原地过滤掉目标场景，返回过滤后切片与是否命中。
func RemoveSceneAutomationFromDeviceInfos(infos AutomateDeviceInfos, sceneAutomationID string) (AutomateDeviceInfos, bool) {
	updated := infos[:0]
	removed := false
	for _, info := range infos {
		if info.SceneAutomationId == sceneAutomationID {
			removed = true
			continue
		}
		updated = append(updated, info)
	}
	return updated, removed
}

func (c *Cache) scanKeys(pattern string, visit func(string) error) error {
	ctx := context.Background()
	var cursor uint64
	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		for _, key := range keys {
			if err := visit(key); err != nil {
				return err
			}
		}
		if nextCursor == 0 {
			return nil
		}
		cursor = nextCursor
	}
}

// SetCacheBySceneAutomationId 按场景维度重建缓存，同时写入单类设备与单一设备两条索引链。
func (c *Cache) SetCacheBySceneAutomationId(sceneAutomationID string, conditions []model.DeviceTriggerCondition, actions []model.ActionInfo) error {
	for _, dim := range allDimensions {
		if err := c.setBySceneAutomationID(dim, sceneAutomationID, conditions, actions); err != nil {
			return err
		}
	}
	return nil
}

func (c *Cache) setBySceneAutomationID(dim KeyDimension, sceneAutomationID string, conditions []model.DeviceTriggerCondition, actions []model.ActionInfo) error {
	condType := dim.GetDeviceTriggerConditionType()
	deviceInfo := AutomateDeviceInfo{SceneAutomationId: sceneAutomationID}
	actionInfos := AutomateActionInfo{Actions: actions}
	groupInfosMap := make(map[string][]model.DeviceTriggerCondition)
	deviceIDsMap := make(map[string]bool)

	for _, v := range conditions {
		groupInfosMap[v.GroupID] = append(groupInfosMap[v.GroupID], v)
		if v.TriggerSource != nil && v.TriggerConditionType == condType {
			deviceIDsMap[*v.TriggerSource] = true
		}
	}
	// 该维度下没有触发设备，不缓存。
	if len(deviceIDsMap) == 0 {
		return nil
	}
	for groupID, val := range groupInfosMap {
		// 跳过不含本维度条件的组，防止把定时任务条件组串进设备触发链。
		if !groupHasConditionType(val, condType) {
			continue
		}
		deviceInfo.GroupIds = append(deviceInfo.GroupIds, groupID)
		actionInfos.GroupIds = append(actionInfos.GroupIds, groupID)
		if err := c.set(keyGroup(dim, groupID), val); err != nil {
			return err
		}
	}
	if err := c.set(keyAction(dim, sceneAutomationID), actionInfos); err != nil {
		return err
	}
	for deviceID := range deviceIDsMap {
		var infos AutomateDeviceInfos
		deviceKey := keyBase(dim, deviceID)
		if _, err := scan(c.get(deviceKey), &infos); err != nil {
			continue
		}
		infos = append(infos, deviceInfo)
		if err := c.set(deviceKey, infos); err != nil {
			return err
		}
	}
	return nil
}

func groupHasConditionType(conditions []model.DeviceTriggerCondition, condType string) bool {
	for _, v := range conditions {
		if v.TriggerConditionType == condType {
			return true
		}
	}
	return false
}

// GetCacheByDeviceId 根据设备 ID 或设备配置 ID 读取联动缓存，并返回缓存命中状态。
func (c *Cache) GetCacheByDeviceId(deviceID, deviceConfigID string) (AutomateExecteParams, int, error) {
	dim := dimensionFor(deviceConfigID)
	baseID := deviceID
	if deviceConfigID != "" {
		baseID = deviceConfigID
	}
	params := AutomateExecteParams{DeviceId: deviceID, DeviceConfigId: deviceConfigID}
	deviceKey := keyBase(dim, baseID)
	infos := make(AutomateDeviceInfos, 0)
	resultInt, err := scan(c.get(deviceKey), &infos)
	if err != nil || resultInt != ResultOK {
		return params, resultInt, err
	}

	for _, info := range infos {
		sceneInfo := AutomateExecteSceneInfo{SceneAutomationId: info.SceneAutomationId}
		for _, groupID := range info.GroupIds {
			var condition DTConditions
			if err := c.get(keyGroup(dim, groupID)).Scan(&condition); err != nil {
				logrus.WithError(err).WithField("group_id", groupID).Warn("automate cache: group entry missing")
				continue
			}
			sceneInfo.GroupsCondition = append(sceneInfo.GroupsCondition, condition...)
		}
		var actionInfo AutomateActionInfo
		if err := c.get(keyAction(dim, info.SceneAutomationId)).Scan(&actionInfo); err != nil {
			logrus.WithError(err).WithField("scene_automation_id", info.SceneAutomationId).Warn("automate cache: action entry missing")
			continue
		}
		sceneInfo.Actions = actionInfo.Actions
		params.AutomateExecteSceeInfos = append(params.AutomateExecteSceeInfos, sceneInfo)
	}
	return params, resultInt, nil
}

// SetCacheByDeviceId 按单个设备或设备配置维度直接写入自动化缓存。
func (c *Cache) SetCacheByDeviceId(deviceID, deviceConfigID string, conditions []model.DeviceTriggerCondition, actions []model.ActionInfo) error {
	dim := dimensionFor(deviceConfigID)
	groupInfosMap := make(map[string][]model.DeviceTriggerCondition)
	sceneGroups := make(map[string]map[string]bool)
	for _, v := range conditions {
		groupInfosMap[v.GroupID] = append(groupInfosMap[v.GroupID], v)
		if sceneGroups[v.SceneAutomationID] == nil {
			sceneGroups[v.SceneAutomationID] = make(map[string]bool)
		}
		sceneGroups[v.SceneAutomationID][v.GroupID] = true
	}
	for groupID, val := range groupInfosMap {
		if err := c.set(keyGroup(dim, groupID), val); err != nil {
			return err
		}
	}

	actionsMap := make(map[string][]model.ActionInfo)
	for _, val := range actions {
		actionsMap[val.SceneAutomationID] = append(actionsMap[val.SceneAutomationID], val)
	}
	var deviceInfos []AutomateDeviceInfo
	for sceneAutomationID, sceneActions := range actionsMap {
		groupsMap, ok := sceneGroups[sceneAutomationID]
		if !ok {
			continue
		}
		groupIDs := make([]string, 0, len(groupsMap))
		for groupID := range groupsMap {
			groupIDs = append(groupIDs, groupID)
		}
		if err := c.set(keyAction(dim, sceneAutomationID), AutomateActionInfo{Actions: sceneActions, GroupIds: groupIDs}); err != nil {
			return err
		}
		deviceInfos = append(deviceInfos, AutomateDeviceInfo{SceneAutomationId: sceneAutomationID, GroupIds: groupIDs})
	}
	baseID := deviceID
	if deviceConfigID != "" {
		baseID = deviceConfigID
	}
	return c.set(keyBase(dim, baseID), deviceInfos)
}

// SetCacheByDeviceIdWithNoTask 显式缓存“当前设备无自动化任务”，避免短时间内重复查库。
func (c *Cache) SetCacheByDeviceIdWithNoTask(deviceID, deviceConfigID string) error {
	baseID := deviceID
	if deviceConfigID != "" {
		baseID = deviceConfigID
	}
	return c.set(keyBase(dimensionFor(deviceConfigID), baseID), ContentNoTask)
}
