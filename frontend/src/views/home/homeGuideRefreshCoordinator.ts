type Refresh = () => Promise<unknown> | void
type Schedule = (task: () => void, fallbackDelay?: number) => void

type HomeGuideRefreshCoordinatorOptions = {
  schedule: Schedule
  refreshTenantSetup: Refresh
  refreshDeploymentHealth: Refresh
  refreshFirstDeviceWorkbench: Refresh
  refreshAutomation: Refresh
  shouldRefreshAutomation: () => boolean
}

// Coordinates only the existing fire-and-forget ordering. Each leaf keeps its
// own loading state and in-flight deduplication in the home page.
export function createHomeGuideRefreshCoordinator(options: HomeGuideRefreshCoordinatorOptions) {
  const scheduleGuideRefresh = (includeWorkbench: boolean) => {
    void options.refreshTenantSetup()
    options.schedule(() => {
      void options.refreshDeploymentHealth()
    }, 100)

    if (includeWorkbench) {
      options.schedule(() => {
        void options.refreshFirstDeviceWorkbench()
      }, 150)
    }

    if (options.shouldRefreshAutomation()) {
      options.schedule(() => {
        void options.refreshAutomation()
      }, 250)
    }
  }

  return {
    refreshFromUser: () => scheduleGuideRefresh(true),
    refreshOnInitialLoad: () => scheduleGuideRefresh(false)
  }
}
