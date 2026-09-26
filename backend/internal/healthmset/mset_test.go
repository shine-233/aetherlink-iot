// 文件用途：healthmset 训练/推理核心的单元测试——TP-21 验收的三条主路径全部钉死在这里。
// 核心逻辑：合成正常样本训练后，均值样本低偏差、远端异常样本高偏差、单调性成立；
// 奇异矩阵（列共线）走 ErrSingularCovariance → Evaluate 降级 singular_matrix；
// 样本不足/冷启动/非法输入逐一断言降级原因；求逆正确性与卡方 CDF 已知分位数做数值锚点。
// 关键注意事项：合成噪声用内置 LCG（固定种子、逐版本确定），不用全局 math/rand，
// 保证断言在任何工具链下可复现；卡方已知值容差 1e-9，是评分映射口径的权威锚点。
// 重构建议：引入新特征工程（如差分特征）时，在本文件补对应分布下的偏差区分度用例。
package healthmset

import (
	"errors"
	"math"
	"testing"
)

// lcgNoise 确定性线性同余噪声源，输出 [-1, 1)。
func lcgNoise(seed uint64) func() float64 {
	state := seed
	return func() float64 {
		state = state*6364136223846793005 + 1442695040888963407
		return float64(int64(state>>11))/float64(1<<53)*2 - 1
	}
}

// synthesizeNormalSamples 合成两特征相关正常样本：x=50+2u，y=40+1.6u+1.2v。
func synthesizeNormalSamples(n int, seed uint64) [][]float64 {
	noise := lcgNoise(seed)
	samples := make([][]float64, 0, n)
	for i := 0; i < n; i++ {
		u := noise()
		v := noise()
		samples = append(samples, []float64{50 + 2*u, 40 + 1.6*u + 1.2*v})
	}
	return samples
}

func offsetSample(model *Model, dx, dy float64) []float64 {
	return []float64{model.Mean[0] + dx, model.Mean[1] + dy}
}

func TestTrainAndScoreNormalVsAbnormal(t *testing.T) {
	keys := []string{"temperature", "humidity"}
	samples := synthesizeNormalSamples(300, 20260925)
	model, err := Train(Config{}, keys, samples)
	if err != nil {
		t.Fatalf("train on normal samples: %v", err)
	}
	if model.Samples != 300 || model.Dim() != 2 {
		t.Fatalf("unexpected model shape: samples=%d dim=%d", model.Samples, model.Dim())
	}

	// 均值样本：马氏距离恰为 0 → 偏差分 0。
	atMean, err := model.Score(append([]float64(nil), model.Mean...))
	if err != nil {
		t.Fatalf("score at mean: %v", err)
	}
	if !atMean.Applied || atMean.DeviationScore != 0 {
		t.Fatalf("mean sample should be zero deviation, got %+v", atMean)
	}

	// 贴近基线的正常样本：低偏差。
	near, err := model.Score(offsetSample(model, 0.3, 0.3))
	if err != nil {
		t.Fatalf("score near mean: %v", err)
	}
	if near.DeviationScore >= 30 {
		t.Fatalf("near-mean sample deviation too high: %.2f", near.DeviationScore)
	}

	// 联合远离基线的异常样本：高偏差。
	far, err := model.Score(offsetSample(model, 6, 6))
	if err != nil {
		t.Fatalf("score far from mean: %v", err)
	}
	if far.DeviationScore <= 90 {
		t.Fatalf("far sample deviation too low: %.2f", far.DeviationScore)
	}

	// 单调性：越偏离基线偏差分越高。
	mid, err := model.Score(offsetSample(model, 2, 2))
	if err != nil {
		t.Fatalf("score mid: %v", err)
	}
	if !(near.DeviationScore < mid.DeviationScore && mid.DeviationScore < far.DeviationScore) {
		t.Fatalf("deviation not monotonic: near=%.2f mid=%.2f far=%.2f", near.DeviationScore, mid.DeviationScore, far.DeviationScore)
	}
	if far.Mahalanobis <= near.Mahalanobis {
		t.Fatalf("mahalanobis distance not monotonic: near=%.6f far=%.6f", near.Mahalanobis, far.Mahalanobis)
	}
}

