import type { EntityRef } from './types';
import type { WidgetDataSource } from './widget-sdk';

export interface DashboardLayout { x: number; y: number; w: number; h: number; minW?: number; minH?: number; }
export interface DashboardWidget { id: string; widgetType: string; version: string; layout: DashboardLayout; config: Record<string, unknown>; dataSources: WidgetDataSource[]; }
export interface ScadaDocument { id: string; name: string; version: number; status: 'DRAFT' | 'PUBLISHED' | 'ARCHIVED'; widgets: DashboardWidget[]; variables: Record<string, unknown>; bindings: ScadaBinding[]; }
export interface ScadaBinding { source: string; targetWidgetId: string; targetProperty: string; transform?: string; }
export interface DashboardRepository {
  list(entity?: EntityRef): Promise<ScadaDocument[]>;
  get(id: string): Promise<ScadaDocument>;
  create(input: Omit<ScadaDocument, 'id' | 'version'>): Promise<ScadaDocument>;
  save(document: ScadaDocument, expectedVersion: number): Promise<ScadaDocument>;
  publish(id: string): Promise<ScadaDocument>;
  rollback(id: string, version: number): Promise<ScadaDocument>;
}
export interface ScadaRuntime { load(document: ScadaDocument): Promise<void>; setVariable(name: string, value: unknown): void; executeCommand(widgetId: string, command: string, params?: Record<string, unknown>): Promise<void>; dispose(): void; }
