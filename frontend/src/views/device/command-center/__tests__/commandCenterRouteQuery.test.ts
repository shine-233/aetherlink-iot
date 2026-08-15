import {
  buildActiveCommandJobQuery,
  buildClearedSavedFilterQuery,
  buildRenamedSavedFilterQuery
} from '../commandCenterRouteQuery'

describe('commandCenterRouteQuery', () => {
  it('sets and clears the active job without mutating the current query', () => {
    const currentQuery = {
      command_job_id: 'job-old',
      fleet_scope: 'device_filter',
      saved_filter_id: 'filter-1'
    }

    expect(buildActiveCommandJobQuery(currentQuery, 'job-new')).toEqual({
      command_job_id: 'job-new',
      fleet_scope: 'device_filter',
      saved_filter_id: 'filter-1'
    })
    expect(buildActiveCommandJobQuery(currentQuery, '')).toEqual({
      command_job_id: undefined,
      fleet_scope: 'device_filter',
      saved_filter_id: 'filter-1'
    })
    expect(currentQuery.command_job_id).toBe('job-old')
  })

  it('renames only the saved filter identity', () => {
    const currentQuery = {
      command_job_id: 'job-1',
      saved_filter_id: 'filter-1',
      saved_filter_name: 'Old name'
    }

    expect(buildRenamedSavedFilterQuery(currentQuery, 'New name')).toEqual({
      command_job_id: 'job-1',
      saved_filter_id: 'filter-1',
      saved_filter_name: 'New name'
    })
    expect(currentQuery.saved_filter_name).toBe('Old name')
  })

  it('clears both saved filter fields while preserving the remaining route context', () => {
    const currentQuery = {
      command_job_id: 'job-1',
      fleet_scope: 'device_filter',
      saved_filter_id: 'filter-1',
      saved_filter_name: 'Online pumps'
    }

    expect(buildClearedSavedFilterQuery(currentQuery)).toEqual({
      command_job_id: 'job-1',
      fleet_scope: 'device_filter'
    })
    expect(currentQuery).toHaveProperty('saved_filter_id', 'filter-1')
    expect(currentQuery).toHaveProperty('saved_filter_name', 'Online pumps')
  })
})
