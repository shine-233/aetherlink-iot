// 文件用途：实体版本快照的 JSON 语义 diff——纯函数 DiffEntityVersionSnapshots 与
// 服务方法 DiffEntityVersion（ROADMAP TB-25，对标 ThingsBoard 版本控制差异视图）。
// 核心逻辑：递归比较两份快照 JSON——对象按键、数组按下标逐位比较，叶子值不等记
// modified、仅新侧存在记 added、仅旧侧存在记 removed；路径以点号表达（数组下标为
// 路径段，如 tags.0），输出按路径字典序确定排序。
// 关键注意事项：
//   1) 纯函数不触库不入库，可对任意两段 JSON 文本求差，便于单测与复用；
//   2) 数值用 json.Decoder.UseNumber 保留字面量精度（1 与 1.0 视为不同，宁可多报不误删）；
//   3) 同名路径两侧类型不同（对象 vs 数组 vs 标量）按 modified 记在路径本身，不跨类型递归；
//   4) 服务方法读取版本一律走 dal.GetEntityVersionForScope（强制 id+tenant_id），fail-closed；
//      不要求两个版本属于同一实体（两者均在租户作用域内即可），实体归属随响应元信息回显。
// 重构建议：若后续需要行内高亮或字段级语义（如仅看某子树），在此扩展参数化 diff，
// 不要把展示逻辑混进纯函数。
package service

import (
	"encoding/json"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"gorm.io/gorm"
)

// rootDiffPath 根标量差异的路径占位：两侧根都不是容器（或类型不同）时挂在该路径上。
const rootDiffPath = "$"

// 语义变更类别常量，与前端差异视图的标签配色一一对应。
const (
	diffKindAdded    = "added"
	diffKindRemoved  = "removed"
	diffKindModified = "modified"
)

// DiffEntityVersionSnapshots 递归比较两份快照 JSON 文本，返回语义差异。
// 任一侧不是合法 JSON 时返回参数错误；两份完全一致时返回全空列表且 Total=0。
func DiffEntityVersionSnapshots(oldSnapshot, newSnapshot string) (*model.EntityVersionDiffResult, error) {
	oldValue, err := decodeSnapshotJSON(oldSnapshot)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "source snapshot is not valid JSON")
	}
	newValue, err := decodeSnapshotJSON(newSnapshot)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "target snapshot is not valid JSON")
	}

	result := &model.EntityVersionDiffResult{
		Added:    []string{},
		Removed:  []string{},
		Modified: []string{},
		Changes:  []model.EntityVersionDiffChange{},
	}
	diffWalk(oldValue, newValue, "", result)
	return result, nil
}

