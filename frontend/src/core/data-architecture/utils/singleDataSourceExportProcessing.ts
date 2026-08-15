type RelatedSingleDataSourceConfig = {
  interactions: any[]
  httpBindings: any[]
}

export function processSingleDataSourceForExport(
  sourceConfig: any,
  currentComponentId: string,
  currentComponentPlaceholder: string,
  dependencies: Set<string>
): any {
  const processValue = (obj: any): any => {
    if (obj === null || obj === undefined) {
      return obj
    }

    if (typeof obj === 'string') {
      return processStringValue(obj, currentComponentId, currentComponentPlaceholder, dependencies)
    }

    if (Array.isArray(obj)) {
      return obj.map((item) => processValue(item))
    }

    if (typeof obj === 'object') {
      const result: any = {}
      for (const [key, value] of Object.entries(obj)) {
        result[key] = processValue(value)
      }
      return result
    }

    return obj
  }

  return processValue(sourceConfig)
}

function processStringValue(
  value: string,
  currentComponentId: string,
  currentComponentPlaceholder: string,
  dependencies: Set<string>
): string {
  if (value.includes(currentComponentId)) {
    return value.replace(new RegExp(currentComponentId, 'g'), currentComponentPlaceholder)
  }

  const componentIdPattern = /^[a-zA-Z][a-zA-Z0-9_-]*_\d+$/
  if (componentIdPattern.test(value) && value !== currentComponentId) {
    dependencies.add(value)
  }

  return value
}

export function collectRelatedSingleDataSourceConfig(
  componentId: string,
  sourceId: string,
  configurationManager: any,
  currentComponentPlaceholder: string,
  dependencies: Set<string>
): RelatedSingleDataSourceConfig {
  const relatedConfig: RelatedSingleDataSourceConfig = {
    interactions: [],
    httpBindings: []
  }

  try {
    const interactionConfig = configurationManager.getConfiguration(componentId, 'interaction')
    if (interactionConfig) {
      const relatedInteractions = findRelatedInteractions(interactionConfig, sourceId)
      relatedConfig.interactions = relatedInteractions.map((interaction) =>
        processSingleDataSourceForExport(interaction, componentId, currentComponentPlaceholder, dependencies)
      )
    }

    const componentConfig = configurationManager.getConfiguration(componentId, 'component')
    if (componentConfig?.httpBindings) {
      const relatedHttpBindings = componentConfig.httpBindings.filter((binding: any) => binding.sourceId === sourceId)
      relatedConfig.httpBindings = relatedHttpBindings.map((binding: any) =>
        processSingleDataSourceForExport(binding, componentId, currentComponentPlaceholder, dependencies)
      )
    }
  } catch (error) {
    console.error('[SingleDataSourceExporter] failed to collect related configuration:', error)
  }

  return relatedConfig
}

function findRelatedInteractions(interactionConfig: any, sourceId: string): any[] {
  const relatedInteractions: any[] = []

  if (!interactionConfig || typeof interactionConfig !== 'object') {
    return relatedInteractions
  }

  const objectDirectlyReferencesSource = (obj: Record<string, any>) =>
    obj.sourceId === sourceId || obj.dataSourceId === sourceId || obj.dataSource?.sourceId === sourceId

  // Recursively collect the smallest interaction objects that reference sourceId.
  const searchInteractions = (obj: any): boolean => {
    if (Array.isArray(obj)) {
      return obj.some((item) => searchInteractions(item))
    }

    if (typeof obj !== 'object' || obj === null) {
      return false
    }

    if (objectDirectlyReferencesSource(obj)) {
      relatedInteractions.push(obj)
      return true
    }

    const childMatched = Object.values(obj).some((value) => searchInteractions(value))
    if (!childMatched && JSON.stringify(obj).includes(sourceId)) {
      relatedInteractions.push(obj)
      return true
    }

    return childMatched
  }

  searchInteractions(interactionConfig)
  return relatedInteractions
}
