// 文件用途：TB-11 离线 Helm 渲染器——用 text/template 以与 Helm 兼容的最小函数集渲染 chart 模板，
// 供 backend/internal/helmchart 单测对 deploy/helm/aetherlink 产物做 yaml.Unmarshal 与必填键校验。
// 核心逻辑：RenderChart 读取 chartDir/templates 下非 _ 前缀的 *.yaml/*.yml 文件，注入
// Release/Chart/Values 上下文逐文件渲染，返回 文件名→渲染产物 的映射；missingkey=zero 对齐 Helm。
// 关键注意事项：funcMap 各函数（quote/default/required/int/until）语义必须与 sprig 保持一致，
// 且模板不得引入此处未实现的函数——否则单测渲染会直接报错而不是与真实 `helm template` 静默分叉；
// values 由 yaml.v3 解析（数值为 int），真实 Helm 为 float64，故 int/quote 均按多标量类型实现。
// 重构建议：模板将来需要 include/_helpers.tpl/sprig 其余函数时，先在本文件补齐语义并同步单测
// 函数清单，再扩展模板。
package helmchart

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"text/template"
)

// Release 是模板可用的最小 .Release 上下文（对齐 Helm 字段名）。
type Release struct {
	Name      string
	Namespace string
	Revision  int
	IsInstall bool
	IsUpgrade bool
}

// ChartMeta 是模板可用的最小 .Chart 上下文（对齐 Helm 字段名）。
type ChartMeta struct {
	Name       string
	Version    string
	AppVersion string
}

// RenderChart 渲染 chartDir 下 templates 目录的全部 YAML 模板。
// 返回 相对文件名→渲染产物；跳过 _ 前缀文件与 NOTES.txt（与 Helm 行为一致）。
func RenderChart(chartDir string, release Release, chart ChartMeta, values map[string]any) (map[string]string, error) {
	templatesDir := filepath.Join(chartDir, "templates")
	entries, err := os.ReadDir(templatesDir)
	if err != nil {
		return nil, fmt.Errorf("read chart templates dir %s: %w", templatesDir, err)
	}
	// 固定按文件名排序，保证错误信息与断言顺序可复现（os.ReadDir 已排序，显式声明意图）。
	sorted := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), "_") {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		sorted = append(sorted, e.Name())
	}
	sort.Strings(sorted)

	rendered := make(map[string]string, len(sorted))
	for _, name := range sorted {
		raw, err := os.ReadFile(filepath.Join(templatesDir, name))
		if err != nil {
			return nil, fmt.Errorf("read template %s: %w", name, err)
		}
		tmpl, err := template.New(name).Funcs(chartFuncMap()).Option("missingkey=zero").Parse(string(raw))
		if err != nil {
			return nil, fmt.Errorf("parse template %s: %w", name, err)
		}
		var buf bytes.Buffer
		context := map[string]any{
			"Release": release,
			"Chart":   chart,
			"Values":  values,
		}
		if err := tmpl.Execute(&buf, context); err != nil {
			return nil, fmt.Errorf("execute template %s: %w", name, err)
		}
		rendered[name] = buf.String()
	}
	if len(rendered) == 0 {
		return nil, fmt.Errorf("no renderable templates found under %s", templatesDir)
	}
	return rendered, nil
}

// chartFuncMap 返回与模板所用 sprig 函数语义一致的最小函数集。
func chartFuncMap() template.FuncMap {
	return template.FuncMap{
		"quote":    sprigQuote,
		"default":  sprigDefault,
		"required": sprigRequired,
		"int":      sprigInt,
		"until":    sprigUntil,
	}
}

// sprigQuote 对齐 sprig quote：逐参数转成字符串后 %q 包裹，空格连接，nil 渲染为空串。
func sprigQuote(args ...any) string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = strconv.Quote(strval(a))
	}
	return strings.Join(out, " ")
}

// sprigDefault 对齐 sprig default：given 缺省或为空值（nil/空串/0/false/空集合）时返回默认值。
func sprigDefault(d any, given ...any) any {
	if len(given) == 0 || isEmptyValue(given[0]) {
		return d
	}
	return given[0]
}

// sprigRequired 对齐 sprig required：given 为空值时返回带 msg 的错误（fail-fast）。
func sprigRequired(msg string, given any) (any, error) {
	if isEmptyValue(given) {
		return nil, fmt.Errorf("%s", msg)
	}
	return given, nil
}

// sprigInt 对齐 sprig int/toInt：多标量类型收敛为 int，失败与 nil 归零。
func sprigInt(v any) int {
	switch x := v.(type) {
	case nil:
		return 0
	case int:
		return x
	case int8:
		return int(x)
	case int16:
		return int(x)
	case int32:
		return int(x)
	case int64:
		return int(x)
	case uint:
		return int(x)
	case uint8:
		return int(x)
	case uint16:
		return int(x)
	case uint32:
		return int(x)
	case uint64:
		return int(x)
	case float32:
		return int(x)
	case float64:
		return int(x)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		if err != nil {
			return 0
		}
		return n
	case bool:
		if x {
			return 1
		}
		return 0
	default:
		return 0
	}
}

// sprigUntil 对齐 sprig until：返回 [0, n) 整数序列（n<=0 返回空序列）。
func sprigUntil(n int) []int {
	if n <= 0 {
		return []int{}
	}
	out := make([]int, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, i)
	}
	return out
}

// strval 把标量值转成展示字符串（对齐 sprig strval 的常用路径）。
func strval(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []byte:
		return string(x)
	case bool:
		return strconv.FormatBool(x)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return strconv.FormatInt(reflect.ValueOf(x).Convert(reflect.TypeOf(int64(0))).Int(), 10)
	case float32, float64:
		// %v 与 strconv 'f' -1 精度一致：24.0 → "24"，不引入科学计数法。
		return strconv.FormatFloat(reflect.ValueOf(x).Convert(reflect.TypeOf(float64(0))).Float(), 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", x)
	}
}

// isEmptyValue 判定模板值为空（对齐 sprig empty 的常用语义），invalid 值视为空。
func isEmptyValue(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.String:
		return rv.Len() == 0
	case reflect.Bool:
		return !rv.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return rv.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return rv.Float() == 0
	case reflect.Slice, reflect.Map, reflect.Array:
		return rv.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return rv.IsNil()
	default:
		return false
	}
}