func TestTrainMaxFeaturesBoundary(t *testing.T) {
	keys := make([]string, MaxFeatures)
	for i := range keys {
		keys[i] = "f"
	}
	keys[0] = "f0" // 全部同名不影响训练，但起个可读名
	for i := range keys {
		keys[i] = string(rune('a'+i)) + "_feat"
	}
	samples := make([][]float64, 0, 32)
	noise := lcgNoise(7)
	for i := 0; i < 32; i++ {
		row := make([]float64, MaxFeatures)
		for j := range row {
			row[j] = 10 + float64(j) + noise()
		}
		samples = append(samples, row)
	}
	model, err := Train(Config{}, keys, samples)
	if err != nil {
		t.Fatalf("train with %d features: %v", MaxFeatures, err)
	}
	if model.Dim() != MaxFeatures {
		t.Fatalf("dim = %d, want %d", model.Dim(), MaxFeatures)
	}
	// 推理样本在训练窗内取第 0 行（必为基线内）→ 偏差分应显著低于越界样本。
	inWindow, err := model.Score(samples[0])
	if err != nil {
		t.Fatalf("score in-window sample: %v", err)
	}
	outlier := append([]float64(nil), model.Mean...)
	for j := range outlier {
		outlier[j] += 8
	}
	out, err := model.Score(outlier)
	if err != nil {
		t.Fatalf("score outlier: %v", err)
	}
	if inWindow.DeviationScore >= out.DeviationScore {
		t.Fatalf("8-feature model failed to separate: in=%.2f out=%.2f", inWindow.DeviationScore, out.DeviationScore)
	}
}

func TestTrainRejectsInvalidInputs(t *testing.T) {
	samples := synthesizeNormalSamples(20, 1)
	cases := []struct {
		name    string
		keys    []string
		samples [][]float64
		want    error
	}{
		{"no keys", nil, samples, ErrNoFeatures},
		{"too many features", []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}, samples[:1], ErrTooManyFeatures},
		{"width mismatch", []string{"a", "b"}, [][]float64{{1, 2}, {3}}, ErrSampleWidthMismatch},
		{"nan value", []string{"a", "b"}, [][]float64{{1, 2}, {3, math.NaN()}}, ErrNonFiniteValue},
		{"inf value", []string{"a", "b"}, [][]float64{{1, 2}, {3, math.Inf(1)}}, ErrNonFiniteValue},
		{"too few samples", []string{"a", "b"}, samples[:15], ErrInsufficientSamples},
		{"samples not more than features", []string{"a", "b", "c", "d"}, narrowSamples(4, 4), ErrInsufficientSamples},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Train(Config{}, tc.keys, tc.samples)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}

	// MinSamples 配置生效：显式放宽后同样本量可训练。
	if _, err := Train(Config{MinSamples: 18}, []string{"a", "b"}, samples[:18]); err != nil {
		t.Fatalf("custom MinSamples should be honored: %v", err)
	}
	// MinSamples 配置低于统计下限（M+1）时被硬下限顶住。
	if _, err := Train(Config{MinSamples: 2}, []string{"a", "b", "c"}, narrowSamples(3, 3)); !errors.Is(err, ErrInsufficientSamples) {
		t.Fatalf("statistical floor should override tiny config: %v", err)
	}
}

// narrowSamples 生成 width 列、n 行的平凡样本（第 j 列值 = j），避免用宽样本误触宽度校验。
func narrowSamples(n, width int) [][]float64 {
	samples := make([][]float64, 0, n)
	for i := 0; i < n; i++ {
		row := make([]float64, width)
		for j := range row {
			row[j] = float64(j) + float64(i)*0.1
		}
		samples = append(samples, row)
	}
	return samples
}

func TestScoreRejectsInvalidSample(t *testing.T) {
	model, err := Train(Config{}, []string{"a", "b"}, synthesizeNormalSamples(20, 2))
	if err != nil {
		t.Fatalf("train: %v", err)
	}
	if _, err := model.Score([]float64{1}); !errors.Is(err, ErrSampleWidthMismatch) {
		t.Fatalf("width mismatch err = %v", err)
	}
	if _, err := model.Score([]float64{1, math.NaN()}); !errors.Is(err, ErrNonFiniteValue) {
		t.Fatalf("nan err = %v", err)
	}
	var nilModel *Model
	if _, err := nilModel.Score([]float64{1, 2}); !errors.Is(err, ErrNilModel) {
		t.Fatalf("nil model err = %v", err)
	}
}

