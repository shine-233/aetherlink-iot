/** Shared DTOs used by the next-phase frontend framework. */
export type EntityType = 'DEVICE' | 'ASSET' | 'CUSTOMER' | 'TENANT' | 'EDGE';
export type DataValue = string | number | boolean | null | Record<string, unknown>;

export interface EntityRef { type: EntityType; id: string; }
export interface TimeRange { from: string; to: string; timezone?: string; }
export interface TelemetryPoint { key: string; ts: string; value: DataValue; quality?: 'good' | 'uncertain' | 'bad'; }
export interface ApiError { code: string; message: string; retryable?: boolean; details?: Record<string, unknown>; }

export interface EntityRelation {
  id: string;
  tenantId: string;
  from: EntityRef;
  to: EntityRef;
  relationType: string;
  direction: 'FORWARD' | 'REVERSE';
  metadata?: Record<string, DataValue>;
}
