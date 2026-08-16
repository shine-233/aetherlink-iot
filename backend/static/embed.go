// 文件用途：将后端静态资源嵌入二进制，消除对运行时工作目录的依赖。
// 核心逻辑：通过 //go:embed 指令把 HTML 文件编译进二进制，router 直接取字节流返回。
// 关键注意事项：新增静态文件时在此追加 //go:embed 指令和导出变量即可。
package static

import _ "embed"

//go:embed metrics-viewer.html
var MetricsViewerHTML []byte

//go:embed metrics-viewer_en.html
var MetricsViewerEnHTML []byte

//go:embed echarts.min.js
var MetricsViewerEChartsJS []byte
