import { describe, expect, it } from 'vitest'
import { filterAuthRoutesByRoles } from '../modules/route/shared'

const visualizationRoute = () => ({
  name: 'visualization',
  path: '/visualization',
  meta: {},
  children: [
    {
      name: 'visualization_native-boards',
      path: '/visualization/native-boards',
      meta: { roles: ['SYS_ADMIN', 'TENANT_ADMIN'] }
    },
    {
      name: 'visualization_native-board',
      path: '/visualization/native-board',
      meta: {}
    },
    {
      name: 'visualization_native-board-editor',
      path: '/visualization/native-board-editor',
      meta: { roles: ['SYS_ADMIN', 'TENANT_ADMIN'] }
    }
  ]
})

describe('route role filtering', () => {
  it('filters protected children below a public parent without mutating the source tree', () => {
    const source = visualizationRoute()
    const filtered = filterAuthRoutesByRoles([source] as any, ['TENANT_USER'])

    expect(filtered[0].children?.map(child => child.name)).toEqual(['visualization_native-board'])
    expect(source.children.map(child => child.name)).toEqual([
      'visualization_native-boards',
      'visualization_native-board',
      'visualization_native-board-editor'
    ])
  })

  it('keeps the native management routes for tenant admins and super admins', () => {
    const tenantAdmin = filterAuthRoutesByRoles([visualizationRoute()] as any, ['TENANT_ADMIN'])
    const sysAdmin = filterAuthRoutesByRoles([visualizationRoute()] as any, ['SYS_ADMIN'])

    expect(tenantAdmin[0].children).toHaveLength(3)
    expect(sysAdmin[0].children).toHaveLength(3)
  })

  it('removes a public parent when all of its children are unauthorized', () => {
    const source = {
      name: 'protected-parent',
      path: '/protected-parent',
      meta: {},
      children: [{ name: 'admin-only', path: '/admin-only', meta: { roles: ['SYS_ADMIN'] } }]
    }

    expect(filterAuthRoutesByRoles([source] as any, ['TENANT_USER'])).toEqual([])
  })
})
