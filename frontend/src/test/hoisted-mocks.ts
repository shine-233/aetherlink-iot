/**
 * 文件用途：提供前端 Vitest 测试环境的基础配置和 mock。
 * 核心逻辑：初始化 Vue Test Utils、国际化、全局组件桩和常用浏览器/Naive UI mock。
 * 关键注意事项：全局 mock 会影响全部单测，新增默认行为前需要确认不会隐藏真实问题。
 * 重构建议：可把全局 mock、业务 fixture 和测试工具分层，减少不同测试之间的隐式耦合。
 */
import { vi } from 'vitest';

export const messageMock = {
  success: vi.fn(),
  error: vi.fn(),
  warning: vi.fn(),
  info: vi.fn(),
  loading: vi.fn()
};

export const dialogMock = {
  success: vi.fn(),
  error: vi.fn(),
  warning: vi.fn(),
  info: vi.fn(),
  create: vi.fn()
};

export function resetHoistedMocks() {
  Object.values(messageMock).forEach(fn => fn.mockClear());
  Object.values(dialogMock).forEach(fn => fn.mockClear());
}
