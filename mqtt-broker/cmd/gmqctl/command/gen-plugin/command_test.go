// 文件用途：验证 `gmqctl gen plugin` 生成器的文件输出和 hook 名称校验。
// 核心逻辑：用临时 testdata 目录运行生成流程，检查主文件、hook 文件和可选配置文件是否符合参数预期。
// 关键注意事项：测试会操作包内全局 flag 变量，新增用例时要显式重置相关状态。
// 重构建议：生成器改为参数结构体后，可把测试改成表驱动并使用 t.TempDir 隔离输出。
package gen_plugin

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRun(t *testing.T) {
	defer os.RemoveAll("./testdata/")
	a := assert.New(t)
	var err error
	os.Mkdir("./testdata/", 0777)
	name = "test"
	hooksStr = "OnBasicAuth"
	config = true
	output = "./testdata"
	a.Nil(run(nil, nil))
	_, err = os.Stat("./testdata/test.go")
	a.Nil(err)
	_, err = os.Stat("./testdata/config.go")
	a.Nil(err)
	_, err = os.Stat("./testdata/hooks.go")
	a.Nil(err)
	a.NotNil(run(nil, nil))
}

func TestRunEmptyHooks(t *testing.T) {
	defer os.RemoveAll("./testdata/")
	a := assert.New(t)
	var err error
	os.Mkdir("./testdata/", 0777)
	name = "test"
	config = true
	output = "./testdata"
	a.Nil(run(nil, nil))
	_, err = os.Stat("./testdata/test.go")
	a.Nil(err)
	_, err = os.Stat("./testdata/config.go")
	a.Nil(err)
	_, err = os.Stat("./testdata/hooks.go")
	a.Nil(err)
}

func TestRunNoConfig(t *testing.T) {
	a := assert.New(t)
	var err error
	defer os.RemoveAll("./testdata/")
	os.Mkdir("./testdata", 0777)
	name = "test"
	hooksStr = "OnBasicAuth"
	config = false
	output = "./testdata"
	run(nil, nil)
	_, err = os.Stat("./testdata/test.go")
	a.Nil(err)
	_, err = os.Stat("./testdata/config.go")
	a.True(os.IsNotExist(err))
	_, err = os.Stat("./testdata/hooks.go")
	a.Nil(err)
}

func TestValidateHookName(t *testing.T) {
	var tt = []struct {
		name  string
		hooks string
		rs    []string
		valid bool
	}{
		{
			name:  "valid",
			hooks: "OnSubscribe, OnSubscribed",
			rs:    []string{"OnSubscribe", "OnSubscribed"},
			valid: true,
		},
		{
			name:  "invalid",
			hooks: "OnAbc,OnDEF",
			rs:    nil,
			valid: false,
		},
	}
	for _, v := range tt {
		t.Run(v.name, func(t *testing.T) {
			a := assert.New(t)
			got, err := ValidateHooks(v.hooks)
			if v.valid {
				a.Nil(err)
			} else {
				a.NotNil(err)
			}
			a.Equal(v.rs, got)
		})
	}

}
