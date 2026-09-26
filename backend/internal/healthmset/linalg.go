// 文件用途：MSET 专用的小维对称矩阵求逆——手写列主元高斯-约当消元（零第三方依赖）。
// 核心逻辑：构造增广矩阵 [A|I]，逐列选绝对值最大的主元行交换后归一并消元，右侧即逆矩阵；
// 奇异判定用相对阈值（主元绝对值 <= 矩阵最大元素×1e-12），命中即返回 ErrSingularCovariance。
// 关键注意事项：仅用于 M≤8 的样本协方差矩阵——列主元消元对一般矩阵数值稳健，但不做
// LU 分解复用、不做迭代精化，维数放大或病态矩阵场景应换专业线性代数库而非扩写本文件。
// 重构建议：若未来确需更大维数，优先引入带秩披露的分解（如 SVD）替代"求逆失败=奇异"的粗粒度口径。
package healthmset

import (
	"fmt"
	"math"
)

// invertMatrix 列主元高斯-约当消元求逆。输入会被复制，不修改调用方矩阵。
// 返回 ErrSingularCovariance 表示矩阵奇异（含全零矩阵与近似线性相关列）。
func invertMatrix(matrix [][]float64) ([][]float64, error) {
	n := len(matrix)
	if n == 0 {
		return nil, fmt.Errorf("%w: empty matrix", ErrSingularCovariance)
	}
	for _, row := range matrix {
		if len(row) != n {
			return nil, fmt.Errorf("%w: matrix is not square", ErrSingularCovariance)
		}
	}

	// 矩阵尺度：相对奇异阈值锚点，避免量纲大小影响判定。
	scale := 0.0
	for _, row := range matrix {
		for _, v := range row {
			if abs := math.Abs(v); abs > scale {
				scale = abs
			}
		}
	}
	eps := singularRelativeEpsilon * scale

	aug := make([][]float64, n)
	for i := range aug {
		aug[i] = make([]float64, 2*n)
		copy(aug[i], matrix[i])
		aug[i][n+i] = 1 // 单位阵作右半增广
	}

	for col := 0; col < n; col++ {
		// 部分主元：在当前列剩余行里选绝对值最大者，压制舍入误差并顺带发现奇异。
		pivotRow := col
		pivotAbs := math.Abs(aug[col][col])
		for r := col + 1; r < n; r++ {
			if abs := math.Abs(aug[r][col]); abs > pivotAbs {
				pivotAbs = abs
				pivotRow = r
			}
		}
		if pivotAbs <= eps || pivotAbs == 0 {
			return nil, fmt.Errorf("%w: pivot at column %d is ~0 (|pivot|=%g, scale=%g)", ErrSingularCovariance, col, pivotAbs, scale)
		}
		if pivotRow != col {
			aug[col], aug[pivotRow] = aug[pivotRow], aug[col]
		}

		pivot := aug[col][col]
		for j := col; j < 2*n; j++ {
			aug[col][j] /= pivot
		}
		for r := 0; r < n; r++ {
			if r == col {
				continue
			}
			factor := aug[r][col]
			if factor == 0 {
				continue
			}
			for j := col; j < 2*n; j++ {
				aug[r][j] -= factor * aug[col][j]
			}
		}
	}

	inverse := make([][]float64, n)
	for i := range inverse {
		inverse[i] = make([]float64, n)
		copy(inverse[i], aug[i][n:])
	}
	return inverse, nil
}