func TestInvertMatrixKnownInverse(t *testing.T) {
	inverse, err := invertMatrix([][]float64{{2, 1}, {1, 2}})
	if err != nil {
		t.Fatalf("invert 2x2: %v", err)
	}
	want := [][]float64{{2.0 / 3, -1.0 / 3}, {-1.0 / 3, 2.0 / 3}}
	for i := range want {
		for j := range want[i] {
			if math.Abs(inverse[i][j]-want[i][j]) > 1e-12 {
				t.Fatalf("inverse[%d][%d] = %g, want %g", i, j, inverse[i][j], want[i][j])
			}
		}
	}

	// 一般 SPD 矩阵：A·A⁻¹ ≈ I。
	a := [][]float64{{4, 2, 2}, {2, 5, 1}, {2, 1, 6}}
	inverse3, err := invertMatrix(a)
	if err != nil {
		t.Fatalf("invert 3x3: %v", err)
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			acc := 0.0
			for k := 0; k < 3; k++ {
				acc += a[i][k] * inverse3[k][j]
			}
			expect := 0.0
			if i == j {
				expect = 1
			}
			if math.Abs(acc-expect) > 1e-10 {
				t.Fatalf("(A·A⁻¹)[%d][%d] = %g, want %g", i, j, acc, expect)
			}
		}
	}
}

func TestInvertMatrixSingular(t *testing.T) {
	cases := [][][]float64{
		{{1, 2}, {2, 4}},                  // 行线性相关
		{{0, 0}, {0, 0}},                  // 全零
		{{1, 2, 3}, {2, 4, 6}, {1, 1, 1}}, // 列共线
	}
	for i, matrix := range cases {
		if _, err := invertMatrix(matrix); !errors.Is(err, ErrSingularCovariance) {
			t.Fatalf("case %d: err = %v, want ErrSingularCovariance", i, err)
		}
	}
}

func TestTrainSingularHistoryDegrades(t *testing.T) {
	// 两特征完全共线（y = x）：样本协方差必奇异。
	samples := make([][]float64, 0, 30)
	noise := lcgNoise(42)
	for i := 0; i < 30; i++ {
		v := 20 + 5*noise()
		samples = append(samples, []float64{v, v})
	}
	if _, err := Train(Config{}, []string{"a", "b"}, samples); !errors.Is(err, ErrSingularCovariance) {
		t.Fatalf("collinear features should be singular, err = %v", err)
	}
}

func TestEvaluateFailClosedPaths(t *testing.T) {
	good := synthesizeNormalSamples(30, 5)
	current := []float64{50, 40}

	cases := []struct {
		name       string
		keys       []string
		history    [][]float64
		current    []float64
		wantReason string
		wantApplid bool
	}{
		{"cold start", []string{"a", "b"}, nil, current, ReasonColdStart, false},
		{"insufficient samples", []string{"a", "b"}, good[:15], current, ReasonInsufficientSamples, false},
		{"singular matrix", []string{"a", "b"}, collinearSamples(30), current, ReasonSingularMatrix, false},
		{"invalid config", nil, good, current, ReasonInvalidConfig, false},
		{"current width mismatch", []string{"a", "b"}, good, []float64{50}, ReasonInvalidSample, false},
		{"current nan", []string{"a", "b"}, good, []float64{50, math.NaN()}, ReasonInvalidSample, false},
		{"happy path", []string{"a", "b"}, good, current, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Evaluate(Config{}, tc.keys, tc.history, tc.current)
			if got.Applied != tc.wantApplid {
				t.Fatalf("applied = %v, want %v (result %+v)", got.Applied, tc.wantApplid, got)
			}
			if got.DegradeReason != tc.wantReason {
				t.Fatalf("reason = %q, want %q", got.DegradeReason, tc.wantReason)
			}
			if got.Degraded == tc.wantApplid {
				t.Fatalf("degraded flag inconsistent with applied: %+v", got)
			}
			if !tc.wantApplid && got.DeviationScore != 0 {
				t.Fatalf("degraded result must be neutral (score=0), got %v", got.DeviationScore)
			}
		})
	}

	// 正常路径再验一遍偏差分方向：基线内低、基线外高。
	inference := Evaluate(Config{}, []string{"a", "b"}, good, []float64{50, 40})
	if inference.DeviationScore >= 30 {
		t.Fatalf("normal current should be low deviation, got %.2f", inference.DeviationScore)
	}
	abnormal := Evaluate(Config{}, []string{"a", "b"}, good, []float64{56, 46})
	if abnormal.DeviationScore <= 90 {
		t.Fatalf("abnormal current should be high deviation, got %.2f", abnormal.DeviationScore)
	}
}

