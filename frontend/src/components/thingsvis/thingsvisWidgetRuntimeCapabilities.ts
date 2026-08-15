export const THINGSVIS_WIDGET_CARTESIAN_CHART_TYPES = [
  'chart/line',
  'chart/bar',
  'chart/time-series',
  'chart/timeseries',
  'line-chart',
  'bar-chart',
  'time-series-chart',
  'timeseries-chart'
]

export const THINGSVIS_WIDGET_PIE_CHART_TYPES = ['chart/pie', 'pie-chart']
export const THINGSVIS_WIDGET_GAUGE_CHART_TYPES = ['chart/gauge', 'gauge-chart']

export const THINGSVIS_WIDGET_MODEL_3D_TYPES = [
  'media/3d-model',
  'media/model-viewer',
  'model/3d',
  'model-viewer',
  'three/model',
  '3d-model'
]

export const THINGSVIS_WIDGET_MODEL_3D_ACCEPTED_EXTENSIONS = ['.glb', '.gltf', '.obj', '.fbx', '.stl']
export const THINGSVIS_WIDGET_MODEL_3D_MAX_UPLOAD_SIZE_MB = 1000

export const THINGSVIS_WIDGET_RUNTIME_CAPABILITIES = {
  version: 1,
  chartFontSizes: {
    supported: true,
    propsKey: 'fontSizes',
    componentTypes: {
      cartesian: THINGSVIS_WIDGET_CARTESIAN_CHART_TYPES,
      pie: THINGSVIS_WIDGET_PIE_CHART_TYPES,
      gauge: THINGSVIS_WIDGET_GAUGE_CHART_TYPES
    }
  },
  model3d: {
    supported: false,
    hostContractSupported: true,
    runtimeRenderingVerified: false,
    requiresExternalRuntime: true,
    componentTypes: THINGSVIS_WIDGET_MODEL_3D_TYPES,
    acceptedExtensions: THINGSVIS_WIDGET_MODEL_3D_ACCEPTED_EXTENSIONS,
    maxUploadSizeMb: THINGSVIS_WIDGET_MODEL_3D_MAX_UPLOAD_SIZE_MB,
    viewerProps: ['modelUrl', 'src', 'assetUrl', 'format', 'cameraControls', 'autoRotate', 'backgroundColor']
  }
} as const
