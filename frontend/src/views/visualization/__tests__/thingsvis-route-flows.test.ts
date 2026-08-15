/**
 * 文件用途: 覆盖Thingsvis Route Flows在可视化场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { describe, expect, it } from 'vitest';
import { generatedRoutes } from '@/router/elegant/routes';

const visualizationRoutes = [
  '/visualization/thingsvis',
  '/visualization/thingsvis-dashboards',
  '/visualization/thingsvis-editor',
  '/visualization/thingsvis-menu-dashboard',
  '/visualization/thingsvis-preview'
];

function flatten(routes: any[]): any[] {
  return routes.flatMap(route => [route, ...flatten(route.children || [])]);
}

describe('ThingsVis route flow contract', () => {
  const routes = flatten(generatedRoutes);

  it('registers all ThingsVis user-facing routes as P1 coverage targets', () => {
    const paths = routes.map(route => route.path);
    expect(paths).toEqual(expect.arrayContaining(visualizationRoutes));
  });

  it('keeps editor and preview routes distinct so E2E can assert edit versus view behavior', () => {
    const editor = routes.find(route => route.path === '/visualization/thingsvis-editor');
    const preview = routes.find(route => route.path === '/visualization/thingsvis-preview');

    expect(editor?.component).toContain('thingsvis-editor');
    expect(preview?.component).toContain('thingsvis-preview');
    expect(preview?.meta?.constant).toBe(true);
  });
});
