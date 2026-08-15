import { describe, expect, it } from 'vitest';
import type { GeneratedRoute } from '@elegant-router/types';
import { generatedRoutes } from '../../elegant/routes';
import { layouts, views } from '../../elegant/imports';

/**
 * Guards the generated route inventory and the P0/P1 business route list.
 * Built-in exception and legal placeholder pages are tracked separately from
 * business-coverage routes.
 */
// Current elegant-router inventory: 61 unique paths / 52 leaf paths. The
// former exception-page tree was removed in favor of the explicit 403/404/500
// status routes below; keep this baseline tied to the generated source files.
const GENERATED_ROUTE_MINIMUM = 61;
const LEAF_ROUTE_MINIMUM = 52;

const parentRoutes = new Set([
  '/alarm',
  '/apply',
  '/automation',
  '/dashboard',
  '/device',
  '/manage',
  '/management',
  '/product',
  '/system-management-user',
  '/visualization'
]);

const expectedP0P1Routes = [
  '/device/manage',
  '/device/command-center',
  '/device/details',
  '/device/thingsmodel',
  '/device/share',
  '/device/shared-with-me',
  '/alarm/rdi-overview',
  '/alarm/warning-message',
  '/automation/scene-manage',
  '/automation/scene-linkage',
  '/management/user',
  '/management/role',
  '/management/api',
  '/management/auth',
  '/apply/plugin',
  '/apply/service',
  '/visualization/native-board',
  '/visualization/native-board-editor',
  '/visualization/native-boards',
  '/visualization/thingsvis',
  '/visualization/thingsvis-dashboards',
  '/visualization/thingsvis-editor',
  '/visualization/thingsvis-menu-dashboard',
  '/visualization/thingsvis-preview',
  '/device-details-app'
];

function flattenRoutes(routes: GeneratedRoute[]): GeneratedRoute[] {
  return routes.flatMap(route => [route, ...flattenRoutes((route.children || []) as GeneratedRoute[])]);
}

function assertResolvableComponent(route: GeneratedRoute) {
  const component = route.component;
  expect(typeof component, `route ${route.path} must declare a component token`).toBe('string');
  expect(component.length, `route ${route.path} must not have an empty component token`).toBeGreaterThan(0);

  if (component.includes('$')) {
    const [layoutToken, viewToken, ...unexpectedTokens] = component.split('$');
    expect(unexpectedTokens, `route ${route.path} has an invalid composite component token`).toHaveLength(0);
    expect(layoutToken.startsWith('layout.'), `route ${route.path} composite token must start with layout.`).toBe(true);
    expect(viewToken.startsWith('view.'), `route ${route.path} composite token must include view.`).toBe(true);
    expect(Object.prototype.hasOwnProperty.call(layouts, layoutToken.slice('layout.'.length)), `route ${route.path} layout is missing from imports`).toBe(true);
    expect(Object.prototype.hasOwnProperty.call(views, viewToken.slice('view.'.length)), `route ${route.path} view is missing from imports`).toBe(true);
    return;
  }

  if (component.startsWith('layout.')) {
    expect(Object.prototype.hasOwnProperty.call(layouts, component.slice('layout.'.length)), `route ${route.path} layout is missing from imports`).toBe(true);
    return;
  }

  expect(component.startsWith('view.'), `route ${route.path} component must be a layout or view token`).toBe(true);
  expect(Object.prototype.hasOwnProperty.call(views, component.slice('view.'.length)), `route ${route.path} view is missing from imports`).toBe(true);
}

describe('route coverage contract', () => {
  const routes = flattenRoutes(generatedRoutes);
  const paths = routes.map(route => route.path);
  const leafPaths = paths.filter(path => !parentRoutes.has(path));

  it('keeps the generated route inventory large enough to prevent stale page catalogs', () => {
    expect(new Set(paths).size).toBeGreaterThanOrEqual(GENERATED_ROUTE_MINIMUM);
    expect(new Set(leafPaths).size).toBeGreaterThanOrEqual(LEAF_ROUTE_MINIMUM);
  });

  it('keeps every route path unique and every route component resolvable', () => {
    expect(new Set(paths).size).toBe(paths.length);
    routes.forEach(route => {
      expect(route.path, 'generated routes must have a non-empty path').toMatch(/^\//);
      assertResolvableComponent(route);
    });
  });

  it('contains every P0/P1 business route that needs page and E2E traceability', () => {
    expect(paths).toEqual(expect.arrayContaining(expectedP0P1Routes));
  });

  it('protects native board management routes while leaving the viewer shareable', () => {
    const listRoute = routes.find(route => route.path === '/visualization/native-boards');
    const editorRoute = routes.find(route => route.path === '/visualization/native-board-editor');
    const viewerRoute = routes.find(route => route.path === '/visualization/native-board');

    expect(listRoute?.meta?.roles).toEqual(['SYS_ADMIN', 'TENANT_ADMIN']);
    expect(editorRoute?.meta?.roles).toEqual(['SYS_ADMIN', 'TENANT_ADMIN']);
    expect(viewerRoute?.meta?.roles || []).toEqual([]);
  });

  it('keeps exception status pages separate from business coverage', () => {
    expect(paths).toEqual(expect.arrayContaining(['/403', '/404', '/500']));
    expect(paths).not.toEqual(expect.arrayContaining(['/exception', '/exception/403', '/exception/404', '/exception/500']));
  });
});
