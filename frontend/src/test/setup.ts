/**
 * 文件用途：提供前端 Vitest 测试环境的基础配置和 mock。
 * 核心逻辑：初始化 Vue Test Utils、国际化、全局组件桩和常用浏览器/Naive UI mock。
 * 关键注意事项：全局 mock 会影响全部单测，新增默认行为前需要确认不会隐藏真实问题。
 * 重构建议：可把全局 mock、业务 fixture 和测试工具分层，减少不同测试之间的隐式耦合。
 */
import { config } from '@vue/test-utils';
import { afterEach, vi } from 'vitest';
import { testI18n } from './i18n';
import { dialogMock, messageMock } from './hoisted-mocks';
import { ensureLocaleReady } from '@/locales';

// 生产在挂载前 await ensureLocaleReady() 装载启动语言目录；语言包改为按需
// 懒加载后，真实 $t 实例默认 messages 为空、会退化成"返回 key"。测试环境
// 同样先预装载 en-us 目录，保证直接导入真实 $t 的业务模块断言到译文。
await ensureLocaleReady();

const mockedCurrentRoute = vi.hoisted(() => ({
  value: {
    name: 'root',
    path: '/',
    fullPath: '/',
    meta: { constant: true }
  }
}));

vi.mock('@/router/routes', () => ({
  ROOT_ROUTE: { name: 'root', path: '/', meta: { constant: true } },
  createRoutes: () => ({ constantVueRoutes: [], authRoutes: [] }),
  getAuthVueRoutes: (routes: unknown[]) => routes
}));

vi.mock('@/router', () => ({
  router: {
    currentRoute: mockedCurrentRoute,
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    getRoutes: vi.fn(() => []),
    addRoute: vi.fn(() => () => {}),
    removeRoute: vi.fn(),
    isReady: vi.fn(() => Promise.resolve())
  },
  setupRouter: vi.fn(() => Promise.resolve())
}));

// Mock virtual:svg-icons-register (Vite plugin virtual module)
vi.mock('virtual:svg-icons-register', () => ({}));

// Mock SvgIcon component to avoid virtual:svg-icons-register import issues
vi.mock('@/components/custom/svg-icon.vue', () => ({
  default: { template: '<svg><slot /></svg>' }
}));

// 全局 mock i18n
config.global.mocks = {
  $t: (key: string) => key
};

config.global.plugins = [testI18n];

config.global.renderStubDefaultSlot = true;

// 全局 stub 常用组件
config.global.stubs = {
  'router-link': true
};

// Mock window.$message (Naive UI discrete API)
(globalThis as any).$message = {
  success: messageMock.success,
  error: messageMock.error,
  warning: messageMock.warning,
  info: messageMock.info,
  loading: messageMock.loading
};

(globalThis as any).$dialog = dialogMock;

// Mock localStorage
const localStorageMock = (() => {
  let store: Record<string, string> = {};
  return {
    getItem: (key: string) => store[key] || null,
    setItem: (key: string, value: string) => {
      store[key] = value;
    },
    removeItem: (key: string) => {
      delete store[key];
    },
    clear: () => {
      store = {};
    }
  };
})();

Object.defineProperty(globalThis, 'localStorage', { value: localStorageMock, writable: true, configurable: true });

class ResizeObserverMock {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
}

Object.defineProperty(globalThis, 'ResizeObserver', { value: ResizeObserverMock, writable: true, configurable: true });

Object.defineProperty(window, 'matchMedia', {
  writable: true,
  configurable: true,
  value: vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn()
  }))
});

// Components under test may start a lazy import while Vue is flushing an
// update. Wait for that import chain before Vitest tears down the file's
// environment; otherwise Vitest reports an unhandled EnvironmentTeardownError
// even when every assertion in the file passed.
afterEach(async () => {
  await vi.dynamicImportSettled()
})
