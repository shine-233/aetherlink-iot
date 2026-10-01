/*
 * AMap lifecycle for the equipment map: SDK loading, map creation, marker sync and teardown.
 */
import { onBeforeUnmount, ref } from 'vue'
import { useScriptTag } from '@vueuse/core'
import { AMAP_SDK_URL, ensureAmapSecurityConfig } from '@/constants/map-sdk'

const DEFAULT_CENTER: [number, number] = [114.05834626586915, 22.546789983033168]

export function useEquipmentAmap() {
  const { load: loadAmap } = useScriptTag(AMAP_SDK_URL, undefined, { manual: true })
  const mapDomRef = ref<HTMLDivElement | null>(null)
  const mapLoading = ref(false)
  const mapReady = ref(false)
  const mapError = ref(false)

  let amapInstance: any = null
  let amapMarker: any = null

  async function initMap(center: [number, number] | null) {
    if (mapReady.value || mapLoading.value || !mapDomRef.value) return
    if (!AMAP_SDK_URL) {
      mapError.value = true
      return
    }

    mapLoading.value = true
    mapError.value = false
    try {
      ensureAmapSecurityConfig()
      await loadAmap(true)
      if (!mapDomRef.value || typeof AMap === 'undefined') return
      const mapOptions = {
        zoom: center ? 13 : 5,
        center: center || DEFAULT_CENTER,
        viewMode: '3D',
        resizeEnable: true
      } as AMap.MapOptions & { resizeEnable: boolean }
      amapInstance = new AMap.Map(mapDomRef.value, mapOptions)
      mapReady.value = true
      updateMarker(center)
    } catch {
      mapError.value = true
    } finally {
      mapLoading.value = false
    }
  }

  function updateMarker(position: [number, number] | null) {
    if (!mapReady.value || !amapInstance || typeof AMap === 'undefined') return

    if (!position) {
      if (amapMarker) {
        amapInstance.remove(amapMarker)
        amapMarker = null
      }
      return
    }

    if (!amapMarker) {
      amapMarker = new AMap.Marker({ position, anchor: 'bottom-center' })
      amapInstance.add(amapMarker)
    } else {
      amapMarker.setPosition(position)
    }
    amapInstance.setZoomAndCenter(13, position)
  }

  onBeforeUnmount(() => {
    if (amapInstance) {
      amapInstance.destroy()
      amapInstance = null
      amapMarker = null
    }
  })

  return { mapDomRef, mapLoading, mapReady, mapError, initMap, updateMarker }
}
