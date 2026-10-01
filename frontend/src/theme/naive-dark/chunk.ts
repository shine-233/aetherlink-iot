/**
 * 文件用途：naive-ui darkTheme 的独立 chunk 入口，仅被 ./index.ts 动态 import。
 * 关键注意事项：不要在别处静态引用本文件，否则 ~37KB 暗色样式变量会回到首屏 entry chunk。
 */
export { darkTheme } from 'naive-ui'
