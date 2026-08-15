import { describe, expect, it } from 'vitest'

import {
  buildSearchDevicesPagedParams,
  buildSearchDevicesPagedResultPayload,
  normalizeSearchDevicesPagedPayload,
  resolveSearchDevicesGroupContext,
  type SearchDevicesPagedRequest
} from './searchDevicesPagedBridge'

describe('searchDevicesPagedBridge', () => {
  it('normalizes request payload with defaults', () => {
    expect(normalizeSearchDevicesPagedPayload({ keyword: 'pump' })).toEqual({
      keyword: 'pump',
      groupId: '__all__',
      deviceConfigId: '',
      isOnline: '',
      warnStatus: '',
      deviceType: '',
      serviceIdentifier: '',
      label: '',
      page: 1,
      pageSize: 10,
      reqId: ''
    })
  })

  it('builds search params without all-group noise', () => {
    const request: SearchDevicesPagedRequest = {
      keyword: 'pump',
      groupId: '__all__',
      deviceConfigId: '',
      isOnline: '',
      warnStatus: '',
      deviceType: '',
      serviceIdentifier: '',
      label: '',
      page: 2,
      pageSize: 25,
      reqId: 'req-1'
    }

    expect(buildSearchDevicesPagedParams(request)).toEqual({
      page: 2,
      page_size: 25,
      search: 'pump'
    })
  })

  it('builds search params for explicit filters', () => {
    const request: SearchDevicesPagedRequest = {
      keyword: 'edge',
      groupId: 'group-1',
      deviceConfigId: 'cfg-1',
      isOnline: '1',
      warnStatus: 'warning',
      deviceType: 'sensor',
      serviceIdentifier: 'modbus',
      label: 'line-a',
      page: 3,
      pageSize: 50,
      reqId: 'req-2'
    }

    expect(buildSearchDevicesPagedParams(request)).toEqual({
      page: 3,
      page_size: 50,
      search: 'edge',
      group_id: 'group-1',
      device_config_id: 'cfg-1',
      is_online: 1,
      warn_status: 'warning',
      device_type: 'sensor',
      service_identifier: 'modbus',
      label: 'line-a'
    })
  })

  it('resolves group context with fallback name when group is missing', () => {
    const request = normalizeSearchDevicesPagedPayload({ groupId: 'group-404' })

    expect(resolveSearchDevicesGroupContext(request, [])).toEqual({
      groupId: 'group-404',
      fallbackGroupName: 'group-404'
    })
  })

  it('builds stable bridge response payload', () => {
    expect(
      buildSearchDevicesPagedResultPayload({ reqId: 'req-3', page: 4, pageSize: 20 }, [{ deviceId: 'd-1' }], undefined)
    ).toEqual({
      reqId: 'req-3',
      devices: [{ deviceId: 'd-1' }],
      total: 0,
      page: 4,
      pageSize: 20
    })
  })
})
