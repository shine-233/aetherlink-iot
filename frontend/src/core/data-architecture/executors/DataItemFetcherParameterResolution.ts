/**
 * Runtime HTTP parameter normalization for DataItemFetcher.
 *
 * This file keeps persisted-config compatibility rules, damaged binding-path
 * recovery, and data-type conversion separate from request orchestration.
 */
import type { HttpParameter } from '@/core/data-architecture/types/http-config'
import { convertValue } from '@/core/data-architecture/types/http-config'
import {
  isDamagedComponentBindingPath,
  isValidComponentBindingPath,
  recoverComponentBindingPathFromVariableName,
  resolveRecoverableComponentBindingPath
} from '@/core/data-architecture/utils/binding-path-recovery'

import type { HttpDataItemConfig } from './DataItemFetcher'
import { collectHttpParameters } from './DataItemFetcherRequestPlan'

interface ComponentBoundParameterValue {
  invalidBinding: boolean
  value: unknown
}

export type ComponentPropertyReader = (bindingPath: string) => Promise<unknown>

export function logHttpParametersLifecycle(config: HttpDataItemConfig, stage: string): void {
  collectHttpParameters(config).forEach(({ source, param, index }) => {
    if (param.value && typeof param.value === 'string') {
      const isSuspiciousPath = !param.value.includes('.') && param.value.length < 10 && param.variableName
      if (isSuspiciousPath) {
        console.error(`[${source}[${index}]] Suspicious binding path:`, {
          key: param.key,
          value: param.value,
          variableName: param.variableName,
          stage
        })
      }
    }
  })
}

export function validateParameterBindingPaths(config: HttpDataItemConfig): void {
  collectHttpParameters(config).forEach(({ param, index }) => {
    if (param.selectedTemplate === 'component-property-binding' || param.valueMode === 'component') {
      const recoveredPath = resolveRecoverableComponentBindingPath(param.value, param.variableName)
      if (!recoveredPath.isValid) {
        console.error('[DataItemFetcher] Invalid parameter binding path:', {
          index,
          key: param.key,
          value: recoveredPath.bindingPath,
          param
        })
      }
    }
  })
}

export async function resolveHttpParameterValue(
  param: HttpParameter,
  readComponentProperty: ComponentPropertyReader
): Promise<unknown> {
  const runtimeParam = normalizeRuntimeParameter(param)
  let resolvedValue: unknown = runtimeParam.value

  if (shouldResolveComponentBinding(runtimeParam)) {
    const componentBoundValue = await resolveComponentBoundParameterValue(runtimeParam, readComponentProperty)
    if (componentBoundValue.invalidBinding) {
      return componentBoundValue.value
    }
    resolvedValue = componentBoundValue.value
  }

  if (isEmptyParameterValue(resolvedValue)) {
    resolvedValue = resolveEmptyParameterFallback(runtimeParam)
    if (resolvedValue === null) {
      return null
    }
  }

  return convertValue(resolvedValue, runtimeParam.dataType)
}

function normalizeRuntimeParameter(param: HttpParameter): HttpParameter {
  if (detectRuntimeIsDynamic(param) && !param.isDynamic) {
    return { ...param, isDynamic: true }
  }

  return param
}

function detectRuntimeIsDynamic(param: HttpParameter): boolean {
  return Boolean(
    param.valueMode === 'component' ||
      param.selectedTemplate === 'component-property-binding' ||
      (typeof param.value === 'string' &&
        param.value.includes('.') &&
        param.value.split('.').length >= 3 &&
        param.value.length > 10 &&
        !/^\d{1,4}$/.test(param.value)) ||
      (param.variableName &&
        !param.variableName.startsWith('var_') &&
        param.variableName.includes('_') &&
        param.variableName.length > 5)
  )
}

function shouldResolveComponentBinding(param: HttpParameter): boolean {
  return Boolean(
    param.isDynamic || param.selectedTemplate === 'component-property-binding' || param.valueMode === 'component'
  )
}

function normalizeComponentBindingPath(param: HttpParameter): string {
  let bindingPath = typeof param.value === 'string' ? param.value : String(param.value ?? '')

  if (isDamagedComponentBindingPath(bindingPath, param.variableName)) {
    console.error('[DataItemFetcher] Damaged binding path detected:', {
      key: param.key,
      bindingPath,
      variableName: param.variableName
    })
    bindingPath = recoverComponentBindingPathFromVariableName(param.variableName) || bindingPath
  }

  return bindingPath
}

async function resolveComponentBoundParameterValue(
  param: HttpParameter,
  readComponentProperty: ComponentPropertyReader
): Promise<ComponentBoundParameterValue> {
  const bindingPath = normalizeComponentBindingPath(param)

  if (!isValidComponentBindingPath(bindingPath)) {
    return {
      invalidBinding: true,
      value: param.defaultValue ?? null
    }
  }

  const actualValue = await readComponentProperty(bindingPath)
  return {
    invalidBinding: false,
    value: isEmptyParameterValue(actualValue) ? undefined : actualValue
  }
}

function resolveEmptyParameterFallback(param: HttpParameter): unknown {
  if (param.defaultValue !== undefined && param.defaultValue !== null) {
    return param.defaultValue
  }

  return null
}

function isEmptyParameterValue(value: unknown): boolean {
  return value === null || value === undefined || value === '' || (typeof value === 'string' && value.trim() === '')
}
