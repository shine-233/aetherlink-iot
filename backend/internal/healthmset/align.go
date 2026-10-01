// 文件用途：MSET 训练窗口的对齐管道——按键分桶的聚合序列对齐成 N×M 样本矩阵（纯函数）。
// 核心逻辑：以"桶起始毫秒"为对齐键，每个桶必须凑齐全部 M 个特征的有限值才算一个完整样本行，
// 行按时间升序返回；缺值桶/含 NaN 或 Inf 的桶整行丢弃（fail-closed：宁少样本不作插补）。
// 关键注意事项：调用方先把 DAL 行转成 Point（X=桶起点毫秒，Y=数值），转换层的类型宽容
// （int/int64/float64/json.Number）属于 DB 形状适配，留在服务层；本函数只认干净类型。
// 重构建议：若后续要支持"按容差最近邻对齐"（键的上报时间天然错开），在 AlignConfig 加
// tolerance 字段扩展匹配规则，矩阵输出形状不变。
package healthmset

import (
	"sort"
)

// Point 单键分桶聚合序列中的一个点：X=桶起点毫秒，Y=聚合值。
type Point struct {
	X int64
	Y float64
}

// AlignResult 对齐产物：keys 为特征键（与输入序列键的给定顺序一致），rows 为完整样本行（升序）。
type AlignResult struct {
	Keys []string
	Rows [][]float64
}

// AlignMatrix 把每个特征键的分桶序列对齐成 N×M 矩阵。
// bucketMs 为桶宽毫秒；序列 X 不保证落在桶边界上（冷层回源可能按查询起点对齐），
// 统一 floor 到 bucketMs 边界后再对齐。seriesKeys 决定特征列顺序（调用方控制，保持稳定）。
func AlignMatrix(bucketMs int64, seriesKeys []string, series map[string][]Point) AlignResult {
	if bucketMs <= 0 || len(seriesKeys) == 0 {
		return AlignResult{Keys: append([]string(nil), seriesKeys...)}
	}

	type bucketRow struct {
		values []float64
		filled []bool
	}
	buckets := make(map[int64]*bucketRow)
	for i, key := range seriesKeys {
		for _, p := range series[key] {
			if !finite(p.Y) {
				continue
			}
			bucket := floorDiv(p.X, bucketMs) * bucketMs
			row := buckets[bucket]
			if row == nil {
				row = &bucketRow{values: make([]float64, len(seriesKeys)), filled: make([]bool, len(seriesKeys))}
				buckets[bucket] = row
			}
			row.values[i] = p.Y
			row.filled[i] = true
		}
	}

	stamps := make([]int64, 0, len(buckets))
	for stamp := range buckets {
		stamps = append(stamps, stamp)
	}
	sort.Slice(stamps, func(i, j int) bool { return stamps[i] < stamps[j] })

	rows := make([][]float64, 0, len(stamps))
	for _, stamp := range stamps {
		row := buckets[stamp]
		complete := true
		for _, ok := range row.filled {
			if !ok {
				complete = false
				break
			}
		}
		if !complete {
			continue // 缺任一特征的桶整行丢弃：不插补、不降维，样本少了由样本不足降级兜底
		}
		sample := append([]float64(nil), row.values...)
		rows = append(rows, sample)
	}
	return AlignResult{Keys: append([]string(nil), seriesKeys...), Rows: rows}
}

// floorDiv Go 的 % 对负数返回负值，这里显式向负无穷取整，保证早于纪元的桶（理论防御）也正确对齐。
func floorDiv(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}
