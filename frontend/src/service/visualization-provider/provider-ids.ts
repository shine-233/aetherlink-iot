/**
 * Stable identifiers shared by the visualization provider seam.
 *
 * Keep provider and built-in project IDs here so route pages, renderers and
 * adapters cannot silently drift by spelling the same contract differently.
 */
export const NATIVE_BOARD_PROVIDER_ID = 'native-board' as const
export const LEGACY_THINGSVIS_PROVIDER_ID = 'legacy-thingsvis' as const
export const NATIVE_BOARD_PROJECT_ID = 'native-boards' as const

export type BuiltInVisualizationProviderId =
  | typeof NATIVE_BOARD_PROVIDER_ID
  | typeof LEGACY_THINGSVIS_PROVIDER_ID
