import type { ComponentDataStorage, DataStorageItem } from './DataWarehouse'

interface ComponentCacheStoreOptions {
  getDefaultCacheExpiry: () => number
  calculateDataSize: (data: any) => number
  isExpired: (item: DataStorageItem) => boolean
}

interface ValidComponentDataCollection {
  data: Record<string, any>
  hasData: boolean
}

export class ComponentCacheStore {
  constructor(
    private readonly componentStorage: Map<string, ComponentDataStorage>,
    private readonly options: ComponentCacheStoreOptions
  ) {}

  getOrCreateComponentStorage(componentId: string, now: number): ComponentDataStorage {
    let componentStorage = this.componentStorage.get(componentId)
    if (!componentStorage) {
      componentStorage = {
        componentId,
        dataSources: new Map(),
        createdAt: now,
        updatedAt: now
      }
      this.componentStorage.set(componentId, componentStorage)
    }
    return componentStorage
  }

  createStorageItem(
    componentId: string,
    sourceId: string,
    data: any,
    sourceType: string,
    dataSize: number,
    dataVersion: string,
    executionId: string,
    customExpiry: number | undefined,
    now: number
  ): DataStorageItem {
    return {
      data,
      timestamp: now,
      expiresAt: customExpiry ? now + customExpiry : now + this.options.getDefaultCacheExpiry(),
      source: {
        sourceId,
        sourceType,
        componentId
      },
      size: dataSize,
      accessCount: 0,
      lastAccessed: now,
      dataVersion,
      executionId
    }
  }

  writeDataSourceItem(
    componentStorage: ComponentDataStorage,
    sourceId: string,
    storageItem: DataStorageItem,
    now: number
  ): void {
    componentStorage.dataSources.set(sourceId, storageItem)
    componentStorage.updatedAt = now
    this.invalidateMergedData(componentStorage)
  }

  getValidMergedStorageItem(componentStorage: ComponentDataStorage): DataStorageItem | null {
    const mergedItem = componentStorage.mergedData
    if (!mergedItem || this.options.isExpired(mergedItem)) {
      return null
    }

    this.markStorageItemAccessed(mergedItem)
    return mergedItem
  }

  collectValidComponentData(componentStorage: ComponentDataStorage): ValidComponentDataCollection {
    const collection: ValidComponentDataCollection = {
      data: {},
      hasData: false
    }

    for (const [sourceId, item] of componentStorage.dataSources) {
      this.collectDataSourceItem(componentStorage, sourceId, item, collection)
    }

    return collection
  }

  unwrapCompleteComponentData(componentData: Record<string, any>): Record<string, any> {
    if (!this.hasCompleteDataSource(componentData)) {
      return componentData
    }

    return this.unwrapCompleteDataSource(componentData.complete)
  }

  cacheMergedComponentData(componentStorage: ComponentDataStorage, finalData: Record<string, any>): void {
    componentStorage.mergedData = this.createMergedStorageItem(componentStorage.componentId, finalData)
  }

  getValidDataSourceItem(componentStorage: ComponentDataStorage, sourceId: string): DataStorageItem | null {
    const item = componentStorage.dataSources.get(sourceId)
    if (!item) {
      return null
    }

    if (this.options.isExpired(item)) {
      componentStorage.dataSources.delete(sourceId)
      return null
    }

    return item
  }

  markStorageItemAccessed(item: DataStorageItem): void {
    item.accessCount++
    item.lastAccessed = Date.now()
  }

  invalidateMergedData(componentStorage: ComponentDataStorage): void {
    componentStorage.mergedData = undefined
  }

  private collectDataSourceItem(
    componentStorage: ComponentDataStorage,
    sourceId: string,
    item: DataStorageItem,
    collection: ValidComponentDataCollection
  ): void {
    if (this.options.isExpired(item)) {
      componentStorage.dataSources.delete(sourceId)
      return
    }

    collection.data[sourceId] = item.data
    collection.hasData = true
    this.markStorageItemAccessed(item)
  }

  private hasCompleteDataSource(componentData: Record<string, any>): boolean {
    return 'complete' in componentData && Boolean(componentData.complete)
  }

  private unwrapCompleteDataSource(completeData: any): Record<string, any> {
    if (this.hasNestedDeviceData(completeData)) {
      return completeData.deviceData.data
    }

    return completeData
  }

  private hasNestedDeviceData(data: any): boolean {
    return Boolean(data?.deviceData?.data)
  }

  private createMergedStorageItem(componentId: string, finalData: Record<string, any>): DataStorageItem {
    const now = Date.now()
    return {
      data: finalData,
      timestamp: now,
      expiresAt: now + this.options.getDefaultCacheExpiry(),
      source: {
        sourceId: '*merged*',
        sourceType: 'merged',
        componentId
      },
      size: this.options.calculateDataSize(finalData),
      accessCount: 1,
      lastAccessed: now
    }
  }
}
