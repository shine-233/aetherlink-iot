import {
  buildFirstDeviceSuccessProofPacket,
  type FirstDeviceSuccessProofPacket
} from './homeFirstDeviceSuccessProof'

export interface FirstDeviceProofDeliveryState {
  device: any
  accessGuide: any
  simulation: any
  readyProof: any
  onboardingGuard: any
  chart: any
  browserTest: any
  deploymentHealthRows: any[]
}

export interface FirstDeviceSupportSummaryDelivery {
  firstDeviceUrl: string
  proofUrl: string
  proofFileHint: string
}

export const buildFirstDeviceWorkbenchUrl = (focus = 'quickstart', origin?: string) => {
  const path = `/home?onboarding=first-device&focus=${encodeURIComponent(focus)}`
  return origin ? new URL(path, origin).toString() : path
}

export const buildFirstDeviceEntryUrl = (origin?: string) => {
  return origin ? new URL('/first-device', origin).toString() : '/first-device'
}

export const buildFirstDeviceProofFilename = (device: any) => {
  const number = String(device?.number || device?.name || 'first-device')
    .replace(/[^a-zA-Z0-9._-]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 48)
  return `aetherlink-first-device-proof-${number || 'first-device'}.json`
}

export const buildFirstDeviceProofDelivery = (
  state: FirstDeviceProofDeliveryState,
  origin?: string
): FirstDeviceSupportSummaryDelivery => ({
  firstDeviceUrl: buildFirstDeviceEntryUrl(origin),
  proofUrl: buildFirstDeviceWorkbenchUrl('proof', origin),
  proofFileHint: buildFirstDeviceProofFilename(state.device)
})

export const buildFirstDeviceSuccessProofDeliveryPacket = (
  state: FirstDeviceProofDeliveryState,
  origin?: string
): FirstDeviceSuccessProofPacket =>
  buildFirstDeviceSuccessProofPacket({
    device: state.device,
    accessGuide: state.accessGuide,
    simulation: state.simulation,
    readyProof: state.readyProof,
    onboardingGuard: state.onboardingGuard,
    chart: state.chart,
    browserTest: state.browserTest,
    deploymentHealthRows: state.deploymentHealthRows,
    delivery: {
      ...buildFirstDeviceProofDelivery(state, origin),
      generatedFromPage: '/first-device'
    }
  })

export const downloadFirstDeviceSuccessProofPacket = (
  packet: FirstDeviceSuccessProofPacket,
  filename: string,
  documentRef: Document = document,
  urlRef: Pick<typeof URL, 'createObjectURL' | 'revokeObjectURL'> = URL
) => {
  const blob = new Blob([JSON.stringify(packet, null, 2)], {
    type: 'application/json;charset=utf-8'
  })
  const url = urlRef.createObjectURL(blob)
  const link = documentRef.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  urlRef.revokeObjectURL(url)
}
