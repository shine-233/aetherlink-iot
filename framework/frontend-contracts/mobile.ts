import type { EntityRef, TelemetryPoint } from './types';

export interface MobileCapabilityMatrix { telemetry: boolean; commands: boolean; alarms: boolean; shadow: boolean; ota: boolean; dashboards: boolean; push: boolean; offlineCache: boolean; }
export interface MobileDeviceSummary { entity: EntityRef; name: string; online: boolean; lastSeen?: string; latest?: TelemetryPoint[]; alarmCount: number; }
export interface MobileApi {
  capabilities(): Promise<MobileCapabilityMatrix>;
  devices(query?: { search?: string; page?: number; pageSize?: number }): Promise<{ items: MobileDeviceSummary[]; total: number }>;
  acknowledgeAlarm(alarmId: string): Promise<void>;
  sendCommand(entity: EntityRef, command: string, params?: Record<string, unknown>): Promise<void>;
  getShadow(entity: EntityRef): Promise<Record<string, unknown>>;
  updateShadow(entity: EntityRef, patch: Record<string, unknown>): Promise<void>;
  subscribePush(token: string, platform: 'ios' | 'android'): Promise<void>;
}
