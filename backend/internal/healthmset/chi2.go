// 文件用途：卡方分布 CDF——MSET 马氏距离到 0~100 偏差评分的统计映射基础（零第三方依赖）。
// 核心逻辑：χ²(k) 的 CDF 等于正则化下不完全伽马函数 P(k/2, x/2)；x < a+1 走泰勒级数
// （gser），否则走修正 Lentz 连分式（gcf），lnΓ 用 Lanczos 近似（g=7，9 系数）。
// 关键注意事项：马氏距离平方在多元正态假设下近似服从 χ²(M)，故 CDF 值即"该样本在此
// 历史分布下的分位数"——均值附近→0 分，越偏离越接近 100；这是评分单调性与可解释性的依据。
// 重构建议：若后续需要暴露"偏差分位阈值"配置（如 95 分位截断），在本文件补 χ² 反函数即可，
// 评分映射主体不必改动。
package healthmset

import "math"

// chiSquareCDF 自由度 k 的卡方分布在 x 处的累积概率 P(χ²(k) ≤ x)。
// k<=0 或 x<=0 返回 0（x=0 处 CDF 恒为 0，与"贴合均值即零偏差"的评分语义一致）。
func chiSquareCDF(k int, x float64) float64 {
	if k <= 0 || x <= 0 {
		return 0
	}
	return regularizedLowerGamma(float64(k)/2, x/2)
}

// regularizedLowerGamma 正则化下不完全伽马函数 P(a,x) ∈ [0,1]。
func regularizedLowerGamma(a, x float64) float64 {
	if a <= 0 || x <= 0 {
		return 0
	}
	if x < a+1 {
		return lowerGammaSeries(a, x)
	}
	return 1 - upperGammaContinuedFraction(a, x)
}

// lowerGammaSeries 级数法求 P(a,x)（适用于 x < a+1，收敛快）。
func lowerGammaSeries(a, x float64) float64 {
	const maxIterations = 1000
	const epsilon = 1e-15
	sum := 1 / a
	term := sum
	ap := a
	for i := 0; i < maxIterations; i++ {
		ap++
		term *= x / ap
		sum += term
		if math.Abs(term) < math.Abs(sum)*epsilon {
			break
		}
	}
	return sum * math.Exp(-x+a*math.Log(x)-lnGamma(a))
}

// upperGammaContinuedFraction 修正 Lentz 连分式求 Q(a,x)=Γ(a,x)/Γ(a)（适用于 x >= a+1）。
func upperGammaContinuedFraction(a, x float64) float64 {
	const maxIterations = 1000
	const epsilon = 1e-15
	const fpMin = 1e-300 // 防 0/0 的极小哨兵
	b := x + 1 - a
	c := 1 / fpMin
	d := 1 / b
	if math.IsInf(d, 0) || math.IsNaN(d) {
		return 1 // x 相对 a 极大时 Q→1（P→0），直接按上界处理
	}
	h := d
	for i := 1; i <= maxIterations; i++ {
		an := -float64(i) * (float64(i) - a)
		b += 2
		d = an*d + b
		if math.Abs(d) < fpMin {
			d = fpMin
		}
		c = b + an/c
		if math.Abs(c) < fpMin {
			c = fpMin
		}
		d = 1 / d
		del := d * c
		h *= del
		if math.Abs(del-1) < epsilon {
			break
		}
	}
	return math.Exp(-x+a*math.Log(x)-lnGamma(a)) * h
}

// lnGamma 伽马函数的自然对数（Lanczos 近似，g=7）。
// 本包入参 a=k/2∈[0.5,4]，恒为正，无需反射公式分支，但实现保持一般性以便复用。
func lnGamma(x float64) float64 {
	const lanczosG = 7
	coefficients := [9]float64{
		0.99999999999980993,
		676.5203681218851,
		-1259.1392167224028,
		771.32342877765313,
		-176.61502916214059,
		12.507343278686905,
		-0.13857109526572012,
		9.9843695780195716e-6,
		1.5056327351493116e-7,
	}
	if x < 0.5 {
		// 反射公式：Γ(x)Γ(1-x) = π / sin(πx)
		return math.Log(math.Pi / (math.Sin(math.Pi*x) * math.Exp(lnGamma(1-x))))
	}
	x--
	a := coefficients[0]
	t := x + float64(lanczosG) + 0.5
	for i := 1; i < lanczosG+2; i++ {
		a += coefficients[i] / (x + float64(i))
	}
	return 0.5*math.Log(2*math.Pi) + (x+0.5)*math.Log(t) - t + math.Log(a)
}