// decodeSnapshotJSON 解析快照文本；UseNumber 让数值保持 json.Number 字面量。
func decodeSnapshotJSON(text string) (interface{}, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

// diffWalk 递归比较两个值：同为对象或同为数组时下钻，否则按叶子比较。
// prefix 为 "" 表示根路径；根上直接出现的标量/类型差异记在 rootDiffPath。
func diffWalk(oldValue, newValue interface{}, prefix string, out *model.EntityVersionDiffResult) {
	switch oldValue.(type) {
	case map[string]interface{}:
		if newValueMap, ok := newValue.(map[string]interface{}); ok {
			diffMaps(oldValue.(map[string]interface{}), newValueMap, prefix, out)
			return
		}
	case []interface{}:
		if newValueSlice, ok := newValue.([]interface{}); ok {
			diffSlices(oldValue.([]interface{}), newValueSlice, prefix, out)
			return
		}
	}
	if !reflect.DeepEqual(oldValue, newValue) {
		recordDiffChange(out, prefix, diffKindModified, oldValue, newValue)
	}
}

// diffMaps 对象逐键比较：键并集排序后，缺失侧记 added/removed，双侧存在则下钻。
func diffMaps(oldMap, newMap map[string]interface{}, prefix string, out *model.EntityVersionDiffResult) {
	keys := unionKeys(oldMap, newMap)
	sort.Strings(keys)
	for _, key := range keys {
		childPath := joinDiffPath(prefix, key)
		oldChild, inOld := oldMap[key]
		newChild, inNew := newMap[key]
		switch {
		case inOld && inNew:
			diffWalk(oldChild, newChild, childPath, out)
		case inNew:
			recordDiffChange(out, childPath, diffKindAdded, nil, newChild)
		default:
			recordDiffChange(out, childPath, diffKindRemoved, oldChild, nil)
		}
	}
}

// diffSlices 数组按下标逐位比较：下标并集排序后，越界侧记 added/removed，双侧存在则下钻。
// 数组整体替换语义由逐位比较自然产生（如 ["a","b"] -> ["a","c","d"] 报 tags.1 modified + tags.2 added）。
func diffSlices(oldSlice, newSlice []interface{}, prefix string, out *model.EntityVersionDiffResult) {
	indexes := make([]int, 0, len(oldSlice)+len(newSlice))
	seen := make(map[int]struct{}, len(oldSlice)+len(newSlice))
	for index := range oldSlice {
		if _, ok := seen[index]; !ok {
			seen[index] = struct{}{}
			indexes = append(indexes, index)
		}
	}
	for index := range newSlice {
		if _, ok := seen[index]; !ok {
			seen[index] = struct{}{}
			indexes = append(indexes, index)
		}
	}
	sort.Ints(indexes)
	for _, index := range indexes {
		childPath := joinDiffPath(prefix, strconv.Itoa(index))
		switch {
		case index < len(oldSlice) && index < len(newSlice):
			diffWalk(oldSlice[index], newSlice[index], childPath, out)
		case index < len(newSlice):
			recordDiffChange(out, childPath, diffKindAdded, nil, newSlice[index])
		default:
			recordDiffChange(out, childPath, diffKindRemoved, oldSlice[index], nil)
		}
	}
}

// unionKeys 收集两侧 map 的键并集（保持稳定顺序由调用方排序）。
func unionKeys(oldMap, newMap map[string]interface{}) []string {
	seen := make(map[string]struct{}, len(oldMap)+len(newMap))
	keys := make([]string, 0, len(oldMap)+len(newMap))
	for key := range oldMap {
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
	}
	for key := range newMap {
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
	}
	return keys
}

// joinDiffPath 以点号拼接父子路径段；prefix 为 "" 时直接返回子段（根的第一段）。
func joinDiffPath(prefix, segment string) string {
	if prefix == "" {
		return segment
	}
	return prefix + "." + segment
}

// recordDiffChange 登记一条变更：同步写入路径列表、明细列表并累加 Total；
// 根路径（""）归一化为 rootDiffPath，保证路径列表里不出现空串。
func recordDiffChange(out *model.EntityVersionDiffResult, path, kind string, oldValue, newValue interface{}) {
	if path == "" {
		path = rootDiffPath
	}
	switch kind {
	case diffKindAdded:
		out.Added = append(out.Added, path)
	case diffKindRemoved:
		out.Removed = append(out.Removed, path)
	default:
		out.Modified = append(out.Modified, path)
	}
	out.Changes = append(out.Changes, model.EntityVersionDiffChange{
		Path:     path,
		Kind:     kind,
		OldValue: oldValue,
		NewValue: newValue,
	})
	out.Total++
}

// DiffEntityVersion 比较同租户下两个快照版本的语义差异（source 为基准，target 为对比）。
// 两个版本 id 均在路径参数中传递；任一版本不存在或不属于当前租户一律 CodeNotFound。
func (*EntityVersionService) DiffEntityVersion(sourceID, targetID string, claims *utils.UserClaims) (*model.EntityVersionDiffRsp, error) {
	tenantID, err := entityVersionScope(claims)
	if err != nil {
		return nil, err
	}
	sourceID = strings.TrimSpace(sourceID)
	targetID = strings.TrimSpace(targetID)
	if sourceID == "" || targetID == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "source id and target id are required")
	}

	source, err := dal.GetEntityVersionForScope(sourceID, tenantID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errcode.NewWithMessage(errcode.CodeNotFound, "entity version not found")
		}
		return nil, err
	}
	target, err := dal.GetEntityVersionForScope(targetID, tenantID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errcode.NewWithMessage(errcode.CodeNotFound, "entity version not found")
		}
		return nil, err
	}

	diff, err := DiffEntityVersionSnapshots(source.Snapshot, target.Snapshot)
	if err != nil {
		return nil, err
	}
	return &model.EntityVersionDiffRsp{Source: source, Target: target, Diff: diff}, nil
}
