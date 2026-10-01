// 文件用途：TP-21 MSET（多元状态估计）健康特征核心——训练与推理的纯逻辑包（零 DB、零第三方依赖）。
// 核心逻辑：健康历史窗口 N×M 矩阵（N 样本 × M 特征，M≤8）训练出均值向量、样本协方差矩阵与逆矩阵
// （列主元高斯-约当消元手写求逆）；推理对单样本计算马氏距离 d²，经自由度 M 的卡方分布 CDF
// （正则化不完全伽马函数：级数 + Lentz 连分式）映射为 0~100 偏差评分（0=贴合历史基线，100=极端偏离）。
// 关键注意事项：fail-closed——冷启动/样本不足/奇异矩阵/非法输入一律不硬算，统一经 Evaluate 降级为
// 中性分（偏差 0）并记原因；NaN/Inf 在训练与推理两侧都被拒绝；奇异判定用相对阈值（1e-12×矩阵尺度），
// 因此"真实方差极小"的特征也会被判奇异而降级，这是刻意的保守口径，不做收缩正则来掩盖。
// 重构建议：训练产物目前无状态、每次评估即时重算；若后续引入模型持久化/滚动更新（需迁移配合），
// 把 Model 的序列化与增量更新拆成独立文件，本文件的数学核心保持无状态纯函数。
package healthmset

import (
	"errors"
	"fmt"
	"math"
)

// 特征维度与样本量约束。MSET 依赖样本协方差满秩，维数必须小而稳。
const (
	// MaxFeatures 单模型最大特征维数（M≤8，保住小矩阵求逆的数值稳定性）。
	MaxFeatures = 8
	// DefaultMinSamples 默认最少训练样本数（配置可覆盖；下限仍受 "样本数 > 特征数" 统计约束）。
	DefaultMinSamples = 16
)

// 降级原因词表（fail-closed 降级时写入 DegradeReason，服务侧沿用同一词表）。
const (
	ReasonColdStart           = "cold_start"             // 无任何可用历史样本
	ReasonInsufficientSamples = "insufficient_samples"   // 历史样本少于训练下限
	ReasonSingularMatrix      = "singular_matrix"        // 样本协方差矩阵奇异（含特征完全共线/零方差）
	ReasonInvalidSample       = "invalid_sample"         // 推理样本与模型不符或含非有限值
	ReasonInvalidConfig       = "invalid_feature_config" // 特征键数不合法（0 个或超过 MaxFeatures）
)

// 哨兵错误：调用方可用 errors.Is 区分降级路径。
var (
	ErrNoFeatures          = errors.New("healthmset: no feature keys")
	ErrTooManyFeatures     = errors.New("healthmset: too many features")
	ErrSampleWidthMismatch = errors.New("healthmset: sample width mismatch")
	ErrNonFiniteValue      = errors.New("healthmset: non-finite value")
	ErrInsufficientSamples = errors.New("healthmset: insufficient training samples")
	ErrSingularCovariance  = errors.New("healthmset: singular covariance matrix")
	ErrNilModel            = errors.New("healthmset: model is nil")
)

// singularRelativeEpsilon 奇异判定的相对阈值：主元绝对值 <= 矩阵尺度×该值 视为奇异。
// 8 阶以内的双精度消元舍入噪声量级约 1e-15~1e-16，取 1e-12 安全地高于噪声、远低于任何真实主元。
const singularRelativeEpsilon = 1e-12

// Config 训练配置。零值合法（MinSamples<=0 时取 DefaultMinSamples）。
type Config struct {
	MinSamples int
}

// Model 训练产物：均值向量 + 样本协方差矩阵 + 其逆矩阵（全量导出便于测试与诊断）。
type Model struct {
	FeatureKeys []string    `json:"feature_keys"`
	Mean        []float64   `json:"mean"`
	Covariance  [][]float64 `json:"covariance"`
	Inverse     [][]float64 `json:"inverse"`
	Samples     int         `json:"samples"`
}

// Dim 特征维数 M。
func (m *Model) Dim() int {
	if m == nil {
		return 0
	}
	return len(m.Mean)
}

// Inference 单次推理结果。降级时 Applied=false、Degraded=true、DeviationScore 恒为 0（中性）。
type Inference struct {
	Applied        bool    `json:"applied"`                  // 是否完成一次有效推理
	Degraded       bool    `json:"degraded"`                 // fail-closed 降级（未产生有效判定）
	DegradeReason  string  `json:"degrade_reason,omitempty"` // 降级原因（见 Reason* 词表）
	DeviationScore float64 `json:"deviation_score"`          // 0~100 偏差评分
	Mahalanobis    float64 `json:"mahalanobis,omitempty"`    // 马氏距离 d（开方后的距离，非平方）
}

// Neutral 构造一个指定原因的中性降级结果。供服务侧在算法包之外（如取数失败）复用同一词表与口径。
func Neutral(reason string) Inference {
	return Inference{Degraded: true, DegradeReason: reason}
}

