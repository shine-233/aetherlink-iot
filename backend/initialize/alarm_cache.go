// 文件用途：维护告警相关的 Redis 缓存读写规则，供启动后的告警与场景联动流程复用。
// 核心逻辑：围绕 group、device、alarm、scene 四类索引键建立双向映射，支持写入、查询与按维度清理缓存。
// 关键注意事项：缓存键格式与清理顺序会直接影响告警去重和场景回收，调整前需同步核对读写双方。
// 重构建议：后续可将缓存键生成与清理策略拆为独立组件，降低单例全局状态的耦合度。

package initialize

import (
	global "aetherlink-iot/backend/pkg/global"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	pkgerrors "github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

var (
	alarmCache *AlarmCache
	alarmMu    sync.Mutex
)

type AlarmCache struct {
	client   *redis.Client
	expireIn time.Duration
}

const alarmCacheDeleteBatchSize = 500

// 缓存1 aralm_groupid 以场景组id 保存出发告警config_id, 设备id
// 缓存2 aralm_device_id 以设备id  组id'
// 缓存3 alarm_config_id+device_id  以告警id+设备id 保存组id（一个告警在该设备上被哪些分组触发过）
// 缓存4 scene_automation_id 以场景id 保存组id

func NewAlarmCache() *AlarmCache {
	alarmMu.Lock()
	defer alarmMu.Unlock()
	if alarmCache == nil {
		alarmCache = &AlarmCache{
			client:   global.REDIS,
			expireIn: time.Hour * 24 * 6,
		}
	}
	return alarmCache
}

//	{
//	    "scene_automation_id":"scene_automation_id_1",
//	    "alarm_config_id_list": ["alarm_config_id_1","alarm_config_id_2"],
//	    "alarm_device_id_list":["device_id_1"]//通过设备配置触发时才保存
//	}
type AlarmCacheGroup struct {
	SceneAutomationId  string   `json:"scene_automation_id"`
	AlarmConfigIdList  []string `json:"alarm_config_id_list"`
	AlaramDeviceIdList []string `json:"alaram_device_id_list"`
	Contents           []string `json:"contents"`
	// ConditionTrueSince 记录条件“连续成立”的起始 Unix 秒。
	// 分组主记录的生命周期本身就等于一次连续成立窗口：条件首次成立时由
	// SetDevice 建档并写入该时间戳，条件恢复时 ConditionAfterAlarm 会调用
	// DeleteBygroupId 删除主记录，因此下次成立会重新计时。告警配置的
	// trigger_duration 就是基于这个时间戳判断是否已经持续足够久。
	ConditionTrueSince int64 `json:"condition_true_since,omitempty"`
}

func (a *AlarmCacheGroup) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, a)
}

type SliceString []string

func (a *SliceString) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, a)
}

// getCacheKeyByGroupId 生成“分组主记录”缓存键，用于回收时反查设备、告警和场景索引。
func (*AlarmCache) getCacheKeyByGroupId(group_id string) string {
	return fmt.Sprintf("alarm_cache_group_v6_%s", group_id)
}

// getCacheKeyByDevice 生成“设备 -> 告警分组”索引键。
func (*AlarmCache) getCacheKeyByDevice(device_id string) string {
	return fmt.Sprintf("alarm_cach_device_v6_%s", device_id)
}

// getCacheKeyByAlarm 生成“告警配置 + 设备 -> 分组列表”索引键，用于判重和删除。
func (*AlarmCache) getCacheKeyByAlarm(alarm_config_id string, device_id string) string {
	return fmt.Sprintf("alarm_cach_alarm_v6_%s_%s", alarm_config_id, device_id)
}

// getCacheKeyByScene 生成“场景 -> 分组列表”索引键，便于按场景回收。
func (*AlarmCache) getCacheKeyByScene(scene_automation_id string) string {
	return fmt.Sprintf("alarm_cach_scene_v6_%s", scene_automation_id)
}

// set 统一序列化并写入 Redis，确保字符串和结构化值走同一过期策略。
func (a *AlarmCache) set(key string, value interface{}) error {
	var valueStr string
	if val, ok := value.(string); ok {
		valueStr = val
	} else {
		valBytes, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("json marshal failed for key %s: %w", key, err)
		}
		valueStr = string(valBytes)
	}
	logrus.Debug(valueStr)
	return a.client.Set(context.Background(), key, valueStr, a.expireIn).Err()
}

