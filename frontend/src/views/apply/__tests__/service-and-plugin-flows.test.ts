/**
 * 文件用途：覆盖 service-and-plugin-flows 在 应用接入 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { describe, expect, it } from 'vitest';
import { generatedRoutes } from '@/router/elegant/routes';

function flatten(routes: any[]): any[] {
  return routes.flatMap(route => [route, ...flatten(route.children || [])]);
}

describe('apply service and plugin route contract', () => {
  const routes = flatten(generatedRoutes);

  it('registers plugin and service marketplace routes as P1 product surfaces', () => {
    const paths = routes.map(route => route.path);
    expect(paths).toEqual(expect.arrayContaining(['/apply/plugin', '/apply/service']));
  });

  it('keeps apply routes as product surfaces after removing component sample pages', () => {
    const plugin = routes.find(route => route.path === '/apply/plugin');
    const service = routes.find(route => route.path === '/apply/service');
    const retiredComponentRoute = routes.find(route => route.path === '/component/button');

    expect(plugin?.component).toContain('apply_plugin');
    expect(service?.component).toContain('apply_service');
    expect(retiredComponentRoute).toBeUndefined();
  });
});
