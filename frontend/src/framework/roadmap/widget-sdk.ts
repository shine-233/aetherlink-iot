import type { EntityRef, TelemetryPoint, TimeRange } from './types';

export interface WidgetDataSource { id: string; entity: EntityRef; keys: string[]; aggregation?: 'raw' | 'avg' | 'min' | 'max' | 'sum' | 'count'; }
export interface WidgetContext {
  widgetId: string;
  locale: string;
  theme: 'light' | 'dark';
  timeRange: TimeRange;
  dataSources: WidgetDataSource[];
  readTelemetry(source: WidgetDataSource): Promise<TelemetryPoint[]>;
  sendCommand(entity: EntityRef, command: string, params?: Record<string, unknown>): Promise<void>;
  openEntity(entity: EntityRef): void;
  reportError(error: unknown): void;
}

export interface WidgetConfigSchema { type: 'object'; properties: Record<string, { type: string; title?: string; default?: unknown; enum?: unknown[] }>; required?: string[]; }
export interface WidgetDefinition {
  type: string;
  version: string;
  title: string;
  icon?: string;
  category: 'chart' | 'status' | 'control' | 'scada' | 'map' | 'table' | 'custom';
  configSchema: WidgetConfigSchema;
  capabilities: { resizable: boolean; timeRange: boolean; command: boolean; fullscreen: boolean };
  mount(container: HTMLElement, context: WidgetContext, config: Record<string, unknown>): WidgetInstance;
}
export interface WidgetInstance { update(data: TelemetryPoint[]): void; resize(): void; destroy(): void; }
export interface WidgetRegistry { register(definition: WidgetDefinition): void; resolve(type: string, version?: string): WidgetDefinition | undefined; list(category?: WidgetDefinition['category']): WidgetDefinition[]; }
