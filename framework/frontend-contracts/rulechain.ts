export interface RetryPolicy { maxAttempts: number; backoffMs: number; multiplier: number; maxBackoffMs: number; }
export interface RuleTrace { traceId: string; nodeId: string; status: 'STARTED' | 'SUCCEEDED' | 'FAILED' | 'DEAD_LETTER'; durationMs?: number; error?: string; timestamp: string; }
export interface RuleChainRuntime { execute(chainId: string, input: unknown, options?: { traceId?: string }): Promise<{ traceId: string; status: string }>; replay(traceId: string): Promise<{ traceId: string; status: string }>; traces(traceId: string): AsyncIterable<RuleTrace>; }