func collinearSamples(n int) [][]float64 {
	samples := make([][]float64, 0, n)
	noise := lcgNoise(99)
	for i := 0; i < n; i++ {
		v := 20 + 5*noise()
		samples = append(samples, []float64{v, v})
	}
	return samples
}

func TestChiSquareCDFKnownValues(t *testing.T) {
	// 偶数自由度闭式参照：P(χ²(2m) ≤ x) = 1 - e^(-x/2)·Σ_{i=0}^{m-1} (x/2)^i / i!。
	evenCDF := func(k int, x float64) float64 {
		h := x / 2
		sum := 1.0
		term := 1.0
		for i := 1; i <= k/2-1; i++ {
			term *= h / float64(i)
			sum += term
		}
		return 1 - math.Exp(-h)*sum
	}
	cases := []struct {
		k    int
		x    float64
		want float64
	}{
		{1, 0, 0},
		{1, 3.841458820694124, 0.95},                            // χ²(1) 95% 分位（常数精度足够）
		{1, 1, math.Erf(math.Sqrt(0.5))},                        // CDF=erf(√(x/2))，级数分支
		{2, 2, evenCDF(2, 2)},                                   // 1 - e⁻¹
		{2, 5.991464547107979, 0.95},                            // χ²(2) 95% 分位
		{4, 9.487729036781154, 0.95},                            // χ²(4) 95% 分位
		{4, 0.5, evenCDF(4, 0.5)},                               // 级数分支
		{8, 15.5073104865872, evenCDF(8, 15.5073104865872)},     // 连分式分支
		{8, 1.3444190944700547, evenCDF(8, 1.3444190944700547)}, // 级数分支
	}
	for _, tc := range cases {
		got := chiSquareCDF(tc.k, tc.x)
		if math.Abs(got-tc.want) > 1e-9 {
			t.Fatalf("chiSquareCDF(%d, %v) = %.15f, want %.15f", tc.k, tc.x, got, tc.want)
		}
	}
	// 单调性抽查：x 越大 CDF 越大。
	if !(chiSquareCDF(3, 1) < chiSquareCDF(3, 5) && chiSquareCDF(3, 5) < chiSquareCDF(3, 20)) {
		t.Fatal("chiSquareCDF should be monotonic in x")
	}
	if got := chiSquareCDF(0, 5); got != 0 {
		t.Fatalf("k=0 should give 0, got %v", got)
	}
}

func TestLnGammaKnownValues(t *testing.T) {
	cases := []struct {
		x    float64
		want float64
	}{
		{0.5, 0.5723649429247001}, // ln √π
		{1, 0},
		{2, 0},
		{4, 1.791759469228055}, // ln 6
	}
	for _, tc := range cases {
		got := lnGamma(tc.x)
		if math.Abs(got-tc.want) > 1e-9 {
			t.Fatalf("lnGamma(%v) = %.15f, want %.15f", tc.x, got, tc.want)
		}
	}
}

func TestNeutralInference(t *testing.T) {
	neutral := Neutral(ReasonColdStart)
	if !neutral.Degraded || neutral.Applied || neutral.DeviationScore != 0 || neutral.DegradeReason != ReasonColdStart {
		t.Fatalf("neutral inference malformed: %+v", neutral)
	}
}
