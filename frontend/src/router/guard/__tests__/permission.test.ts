/**
 * 文件用途：验证 路由守卫单元测试 的关键行为和回归边界。
 * 核心逻辑：通过 Vitest、组件挂载或模块 mock 构造输入，断言状态、事件、请求参数或可见输出。
 * 关键注意事项：测试数据和 mock 必须贴近真实契约，避免只证明代码能运行而没有业务断言。
 * 重构建议：可沉淀共享 fixture 与挂载工具，并补充异常、空数据和权限边界用例。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPermissionGuard } from '../permission';

const authState = vi.hoisted(() => ({
  token: '',
  userInfo: { roles: [] as string[] },
  resetStore: vi.fn()
}));

const routeState = vi.hoisted(() => ({
  isInitAuthRoute: true,
  initAuthRoute: vi.fn(),
  getIsAuthRouteExist: vi.fn()
}));

vi.mock('@/store/modules/auth', () => ({
  useAuthStore: () => authState
}));

vi.mock('@/store/modules/route', () => ({
  useRouteStore: () => routeState
}));

vi.mock('@/utils/storage', () => ({
  localStg: {
    get: vi.fn((key: string) => (key === 'token' ? authState.token : null))
  }
}));

type TestGuard = (...args: unknown[]) => void;

function installGuard() {
  const beforeEach = vi.fn();
  createPermissionGuard({ beforeEach } as any);
  return beforeEach.mock.calls[0][0] as TestGuard;
}

function route(overrides: Record<string, any> = {}) {
  return {
    name: 'device_manage',
    path: '/device/manage',
    fullPath: '/device/manage',
    query: {},
    hash: '',
    meta: { roles: ['TENANT_ADMIN'] },
    ...overrides
  };
}

describe('permission guard contract', () => {
  beforeEach(() => {
    authState.token = '';
    authState.userInfo = { roles: [] };
    authState.resetStore.mockReset();
    routeState.isInitAuthRoute = true;
    routeState.initAuthRoute.mockReset();
    routeState.getIsAuthRouteExist.mockReset();
  });

  it('redirects anonymous protected navigation to login with the original path', async () => {
    const guard = installGuard();
    const next = vi.fn();

    await guard(route(), route({ fullPath: '/home' }), next);

    expect(next).toHaveBeenCalledWith({ name: 'login', query: { redirect: '/device/manage' } });
  });

  it('allows tenant-admin and super-admin roles through protected routes', async () => {
    const guard = installGuard();
    const next = vi.fn();
    authState.token = 'token';
    authState.userInfo.roles = ['TENANT_ADMIN'];

    await guard(route(), route({ fullPath: '/home' }), next);
    expect(next).toHaveBeenCalledWith();

    next.mockReset();
    authState.userInfo.roles = ['SYS_ADMIN'];
    await guard(route({ meta: { roles: ['TENANT_ADMIN'] } }), route({ fullPath: '/home' }), next);
    expect(next).toHaveBeenCalledWith();
  });

  it('sends logged-in users without route permission to 403', async () => {
    const guard = installGuard();
    const next = vi.fn();
    authState.token = 'token';
    authState.userInfo.roles = ['TENANT_USER'];

    await guard(route(), route({ fullPath: '/home' }), next);

    expect(next).toHaveBeenCalledWith({ name: '403' });
  });

  it('redirects a logged-in user away from login to root', async () => {
    const guard = installGuard();
    const next = vi.fn();
    authState.token = 'token';

    await guard(route({ name: 'login', path: '/login', fullPath: '/login', meta: { constant: true } }), route(), next);

    expect(next).toHaveBeenCalledWith({ name: 'root' });
  });

  it('initializes auth routes and redirects recovered not-found routes', async () => {
    const guard = installGuard();
    const next = vi.fn();
    authState.token = 'token';
    routeState.isInitAuthRoute = false;
    routeState.initAuthRoute.mockResolvedValue(true);

    await guard(
      route({ name: 'not-found', path: '/visualization/thingsvis', fullPath: '/visualization/thingsvis', redirectedFrom: { name: 'device_manage' } }),
      route(),
      next
    );

    expect(routeState.initAuthRoute).toHaveBeenCalledTimes(1);
    expect(next).toHaveBeenCalledWith({
      path: '/visualization/thingsvis',
      replace: true,
      query: {},
      hash: ''
    });
  });
});