// SetDevice 写入分组下的设备触发信息，并同步维护设备索引和场景索引。
// 首次建档时记录 ConditionTrueSince，已存在的分组保留原有起始时间，
// 从而让“条件连续成立多久”可以在不新增表的前提下被判断。
func (a *AlarmCache) SetDevice(group_id, scene_automation_id string, device_ids, contents []string) error {
	alarmMu.Lock()
	defer alarmMu.Unlock()
	var info AlarmCacheGroup
	now := time.Now().UTC().Unix()
	cacheKey := a.getCacheKeyByGroupId(group_id)
	if count, err := a.client.Exists(context.Background(), cacheKey).Result(); err != nil {
		return pkgerrors.Wrap(err, "检查缓存是否存在失败")
	} else if count > 0 {
		err = a.client.Get(context.Background(), cacheKey).Scan(&info)
		if err != nil {
			return pkgerrors.Wrap(err, "获取缓存失败")
		}
		info.Contents = contents
		// 升级前写入的分组没有该字段，按本次观测时间补一个起点，
		// 避免历史缓存被当成“已持续无限久”而立刻触发。
		if info.ConditionTrueSince == 0 {
			info.ConditionTrueSince = now
		}
	} else {
		info = AlarmCacheGroup{
			SceneAutomationId:  scene_automation_id,
			AlaramDeviceIdList: device_ids,
			Contents:           contents,
			ConditionTrueSince: now,
		}
	}
	logrus.Debugf("AlarmCacheGroupSet:%#v", info)
	err := a.set(cacheKey, info)
	if err != nil {
		return err
	}
	for _, device_id := range device_ids {
		cacheKey = a.getCacheKeyByDevice(device_id)
		err = a.groupCacheAdd(cacheKey, group_id)
		if err != nil {
			return err
		}
	}
	cacheKey = a.getCacheKeyByScene(scene_automation_id)
	logrus.Debug("SetDevice:", cacheKey, "==>", group_id)
	return a.groupCacheAdd(cacheKey, group_id)
}

// GetAlarmDeviceExists 判断告警配置在给定设备列表中是否已被指定条件组登记过。
func (a *AlarmCache) GetAlarmDeviceExists(deviceIds []string, alarmId, groupId string) (bool, error) {
	for _, deviceId := range deviceIds {
		// key 结构: alarm_cach_alarm_v6_<alarmId>_<deviceId>
		cacheKey := a.getCacheKeyByAlarm(alarmId, deviceId)
		var groupIds SliceString
		err := a.client.Get(context.Background(), cacheKey).Scan(&groupIds)
		logrus.Debug("GetAlarmDeviceExists:", cacheKey, "==>", groupIds)
		if err != nil && !errors.Is(err, redis.Nil) {
			return false, err
		}
		for _, g := range groupIds {
			if g == groupId {
				return true, nil
			}
		}
	}
	return false, nil
}

// groupCacheAdd 向索引键追加分组 ID；若已存在则保持幂等不重复写入。
func (a *AlarmCache) groupCacheAdd(cacheKey, groupId string) error {
	var groupIds SliceString
	err := a.client.Get(context.Background(), cacheKey).Scan(&groupIds)
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	var isOk bool
	for _, g := range groupIds {
		if g == groupId {
			isOk = true
			break
		}
	}
	//已经存在 就不加入
	if isOk {
		return nil
	}
	groupIds = append(groupIds, groupId)
	err = a.set(cacheKey, groupIds)
	if err != nil {
		return err
	}
	return nil
}
func (a *AlarmCache) name() {

}

// groupCacheDel 从索引键中移除指定分组；若索引为空则直接删除该键。
func (a *AlarmCache) groupCacheDel(cachekey, group_id string) error {
	var groupIds SliceString
	err := a.client.Get(context.Background(), cachekey).Scan(&groupIds)
	if err != nil && err != redis.Nil {
		return err
	}
	for i, g := range groupIds {
		if g == group_id {
			groupIds = append(groupIds[:i], groupIds[i+1:]...)
		}
	}
	if len(groupIds) > 0 {
		err = a.set(cachekey, groupIds)
	} else {
		err = a.client.Del(context.Background(), cachekey).Err()
	}

	if err != nil {
		return err
	}
	return nil
}

func removeGroupID(groupIds SliceString, groupID string) (SliceString, bool) {
	filtered := groupIds[:0]
	removed := false
	for _, g := range groupIds {
		if g == groupID {
			removed = true
			continue
		}
		filtered = append(filtered, g)
	}
	return filtered, removed
}

func uniqueAlarmCacheKeys(keys []string) []string {
	unique := make([]string, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, key)
	}
	return unique
}

func (a *AlarmCache) groupIndexKeys(info AlarmCacheGroup) []string {
	keys := make([]string, 0, len(info.AlarmConfigIdList)*len(info.AlaramDeviceIdList)+len(info.AlaramDeviceIdList)+1)
	for _, alarmID := range info.AlarmConfigIdList {
		for _, deviceID := range info.AlaramDeviceIdList {
			keys = append(keys, a.getCacheKeyByAlarm(alarmID, deviceID))
		}
	}
	for _, deviceID := range info.AlaramDeviceIdList {
		keys = append(keys, a.getCacheKeyByDevice(deviceID))
	}
	keys = append(keys, a.getCacheKeyByScene(info.SceneAutomationId))
	return uniqueAlarmCacheKeys(keys)
}

