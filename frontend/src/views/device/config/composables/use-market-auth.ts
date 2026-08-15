/**
 * 市场登录态组合函数，负责物模型市场相关页面共享的 market token 读写。
 * 核心链路：从 sessionStorage 读取 token -> 暴露登录态判断、设置、清空和读取方法 -> 供设备配置市场弹窗/抽屉复用。
 * 静态维护重点：
 * 1. 登录态目前只保存在 sessionStorage，刷新当前标签页后仍可用，但跨浏览器窗口不会自动同步。
 * 2. 这里不校验 token 有效期，也不解析用户信息，后续若市场能力继续扩展，建议补过期判断与统一鉴权错误处理。
 * 3. `marketToken` 提升为模块级 ref，所有调用方共享同一内存快照，修改存储策略时要同步检查所有设备市场页面。
 */
import { ref } from 'vue'

const marketToken = ref<string | null>(sessionStorage.getItem('market_token'))

export function useMarketAuth() {
  // 目前只根据 token 是否存在判断登录态，不区分 token 过期或格式异常。
  const isLoggedIn = () => {
    if (!marketToken.value) return false
    return true
  }

  // 写入 token 时同时更新内存态与 sessionStorage，保持同标签页内组件立即同步。
  const setToken = (token: string) => {
    marketToken.value = token
    sessionStorage.setItem('market_token', token)
  }

  // 清理登录态时同步删除浏览器会话存储，避免市场页面误以为仍然登录。
  const clearToken = () => {
    marketToken.value = null
    sessionStorage.removeItem('market_token')
  }

  // 某些页面只需要携带原始 token 调接口，因此保留简单读取方法。
  const getToken = () => marketToken.value

  return { isLoggedIn, setToken, clearToken, getToken }
}
