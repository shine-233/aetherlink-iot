// 文件用途：healthmset 训练窗口对齐管道（AlignMatrix）的单元测试。
// 核心逻辑：钉死"完整桶才算样本、行按时间升序、X 统一 floor 到桶边界、非有限值整桶丢弃"
// 四条口径；这些口径直接决定训练矩阵的形状，改语义必须先改这里。
// 关键注意事项：列顺序由 seriesKeys 决定而非 map 遍历序——Go map 遍历无序，调用方必须
// 显式给 keys；本测试同时验证乱序键下每列取值正确。
// 重构建议：支持容差对齐后补"最近邻匹配"用例。
package healthmset

import (
	"math"
	"reflect"
	"testing"
)

func TestAlignMatrixCompleteRowsOnly(t *testing.T) {
	bucketMs := int64(60000)
	series := map[string][]Point{
		"temp": {
			{X: 0, Y: 20},
			{X: 60000, Y: 21},
			{X: 120000, Y: 22},
		},
		"humi": {
			{X: 0, Y: 50},
			// 60000 桶缺 humi → 该行整行丢弃
			{X: 120000, Y: 52},
		},
	}
	got := AlignMatrix(bucketMs, []string{"temp", "humi"}, series)
	want := [][]float64{{20, 50}, {22, 52}}
	if !reflect.DeepEqual(got.Rows, want) {
		t.Fatalf("rows = %v, want %v", got.Rows, want)
	}
	if !reflect.DeepEqual(got.Keys, []string{"temp", "humi"}) {
		t.Fatalf("keys = %v", got.Keys)
	}
}

func TestAlignMatrixFloorsTimestampsAndKeepsColumnOrder(t *testing.T) {
	bucketMs := int64(60000)
	series := map[string][]Point{
		"b": {{X: 61000, Y: 2}, {X: 121000, Y: 4}}, // 落在 60000/120000 桶
		"a": {{X: 59000, Y: 1}, {X: 119000, Y: 3}}, // 落在 0/60000 桶
	}
	got := AlignMatrix(bucketMs, []string{"a", "b"}, series)
	// 对齐明细：a 的 59000→0 桶(Y=1)、119000→60000 桶(Y=3)；b 的 61000→60000 桶(Y=2)、121000→120000 桶(Y=4)。
	// 0 桶缺 b 丢弃；60000 桶 = {a:3, b:2} 完整；120000 桶缺 a 丢弃。
	want := [][]float64{{3, 2}}
	if !reflect.DeepEqual(got.Rows, want) {
		t.Fatalf("rows = %v, want %v", got.Rows, want)
	}
}

func TestAlignMatrixDropsNonFiniteValues(t *testing.T) {
	bucketMs := int64(1000)
	series := map[string][]Point{
		"a": {{X: 0, Y: 1}, {X: 1000, Y: math.NaN()}, {X: 2000, Y: math.Inf(-1)}, {X: 3000, Y: 4}},
		"b": {{X: 0, Y: 10}, {X: 1000, Y: 11}, {X: 2000, Y: 12}, {X: 3000, Y: 13}},
	}
	got := AlignMatrix(bucketMs, []string{"a", "b"}, series)
	want := [][]float64{{1, 10}, {4, 13}}
	if !reflect.DeepEqual(got.Rows, want) {
		t.Fatalf("rows = %v, want %v", got.Rows, want)
	}
}

func TestAlignMatrixGuards(t *testing.T) {
	series := map[string][]Point{"a": {{X: 0, Y: 1}}}
	if got := AlignMatrix(0, []string{"a"}, series); len(got.Rows) != 0 {
		t.Fatalf("bucketMs<=0 should produce no rows, got %v", got.Rows)
	}
	if got := AlignMatrix(1000, nil, series); len(got.Rows) != 0 {
		t.Fatalf("no keys should produce no rows, got %v", got.Rows)
	}
	if got := AlignMatrix(1000, []string{"a"}, nil); len(got.Rows) != 0 {
		t.Fatalf("nil series should produce no rows, got %v", got.Rows)
	}
}

func TestFloorDiv(t *testing.T) {
	cases := []struct {
		a, b, want int64
	}{
		{61000, 60000, 1},
		{-61000, 60000, -2}, // 向负无穷取整
		{0, 60000, 0},
		{120000, 60000, 2},
	}
	for _, tc := range cases {
		if got := floorDiv(tc.a, tc.b); got != tc.want {
			t.Fatalf("floorDiv(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
