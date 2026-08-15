// 文件用途：维护 plugin\admin\utils_test.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package admin

import (
	"container/list"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIndexer(t *testing.T) {
	a := assert.New(t)
	i := NewIndexer()
	for j := 0; j < 100; j++ {
		i.Set(strconv.Itoa(j), j)
		a.EqualValues(j, i.GetByID(strconv.Itoa(j)).Value)
	}
	a.EqualValues(100, i.Len())

	var jj int
	i.Iterate(func(elem *list.Element) {
		v := elem.Value.(int)
		a.Equal(jj, v)
		jj++
	}, 0, uint(i.Len()))

	e := i.Remove("5")
	a.Equal(5, e.Value.(int))

	var rs []int
	i.Iterate(func(elem *list.Element) {
		rs = append(rs, elem.Value.(int))
	}, 4, 2)
	// 5 is removed
	a.Equal([]int{4, 6}, rs)

}