func decodeAlarmCacheGroupIDs(value interface{}) (SliceString, error) {
	var groupIds SliceString
	switch val := value.(type) {
	case nil:
		return groupIds, nil
	case string:
		if val == "" {
			return groupIds, nil
		}
		return groupIds, json.Unmarshal([]byte(val), &groupIds)
	case []byte:
		if len(val) == 0 {
			return groupIds, nil
		}
		return groupIds, json.Unmarshal(val, &groupIds)
	default:
		return groupIds, fmt.Errorf("unexpected alarm cache index value type %T", value)
	}
}

func (a *AlarmCache) deleteGroupIndexBatch(ctx context.Context, keys []string, groupID string) error {
	values, err := a.client.MGet(ctx, keys...).Result()
	if err != nil {
		return err
	}

	pipe := a.client.Pipeline()
	commands := 0
	for index, value := range values {
		groupIds, err := decodeAlarmCacheGroupIDs(value)
		if err != nil {
			return err
		}
		groupIds, removed := removeGroupID(groupIds, groupID)
		if !removed {
			continue
		}
		if len(groupIds) > 0 {
			valueBytes, err := json.Marshal(groupIds)
			if err != nil {
				return err
			}
			pipe.Set(ctx, keys[index], string(valueBytes), a.expireIn)
		} else {
			pipe.Del(ctx, keys[index])
		}
		commands++
	}

	if commands == 0 {
		return nil
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (a *AlarmCache) deleteGroupIndexes(ctx context.Context, keys []string, groupID string) error {
	for start := 0; start < len(keys); start += alarmCacheDeleteBatchSize {
		end := start + alarmCacheDeleteBatchSize
		if end > len(keys) {
			end = len(keys)
		}
		if err := a.deleteGroupIndexBatch(ctx, keys[start:end], groupID); err != nil {
			return err
		}
	}
	return nil
}

// SetAlarm 写入分组关联的告警配置，并补齐“告警 + 设备 -> 分组”索引。
func (a *AlarmCache) SetAlarm(group_id string, alarm_config_ids []string, deviceId string) error {
	alarmMu.Lock()
	defer alarmMu.Unlock()
	var info AlarmCacheGroup
	cachekey := a.getCacheKeyByGroupId(group_id)
	err := a.client.Get(context.Background(), cachekey).Scan(&info)
	if err != nil && err != redis.Nil {
		return err
	}
	info.AlarmConfigIdList = alarm_config_ids
	err = a.set(cachekey, info)
	if err != nil {
		return err
	}
	for _, alarm_id := range alarm_config_ids {
		cachekey = a.getCacheKeyByAlarm(alarm_id, deviceId)
		err = a.groupCacheAdd(cachekey, group_id)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetByGroupId 读取单个条件组的告警缓存主记录。
func (a *AlarmCache) GetByGroupId(group_id string) (AlarmCacheGroup, error) {
	var info AlarmCacheGroup
	cachekey := a.getCacheKeyByGroupId(group_id)
	err := a.client.Get(context.Background(), cachekey).Scan(&info)
	if err != nil && err != redis.Nil {
		return info, err
	}
	return info, nil
}

// GetBySceneAutomationId 根据场景 ID 读取其挂接的条件组列表。
func (a *AlarmCache) GetBySceneAutomationId(scene_automation_id string) ([]string, error) {
	var groupIds SliceString
	cachekey := a.getCacheKeyByScene(scene_automation_id)
	err := a.client.Get(context.Background(), cachekey).Scan(&groupIds)
	if err != nil && err != redis.Nil {
		return groupIds, err
	}
	return groupIds, nil
}

// DeleteBygroupId 按分组主记录执行级联清理，保持各类反向索引一致。
func (a *AlarmCache) DeleteBygroupId(group_Id string) error {
	alarmMu.Lock()
	defer alarmMu.Unlock()
	info, err := a.GetByGroupId(group_Id)
	if err != nil {
		return err
	}
	ctx := context.Background()
	if err := a.deleteGroupIndexes(ctx, a.groupIndexKeys(info), group_Id); err != nil {
		return err
	}

	cacheKey := a.getCacheKeyByGroupId(group_Id)

	return a.client.Del(ctx, cacheKey).Err()
}

// DeleteByAlarmId 按告警配置 ID 扫描并删除所有设备维度索引键。
func (a *AlarmCache) DeleteByAlarmId(alarmId string) error {
	alarmMu.Lock()
	defer alarmMu.Unlock()
	pattern := fmt.Sprintf("alarm_cach_alarm_v6_%s_*", alarmId)
	var cursor uint64
	for {
		keys, nextCursor, err := a.client.Scan(context.Background(), cursor, pattern, 100).Result()
		if err != nil {
			return pkgerrors.Wrap(err, "扫描告警缓存key失败")
		}
		if len(keys) > 0 {
			if err := a.client.Del(context.Background(), keys...).Err(); err != nil {
				return pkgerrors.Wrap(err, "删除告警缓存失败")
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}
