/**
 * 文件用途：验证 路由守卫单元测试 的关键行为和回归边界。
 * 核心逻辑：通过 Vitest、组件挂载或模块 mock 构造输入，断言状态、事件、请求参数或可见输出。
 * 关键注意事项：测试数据和 mock 必须贴近真实契约，避免只证明代码能运行而没有业务断言。
 * 重构建议：可沉淀共享 fixture 与挂载工具，并补充异常、空数据和权限边界用例。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createMobileLayoutGuard } from '../mobile';

const appState = vi.hoisted(() => ({
  isMobile: false
}));

vi.mock('@/store/modules/app', () => ({
  useAppStore: () => appState
}));

type TestGuard = (...args: unknown[]) => void;

function installGuard(routes: any[] = []) {
  const beforeEach = vi.fn();
  createMobileLayoutGuard({
    beforeEach,
    getRoutes: () => routes
  } as any);
  return beforeEach.mock.calls[0][0] as TestGuard;
}

function route(overrides: Record<string, any> = {}) {
  return {
    name: 'device_manage',
    meta: {},
    ...overrides
  };
}

describe('mobile layout guard contract', () => {
  beforeEach(() => {
    appState.isMobile = false;
  });

  it('always resolves navigation on desktop routes', () => {
    const guard = installGuard();
    const next = vi.fn();

    guard(route(), route({ name: 'login' }), next);

    expect(next).toHaveBeenCalledTimes(1);
    expect(next).toHaveBeenCalledWith();
  });

  it('checks registered base-layout routes on mobile before resolving navigation', () => {
    appState.isMobile = true;
    const guard = installGuard([
      {
        name: 'device_manage',
        components: { default: { name: 'BaseLayout' } }
      }
    ]);
    const next = vi.fn();

    guard(route(), route({ name: 'login' }), next);

    expect(next).toHaveBeenCalledTimes(1);
  });

  it('does not treat constant, error, or explicitly disabled routes as mobile-layout candidates', () => {
    appState.isMobile = true;
    const getRoutes = vi.fn(() => [
      {
        name: 'login',
        components: { default: { name: 'BaseLayout' } }
      }
    ]);
    const beforeEach = vi.fn();
    createMobileLayoutGuard({ beforeEach, getRoutes } as any);
    const guard = beforeEach.mock.calls[0][0] as TestGuard;
    const next = vi.fn();

    guard(route({ name: 'login', meta: { constant: true } }), route(), next);
    guard(route({ name: '403', meta: {} }), route(), next);
    guard(route({ name: 'device_manage', meta: { disableMobileLayout: true } }), route(), next);

    expect(next).toHaveBeenCalledTimes(3);
    expect(getRoutes).not.toHaveBeenCalled();
  });
});
