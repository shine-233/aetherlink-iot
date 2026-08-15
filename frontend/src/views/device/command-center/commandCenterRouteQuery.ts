import type { LocationQueryRaw } from 'vue-router'

/**
 * Builds command-center route queries without mutating Vue Router's current
 * query object. Keeping this contract pure prevents one navigation action from
 * leaking stale filter or job identity into another action.
 */
export function buildActiveCommandJobQuery(query: LocationQueryRaw, jobId: string): LocationQueryRaw {
  return {
    ...query,
    command_job_id: jobId || undefined
  }
}

export function buildRenamedSavedFilterQuery(query: LocationQueryRaw, nextName: string): LocationQueryRaw {
  return {
    ...query,
    saved_filter_name: nextName
  }
}

export function buildClearedSavedFilterQuery(query: LocationQueryRaw): LocationQueryRaw {
  const nextQuery = { ...query }
  delete nextQuery.saved_filter_id
  delete nextQuery.saved_filter_name
  return nextQuery
}
