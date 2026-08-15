import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useRdiCommands } from '../useRdiCommands'
import type { RDIConfig } from '@/service/api/rdi'

const { mockSendRdiCommand, mockGetOtaPackageList, mockRdiLatestFirmware, mockMessage } = vi.hoisted(() => ({
  mockSendRdiCommand: vi.fn(),
  mockGetOtaPackageList: vi.fn(),
  mockRdiLatestFirmware: vi.fn(),
  mockMessage: {
    success: vi.fn(),
    error: vi.fn(),
    warning: vi.fn(),
    info: vi.fn()
  }
}))

vi.mock('@/service/api', () => ({
  sendRdiCommand: (...args: any[]) => mockSendRdiCommand(...args),
  rdiLatestFirmware: (...args: any[]) => mockRdiLatestFirmware(...args)
}))

vi.mock('@/service/product/update-package', () => ({
  getOtaPackageList: (...args: any[]) => mockGetOtaPackageList(...args)
}))

vi.mock('@/utils/common/discrete', () => ({
  message: mockMessage
}))

vi.mock('@/utils/common/tool', () => ({
  getBaseServerUrl: () => 'http://localhost:8080/api/v1'
}))

function createConfig(overrides: Partial<RDIConfig> = {}): RDIConfig {
  return {
    data_collection_interval: 60,
    alarm_sensor_1_enabled: true,
    alarm_sensor_2_enabled: true,
    sensor_1_upper: 80,
    sensor_1_lower: -10,
    sensor_2_upper: 80,
    sensor_2_lower: -10,
    sensor_1_duration: 30,
    sensor_2_duration: 30,
    switch_1_alarm_mode: 'disabled',
    switch_2_alarm_mode: 'disabled',
    switch_1_alarm_duration: 30,
    switch_2_alarm_duration: 30,
    dry_contact_alarm_level: 'high',
    dry_contact_normal_level: 'low',
    dry_contact_alarm_delay: 0,
    dry_contact_normal_delay: 0,
    notification_enabled: false,
    notification_temperature_alarm: true,
    notification_switch_alarm: true,
    notification_warranty_alarm: true,
    sensor_alarm_emails: '',
    switch_alarm_emails: '',
    warranty_alarm_emails: '',
    sensor_1_alarm_emails: '',
    sensor_2_alarm_emails: '',
    switch_1_alarm_emails: '',
    switch_2_alarm_emails: '',
    field_setting: {},
    ...overrides
  } as RDIConfig
}

function createComposable(config: RDIConfig = createConfig()) {
  return useRdiCommands(() => 'dev-1', config, key => String(key))
}

describe('useRdiCommands command tracking summary', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('stores returned tracking details and surfaces the summary after OTA send', async () => {
    mockSendRdiCommand.mockResolvedValue({
      error: null,
      data: {
        message_id: 'mid-1',
        identifier: 'ota_upgrade',
        tracking_status: 'queued',
        log_recorded: true,
        command_tracking: {
          message_id: 'mid-1',
          identifier: 'ota_upgrade',
          status: 'queued',
          device_id: 'dev-1',
          operation_type: 'ota_upgrade',
          log_recorded: true
        }
      }
    })

    const composable = createComposable()
    composable.otaCommand.firmware_url = 'http://example.com/fw.bin'
    composable.otaCommand.version = '1.2.3'
    composable.otaCommand.size = 1024
    composable.otaCommand.md5 = 'abc123'

    await composable.sendOtaUpgrade()

    expect(composable.lastCommandTracking.value?.message_id).toBe('mid-1')
    expect(composable.commandTrackingSummary.value).toBe('ota_upgrade message_id=mid-1 status=queued (log recorded)')
    expect(mockMessage.success).toHaveBeenCalledWith('ota_upgrade message_id=mid-1 status=queued (log recorded)')
  })
})