// Train 用 N×M 健康历史窗口训练 MSET 模型。
// samples 为行优先矩阵（每行一个样本）；任何校验失败返回哨兵错误，不做部分成功。
func Train(cfg Config, featureKeys []string, samples [][]float64) (*Model, error) {
	m := len(featureKeys)
	if m == 0 {
		return nil, ErrNoFeatures
	}
	if m > MaxFeatures {
		return nil, fmt.Errorf("%w: got %d feature keys, max %d", ErrTooManyFeatures, m, MaxFeatures)
	}
	// 行形状与数值合法性先于样本量检查：坏行是调用方缺陷，样本再多也不许混进训练集。
	for i, row := range samples {
		if len(row) != m {
			return nil, fmt.Errorf("%w: sample %d has %d values, want %d", ErrSampleWidthMismatch, i, len(row), m)
		}
		for j, v := range row {
			if !finite(v) {
				return nil, fmt.Errorf("%w: sample %d feature %d", ErrNonFiniteValue, i, j)
			}
		}
	}
	n := len(samples)
	minSamples := cfg.MinSamples
	if minSamples <= 0 {
		minSamples = DefaultMinSamples
	}
	// 统计硬下限：N 个样本的协方差秩最多 N-1，必须 N > M 才可能满秩；
	// 配置的 minSamples 低于 M+1 时按 M+1 执行（不拦住"配置给了个过小值"这种坑）。
	effectiveMin := minSamples
	if effectiveMin < m+1 {
		effectiveMin = m + 1
	}
	if n < effectiveMin {
		return nil, fmt.Errorf("%w: got %d samples, need at least %d (features=%d)", ErrInsufficientSamples, n, effectiveMin, m)
	}

	mean := make([]float64, m)
	for _, row := range samples {
		for j, v := range row {
			mean[j] += v
		}
	}
	for j := range mean {
		mean[j] /= float64(n)
	}

	// 样本协方差（n-1 无偏口径），按对称性只算上三角再镜像。
	cov := make([][]float64, m)
	for i := range cov {
		cov[i] = make([]float64, m)
	}
	for _, row := range samples {
		for i := 0; i < m; i++ {
			di := row[i] - mean[i]
			for j := i; j < m; j++ {
				cov[i][j] += di * (row[j] - mean[j])
			}
		}
	}
	for i := 0; i < m; i++ {
		for j := i; j < m; j++ {
			cov[i][j] /= float64(n - 1)
			cov[j][i] = cov[i][j]
		}
	}

	inverse, err := invertMatrix(cov)
	if err != nil {
		return nil, err
	}
	return &Model{
		FeatureKeys: append([]string(nil), featureKeys...),
		Mean:        mean,
		Covariance:  cov,
		Inverse:     inverse,
		Samples:     n,
	}, nil
}

// Score 用训练好的模型对单个样本推理：马氏距离 d² → χ²(M) CDF → 0~100 偏差评分。
func (m *Model) Score(sample []float64) (Inference, error) {
	if m == nil {
		return Neutral(ReasonInvalidSample), ErrNilModel
	}
	dim := len(m.Mean)
	if len(sample) != dim {
		return Neutral(ReasonInvalidSample), fmt.Errorf("%w: sample has %d values, model has %d features", ErrSampleWidthMismatch, len(sample), dim)
	}
	for j, v := range sample {
		if !finite(v) {
			return Neutral(ReasonInvalidSample), fmt.Errorf("%w: sample feature %d", ErrNonFiniteValue, j)
		}
	}

	var d2 float64
	for i := 0; i < dim; i++ {
		di := sample[i] - m.Mean[i]
		for j := 0; j < dim; j++ {
			d2 += di * m.Inverse[i][j] * (sample[j] - m.Mean[j])
		}
	}
	if d2 < 0 {
		// 逆矩阵数值误差可能给出 -1e-18 量级的负值，钳回 0；更大的负值同样按 0 处理（保守方向）。
		d2 = 0
	}
	score := 0.0
	if d2 > 0 {
		score = clamp01(chiSquareCDF(dim, d2)) * 100
		score = math.Round(score*100) / 100
	}
	return Inference{
		Applied:        true,
		DeviationScore: score,
		Mahalanobis:    math.Round(math.Sqrt(d2)*1e6) / 1e6,
	}, nil
}

// Evaluate 训练+推理一体的 fail-closed 门面：history 训练、current 推理；
// 任何失败都降级为中性分并给出原因，绝不 panic、绝不返回部分有效的偏差分。
func Evaluate(cfg Config, featureKeys []string, history [][]float64, current []float64) Inference {
	// 配置合法性最优先：特征键数不合法时，后续一切判断都没有意义。
	if len(featureKeys) == 0 || len(featureKeys) > MaxFeatures {
		return Neutral(ReasonInvalidConfig)
	}
	if len(history) == 0 {
		return Neutral(ReasonColdStart)
	}
	if len(current) != len(featureKeys) {
		return Neutral(ReasonInvalidSample)
	}
	for _, v := range current {
		if !finite(v) {
			return Neutral(ReasonInvalidSample)
		}
	}
	model, err := Train(cfg, featureKeys, history)
	if err != nil {
		switch {
		case errors.Is(err, ErrInsufficientSamples):
			return Neutral(ReasonInsufficientSamples)
		case errors.Is(err, ErrSingularCovariance):
			return Neutral(ReasonSingularMatrix)
		case errors.Is(err, ErrNoFeatures), errors.Is(err, ErrTooManyFeatures):
			return Neutral(ReasonInvalidConfig)
		default: // 宽度不符 / 非有限值
			return Neutral(ReasonInvalidSample)
		}
	}
	inference, err := model.Score(current)
	if err != nil {
		return Neutral(ReasonInvalidSample)
	}
	return inference
}

// finite 数值有限性守卫：NaN 与 ±Inf 一律拒绝（训练侧防污染，推理侧防误判）。
func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
