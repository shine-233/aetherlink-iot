import { smartDeepClone } from '@/utils/deep-clone'

export interface ConfigurationExportProcessingResult {
  processedConfig: any
  dependencies: string[]
  statistics: {
    dataSourceCount: number
    interactionCount: number
    httpConfigCount: number
  }
  dependencyMapping: Record<string, { usage: string[]; required: boolean }>
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

  const componentIdPattern = /comp_[a-zA-Z0-9_-]+/g
  const matches = value.match(componentIdPattern)
  if (matches) {
    matches.forEach((match) => {
      if (match !== currentComponentId) {
        dependencies.add(match)
      }
    })
  }

  return value
}

function processComponentId(
  componentId: string,
  currentComponentId: string,
  currentComponentPlaceholder: string,
  dependencies: Set<string>
): string {
  if (componentId === currentComponentId) {
    return currentComponentPlaceholder
  }

  dependencies.add(componentId)
  return componentId
}

function isComponentIdField(key: string): boolean {
  const componentIdFields = ['componentId', 'targetComponentId', 'sourceComponentId']
  return componentIdFields.includes(key)
}

function findComponentUsage(componentId: string, config: any): string[] {
  const usage: string[] = []

  const findUsage = (obj: any, path: string = ''): void => {
    if (typeof obj === 'string' && obj.includes(componentId)) {
      usage.push(path || 'root')
      return
    }

    if (Array.isArray(obj)) {
      obj.forEach((item, index) => {
        findUsage(item, `${path}[${index}]`)
      })
      return
    }

    if (typeof obj === 'object' && obj !== null) {
      for (const [key, value] of Object.entries(obj)) {
        const currentPath = path ? `${path}.${key}` : key
        findUsage(value, currentPath)
      }
    }
  }

  findUsage(config)
  return usage
}

function buildDependencyMapping(dependencies: string[], processedConfig: any) {
  const mapping: Record<string, { usage: string[]; required: boolean }> = {}

  dependencies.forEach((depId) => {
    mapping[depId] = {
      usage: findComponentUsage(depId, processedConfig),
      required: true
    }
  })

  return mapping
}

export function processConfigurationForExport(
  config: any,
  currentComponentId: string,
  currentComponentPlaceholder: string
): ConfigurationExportProcessingResult {
  const dependencies = new Set<string>()
  let httpConfigCount = 0
  let interactionCount = 0

  const processValue = (obj: any, path: string = ''): any => {
    if (obj === null || obj === undefined) {
      return obj
    }

    if (typeof obj === 'string') {
      return processStringValue(obj, currentComponentId, currentComponentPlaceholder, dependencies)
    }

    if (Array.isArray(obj)) {
      return obj.map((item, index) => processValue(item, `${path}[${index}]`))
    }

    if (typeof obj === 'object') {
      const processed: any = {}

      for (const [key, value] of Object.entries(obj)) {
        const currentPath = path ? `${path}.${key}` : key

        if (key === 'responses' && Array.isArray(value)) {
          interactionCount += (value as any[]).length
        }
        if (key === 'httpConfigData' || (key === 'type' && value === 'http')) {
          httpConfigCount++
        }

        if (isComponentIdField(key) && typeof value === 'string') {
          processed[key] = processComponentId(value, currentComponentId, currentComponentPlaceholder, dependencies)
        } else {
          processed[key] = processValue(value, currentPath)
        }
      }

      return processed
    }

    return obj
  }

  const processedConfig = processValue(smartDeepClone(config))
  const dependencyList = Array.from(dependencies)

  return {
    processedConfig,
    dependencies: dependencyList,
    statistics: {
      dataSourceCount: config.dataSource?.dataSources?.length || 0,
      interactionCount,
      httpConfigCount
    },
    dependencyMapping: buildDependencyMapping(dependencyList, processedConfig)
  }
}
