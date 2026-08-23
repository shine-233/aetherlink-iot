/**
 * 文件用途: RDI composable useRdiHistory 的单元测试。
 * 核心逻辑: 通过 mock API、store 或时间行为验证 composable 的状态输出、动作和异常分支。
 * 关键注意事项: 测试应聚焦 composable 契约，避免依赖 RDI 操作视图 DOM 细节。
 * 重构建议: 继续补成功、失败、空数据和清理生命周期用例，提升组合函数边界可信度。
 */
import { describe, expect, it, vi, type Mock } from 'vitest';
import { ref } from 'vue';

/**
 * 说明: useRdiHistory.ts 中以下纯函数为模块私有(未导出),无法直接进行单元测试:
 * - normalizeHistoryTimestamp: 历史时间戳归一化(秒/毫秒/ISO 字符串)
 * - normalizeHistoryValue: 历史数据值归一化(数字/布尔/字符串)
 * - csvEscape: CSV 字段转义(含逗号/引号/换行)
 * - formatHistoryChartValue: 图表值格式化(含温度单位转换)
 * - formatHistoryExportValue: 导出值格式化
 * - normalizeHistoryPoints / normalizeHistoryExportRows: 数据点归一化
 * - downloadCsv: CSV 下载
 *
 * 如需对这些私有函数进行直接测试,需在源码中将它们导出(本次任务不修改源码)。
 * 本测试文件针对已导出的 formatDurationLabel、formatEnergyValue 进行直接测试,
 * 并通过 composable 返回的 historyChartData + historyChartOptions 间接验证温度单位转换,
 * 通过 loadEnergyStatistics 间接验证时间戳与值的归一化逻辑。
 */

// Mock 外部依赖
vi.mock('@/service/api', () => ({
  rdiDeviceHistory: vi.fn()
}));

vi.mock('@/utils/common/discrete', () => ({
  message: {
    success: vi.fn(),
    error: vi.fn(),
    warning: vi.fn(),
    info: vi.fn()
  }
}));

vi.mock('@/utils/common/tool', () => ({
  getBaseServerUrl: () => 'http://localhost:8080/api/v1'
}));

import { useRdiHistory } from '../useRdiHistory';
import { rdiDeviceHistory } from '@/service/api';

/** 测试局部类型：rdiDeviceHistory 的最小契约(避免耦合生产类型)。 */
interface HistoryQueryParams {
  key?: string;
  page?: number;
  page_size?: number;
}

interface HistoryPointFixture {
  ts: number | string | null;
  value: unknown;
}

interface HistoryResultFixture {
  error: Error | null;
  data: { total?: number; list?: HistoryPointFixture[] } | null;
}

type RdiHistoryMock = Mock<(deviceId: string, params: HistoryQueryParams) => Promise<HistoryResultFixture>>;

function useHistoryMock() {
  return rdiDeviceHistory as unknown as RdiHistoryMock;
}

/** 测试局部类型：historyChartOptions 中被断言的最小字段。 */
interface ChartSeriesStub {
  name: string;
  data: Array<[number, number | null]>;
  lineStyle: { color: string };
  itemStyle: { color: string };
  connectNulls?: boolean;
}

interface ChartOptionsStub {
  series: ChartSeriesStub[];
  color: string[];
}

// 创建 composable 实例的辅助函数
function createComposable(unit: 'C' | 'F' = 'C') {
  return useRdiHistory(
    () => 'dev-1',
    () => unit,
    (key: string) => String(key)
  );
}

describe('useRdiHistory - 纯函数与格式化', () => {
  describe('RDI_DURATION_MAX_SECONDS 常量', () => {
    it('应为 24 小时对应的秒数', () => {
      const { RDI_DURATION_MAX_SECONDS } = createComposable();
      expect(RDI_DURATION_MAX_SECONDS).toBe(24 * 60 * 60);
      expect(RDI_DURATION_MAX_SECONDS).toBe(86400);
    });
  });

  describe('formatDurationLabel - 时长格式化', () => {
    it('0 秒返回 0H', () => {
      const { formatDurationLabel } = createComposable();
      expect(formatDurationLabel(0)).toBe('0H');
    });

    it('整分钟秒数返回 NM 格式', () => {
      const { formatDurationLabel } = createComposable();
      expect(formatDurationLabel(60)).toBe('1M');
      expect(formatDurationLabel(120)).toBe('2M');
      expect(formatDurationLabel(1800)).toBe('30M');
    });

    it('非整分钟秒数返回 NS 格式', () => {
      const { formatDurationLabel } = createComposable();
      expect(formatDurationLabel(90)).toBe('90S');
      expect(formatDurationLabel(45)).toBe('45S');
    });

    it('整小时秒数返回 NH 格式', () => {
      const { formatDurationLabel } = createComposable();
      expect(formatDurationLabel(3600)).toBe('1H');
      expect(formatDurationLabel(7200)).toBe('2H');
      expect(formatDurationLabel(21600)).toBe('6H');
    });

    it('小时+分钟组合返回 NH NM 格式', () => {
      const { formatDurationLabel } = createComposable();
      expect(formatDurationLabel(5400)).toBe('1H 30M'); // 1h30m
      expect(formatDurationLabel(9000)).toBe('2H 30M'); // 2h30m
    });

    it('小时整分钟部分为 0 时仅返回 NH', () => {
      const { formatDurationLabel } = createComposable();
      expect(formatDurationLabel(3601)).toBe('1H'); // 1h0m(四舍五入后为 0)
    });

    it('超过 24 小时的值被截断为 24H', () => {
      const { formatDurationLabel } = createComposable();
      expect(formatDurationLabel(90000)).toBe('24H'); // 25h -> 截断为 24h
      expect(formatDurationLabel(100000)).toBe('24H');
    });

    it('恰好 24 小时返回 24H', () => {
      const { formatDurationLabel } = createComposable();
      expect(formatDurationLabel(86400)).toBe('24H');
    });

    it('负数被归一化为 0H', () => {
      const { formatDurationLabel } = createComposable();
      expect(formatDurationLabel(-100)).toBe('0H');
    });

    it('null/undefined 被归一化为 0H', () => {
      const { formatDurationLabel } = createComposable();
      expect(formatDurationLabel(null)).toBe('0H');
      expect(formatDurationLabel(undefined)).toBe('0H');
    });

    it('NaN 字符串被归一化为 0H', () => {
      const { formatDurationLabel } = createComposable();
      expect(formatDurationLabel(NaN)).toBe('0H');
    });
  });

  describe('formatEnergyValue - 能量值格式化', () => {
    it('null 返回占位符 --', () => {
      const { formatEnergyValue } = createComposable();
      expect(formatEnergyValue(null)).toBe('--');
    });

    it('0 返回 0.00 kWh', () => {
      const { formatEnergyValue } = createComposable();
      expect(formatEnergyValue(0)).toBe('0.00 kWh');
    });

    it('正数保留两位小数并附加 kWh 单位', () => {
      const { formatEnergyValue } = createComposable();
      expect(formatEnergyValue(123.456)).toBe('123.46 kWh');
      expect(formatEnergyValue(1.5)).toBe('1.50 kWh');
    });

    it('负数同样格式化', () => {
      const { formatEnergyValue } = createComposable();
      expect(formatEnergyValue(-3.14159)).toBe('-3.14 kWh');
    });
  });

  describe('温度单位转换(通过 historyChartOptions 间接测试)', () => {
    it('摄氏度单位下温度值保持不变', () => {
      const composable = createComposable('C');
      composable.historyChartData.value = {
        temperature_1: [{ ts: 1000, value: 25 }]
      };
      const options = composable.historyChartOptions.value as unknown as ChartOptionsStub;
      // temperature_1 是 series[0]
      const tempSeries = options.series[0];
      expect(tempSeries.data[0]).toEqual([1000, 25]);
    });

    it('华氏度单位下温度值进行 C->F 转换(0C = 32F)', () => {
      const composable = createComposable('F');
      composable.historyChartData.value = {
        temperature_1: [{ ts: 1000, value: 0 }]
      };
      const options = composable.historyChartOptions.value as unknown as ChartOptionsStub;
      const tempSeries = options.series[0];
      expect(tempSeries.data[0]).toEqual([1000, 32]);
    });

    it('华氏度单位下温度值进行 C->F 转换(100C = 212F)', () => {
      const composable = createComposable('F');
      composable.historyChartData.value = {
        temperature_1: [{ ts: 2000, value: 100 }]
      };
      const options = composable.historyChartOptions.value as unknown as ChartOptionsStub;
      const tempSeries = options.series[0];
      expect(tempSeries.data[0]).toEqual([2000, 212]);
    });

    it('华氏度单位下 temperature_2 同样进行转换', () => {
      const composable = createComposable('F');
      composable.historyChartData.value = {
        temperature_2: [{ ts: 3000, value: 10 }]
      };
      const options = composable.historyChartOptions.value as unknown as ChartOptionsStub;
      const tempSeries = options.series[0];
      expect(tempSeries.data[0]).toEqual([3000, 50]);
    });

    it('非温度系列(如 switch_1)不进行单位转换', () => {
      const composable = createComposable('F');
      composable.historyChartData.value = {
        switch_1: [{ ts: 1000, value: 1 }]
      };
      const options = composable.historyChartOptions.value as unknown as ChartOptionsStub;
      const switchSeries = options.series[0];
      expect(switchSeries.data[0]).toEqual([1000, 1]);
    });
  });

  describe('历史曲线选择', () => {
    beforeEach(() => {
      vi.clearAllMocks();
      (rdiDeviceHistory as unknown as Mock).mockResolvedValue({ error: null, data: { list: [] } });
    });

    it('空选择时回退到默认全部序列', async () => {
      const composable = createComposable('C');
      composable.historyChartSeriesKeys.value = [];

      await composable.loadEnergyStatistics();

      const requestedKeys = (rdiDeviceHistory as unknown as Mock).mock.calls.map(
        ([, params]) => params.key
      );
      expect(requestedKeys).toEqual([
        'temperature_1',
        'temperature_2',
        'switch_1',
        'switch_2',
        'dry_contact_output',
        'electricity_consumption'
      ]);
      expect(composable.historyChartSeriesKeys.value).toEqual(requestedKeys);
    });

    it('非空选择时只请求并展示用户选择的序列', async () => {
      const composable = createComposable('C');
      composable.historyChartSeriesKeys.value = ['temperature_1', 'switch_2'];

      await composable.loadEnergyStatistics();

      const requestedKeys = (rdiDeviceHistory as unknown as Mock).mock.calls.map(
        ([, params]) => params.key
      );
      expect(requestedKeys).toEqual(['temperature_1', 'switch_2']);
      expect(composable.historyChartSeriesKeys.value).toEqual(['temperature_1', 'switch_2']);
      expect(Object.keys(composable.historyChartData.value)).toEqual(['temperature_1', 'switch_2']);
      expect((composable.historyChartOptions.value as unknown as ChartOptionsStub).series.map((series) => series.name)).toEqual([
        'T1',
        'SW2'
      ]);
    });

    it('为每个客户可见输入保持稳定且互不重复的曲线颜色', () => {
      const composable = createComposable('C');
      composable.historyChartData.value = {
        temperature_1: [{ ts: 1, value: 20 }],
        temperature_2: [{ ts: 1, value: 21 }],
        switch_1: [{ ts: 1, value: 0 }],
        switch_2: [{ ts: 1, value: 1 }],
        dry_contact_output: [{ ts: 1, value: 1 }],
        electricity_consumption: [{ ts: 1, value: 12.5 }]
      };

      const options = composable.historyChartOptions.value as unknown as ChartOptionsStub;
      const seriesColors = Object.fromEntries(
        options.series.map((series) => [series.name, series.lineStyle.color])
      );

      expect(seriesColors).toEqual({
        T1: '#f43f5e',
        T2: '#2563eb',
        SW1: '#7c3aed',
        SW2: '#f97316',
        DO: '#16a34a',
        kWh: '#0891b2'
      });
      expect(new Set(Object.values(seriesColors)).size).toBe(6);
      expect(options.color).toEqual(Object.values(seriesColors));
      expect(options.series.every((series) => series.itemStyle.color === series.lineStyle.color)).toBe(true);
    });
  });

  describe('history series pagination', () => {
    beforeEach(() => {
      vi.clearAllMocks();
    });

    it('loads every page reported by total for a 30-day series sampled every 45 seconds', async () => {
      const mockedRdiHistory = useHistoryMock();
      const total = (30 * 24 * 60 * 60) / 45;
      const baseTs = 1_700_000_000_000;

      mockedRdiHistory.mockImplementation((_, params) => {
        const firstIndex = (params.page - 1) * params.page_size;
        const itemCount = Math.max(0, Math.min(params.page_size, total - firstIndex));
        return Promise.resolve({
          error: null,
          data: {
            total,
            list: Array.from({ length: itemCount }, (_, index) => ({
              ts: baseTs + firstIndex + index,
              value: firstIndex + index
            }))
          }
        });
      });

      const composable = createComposable('C');
      composable.energyRange.value = 'last_30d';
      composable.historyChartSeriesKeys.value = ['temperature_1'];

      await composable.loadEnergyStatistics();

      const requests = mockedRdiHistory.mock.calls.map(([, params]) => params);
      expect(requests).toHaveLength(12);
      expect(requests.map((params) => params.page)).toEqual([1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12]);
      expect(requests.every((params) => params.page_size === 5000)).toBe(true);
      expect(composable.historyChartData.value.temperature_1).toHaveLength(total);
    });

    it('continues from total across short pages and removes duplicate boundary points', async () => {
      const mockedRdiHistory = useHistoryMock();
      mockedRdiHistory.mockImplementation((_, params) => {
        if (params.page === 1) {
          return Promise.resolve({
            error: null,
            data: {
              total: 3,
              list: [
                { ts: 3000, value: 3 },
                { ts: 2000, value: 2 }
              ]
            }
          });
        }
        return Promise.resolve({
          error: null,
          data: {
            total: 3,
            list: [
              { ts: 2000, value: 2 },
              { ts: 1000, value: 1 }
            ]
          }
        });
      });

      const composable = createComposable('C');
      composable.historyChartSeriesKeys.value = ['temperature_1'];

      await composable.loadEnergyStatistics();

      expect(mockedRdiHistory.mock.calls.map(([, params]) => params.page)).toEqual([1, 2]);
      // 本用例只验证跨页去重后的数据点，null 值为断线 gap 标记（由 line 480 用例专门覆盖），此处过滤。
      const dataPoints = composable.historyChartData.value.temperature_1.filter((point) => point.value !== null);
      expect(dataPoints).toEqual([
        { ts: 1000000, value: 1 },
        { ts: 2000000, value: 2 },
        { ts: 3000000, value: 3 }
      ]);
    });

    it('stops when a backend repeats a page without adding any new point', async () => {
      const mockedRdiHistory = useHistoryMock();
      mockedRdiHistory.mockResolvedValue({
        error: null,
        data: {
          total: 10,
          list: [
            { ts: 2000, value: 2 },
            { ts: 1000, value: 1 }
          ]
        }
      });

      const composable = createComposable('C');
      composable.historyChartSeriesKeys.value = ['temperature_1'];

      await composable.loadEnergyStatistics();

      expect(mockedRdiHistory).toHaveBeenCalledTimes(2);
      // 只统计数据点，剔除断线 gap 标记（null 值），后者由 line 480 用例专门覆盖。
      const dataPoints = composable.historyChartData.value.temperature_1.filter((point) => point.value !== null);
      expect(dataPoints).toHaveLength(2);
    });
  });

  describe('history failure and sampling-gap evidence', () => {
    beforeEach(() => {
      vi.clearAllMocks();
    });

    it('marks a first-page API error as failed instead of treating it as a successful empty series', async () => {
      (rdiDeviceHistory as unknown as Mock).mockResolvedValue({
        error: new Error('history request failed'),
        data: null
      });

      const composable = createComposable('C');
      composable.historyChartSeriesKeys.value = ['temperature_1'];

      await composable.loadEnergyStatistics();

      expect(composable.historyChartData.value.temperature_1).toEqual([]);
      expect(composable.failedHistorySeriesLabels.value).toEqual(['T1']);
      expect(composable.partialHistorySeriesLabels.value).toEqual([]);
      expect(composable.hasHistoryFailures.value).toBe(true);
      expect(composable.hasSuccessfulHistoryData.value).toBe(false);
    });

    it('marks a thrown request exception as failed instead of treating it as a successful empty series', async () => {
      useHistoryMock().mockRejectedValue(new Error('network unavailable'));

      const composable = createComposable('C');
      composable.historyChartSeriesKeys.value = ['temperature_1'];

      await composable.loadEnergyStatistics();

      expect(composable.historyChartData.value.temperature_1).toEqual([]);
      expect(composable.failedHistorySeriesLabels.value).toEqual(['T1']);
      expect(composable.hasHistoryFailures.value).toBe(true);
    });

    it('keeps successful empty energy data distinct from a failed energy request', async () => {
      const mockedRdiHistory = useHistoryMock();
      const composable = createComposable('C');
      composable.historyChartSeriesKeys.value = ['electricity_consumption'];

      mockedRdiHistory.mockResolvedValueOnce({ error: null, data: { total: 0, list: [] } });
      await composable.loadEnergyStatistics();

      expect(composable.failedHistorySeriesLabels.value).toEqual([]);
      expect(composable.hasHistoryFailures.value).toBe(false);
      expect(composable.energyStatisticsAvailable.value).toBe(true);
      expect(composable.energyStats.sample_count).toBe(0);

      mockedRdiHistory.mockResolvedValueOnce({ error: new Error('timeout'), data: null });
      await composable.loadEnergyStatistics();

      expect(composable.failedHistorySeriesLabels.value).toEqual(['kWh']);
      expect(composable.hasHistoryFailures.value).toBe(true);
      expect(composable.energyStatisticsAvailable.value).toBe(false);
    });

    it('preserves points from completed pages when a later page fails and exposes partial-series evidence', async () => {
      const mockedRdiHistory = useHistoryMock();
      const baseTs = 1_700_000_000_000;
      mockedRdiHistory.mockImplementation((_, params) => {
        if (params.page === 1) {
          return Promise.resolve({
            error: null,
            data: {
              total: 3,
              list: [
                { ts: baseTs + 45_000, value: 21 },
                { ts: baseTs, value: 20 }
              ]
            }
          });
        }
        return Promise.resolve({ error: new Error('page 2 failed'), data: null });
      });

      const composable = createComposable('C');
      composable.historyChartSeriesKeys.value = ['temperature_1'];

      await composable.loadEnergyStatistics();

      expect(mockedRdiHistory.mock.calls.map(([, params]) => params.page)).toEqual([1, 2]);
      expect(composable.historyChartData.value.temperature_1).toEqual([
        { ts: baseTs, value: 20 },
        { ts: baseTs + 45_000, value: 21 }
      ]);
      expect(composable.failedHistorySeriesLabels.value).toEqual([]);
      expect(composable.partialHistorySeriesLabels.value).toEqual(['T1']);
      expect(composable.hasSuccessfulHistoryData.value).toBe(true);
    });

    it('inserts a null marker for a sampling interval over 90 seconds and configures the chart not to bridge it', async () => {
      const baseTs = 1_700_000_000_000;
      (rdiDeviceHistory as unknown as Mock).mockResolvedValue({
        error: null,
        data: {
          total: 2,
          list: [
            { ts: baseTs + 180_000, value: 21 },
            { ts: baseTs, value: 20 }
          ]
        }
      });

      const composable = createComposable('C');
      composable.historyChartSeriesKeys.value = ['temperature_1'];

      await composable.loadEnergyStatistics();

      expect(composable.historyChartData.value.temperature_1).toEqual([
        { ts: baseTs, value: 20 },
        { ts: baseTs + 90_000, value: null },
        { ts: baseTs + 180_000, value: 21 }
      ]);
      expect(composable.gappedHistorySeriesLabels.value).toEqual(['T1']);

      const temperatureSeries = (composable.historyChartOptions.value as unknown as ChartOptionsStub).series[0];
      expect(temperatureSeries.connectNulls).toBe(false);
      expect(temperatureSeries.data).toEqual([
        [baseTs, 20],
        [baseTs + 90_000, null],
        [baseTs + 180_000, 21]
      ]);
    });

    it('clears failed, partial and gap labels synchronously when the device changes', async () => {
      const currentDeviceId = ref('device-1');
      (rdiDeviceHistory as unknown as Mock).mockResolvedValue({
        error: new Error('history request failed'),
        data: null
      });
      const composable = useRdiHistory(
        () => currentDeviceId.value,
        () => 'C',
        (key: string) => String(key)
      );
      composable.historyChartSeriesKeys.value = ['temperature_1'];

      await composable.loadEnergyStatistics();
      expect(composable.failedHistorySeriesLabels.value).toEqual(['T1']);

      currentDeviceId.value = 'device-2';

      expect(composable.failedHistorySeriesLabels.value).toEqual([]);
      expect(composable.partialHistorySeriesLabels.value).toEqual([]);
      expect(composable.gappedHistorySeriesLabels.value).toEqual([]);
      expect(composable.hasHistoryFailures.value).toBe(false);
      expect(composable.historyChartData.value).toEqual({});
    });
  });

  describe('时间戳与值归一化(通过 loadEnergyStatistics 间接测试)', () => {
    beforeEach(() => {
      vi.clearAllMocks();
    });

    it('秒级时间戳被转换为毫秒级', async () => {
      // 秒级时间戳 1700000000 -> 毫秒 1700000000000
      const mockedRdiHistory = useHistoryMock();
      mockedRdiHistory.mockImplementation((_, params) => {
        if (params.key === 'electricity_consumption') {
          return Promise.resolve({
            error: null,
            data: { list: [{ ts: 1700000000, value: 1.5 }] }
          });
        }
        return Promise.resolve({ error: null, data: { list: [] } });
      });

      const composable = createComposable('C');
      await composable.loadEnergyStatistics();
      const points = composable.historyChartData.value.electricity_consumption || [];
      expect(points.length).toBe(1);
      expect(points[0].ts).toBe(1700000000000);
      expect(points[0].value).toBe(1.5);
    });

    it('毫秒级时间戳保持不变', async () => {
      const mockedRdiHistory = useHistoryMock();
      mockedRdiHistory.mockImplementation((_, params) => {
        if (params.key === 'electricity_consumption') {
          return Promise.resolve({
            error: null,
            data: { list: [{ ts: 1700000000000, value: 2.0 }] }
          });
        }
        return Promise.resolve({ error: null, data: { list: [] } });
      });

      const composable = createComposable('C');
      await composable.loadEnergyStatistics();
      const points = composable.historyChartData.value.electricity_consumption || [];
      expect(points.length).toBe(1);
      expect(points[0].ts).toBe(1700000000000);
    });

    it('布尔值 true 被归一化为 1', async () => {
      const mockedRdiHistory = useHistoryMock();
      mockedRdiHistory.mockImplementation((_, params) => {
        if (params.key === 'switch_1') {
          return Promise.resolve({
            error: null,
            data: { list: [{ ts: 1700000000000, value: true }] }
          });
        }
        return Promise.resolve({ error: null, data: { list: [] } });
      });

      const composable = createComposable('C');
      await composable.loadEnergyStatistics();
      const points = composable.historyChartData.value.switch_1 || [];
      expect(points.length).toBe(1);
      expect(points[0].value).toBe(1);
    });

    it('字符串 "on"/"off" 被归一化为 1/0', async () => {
      const mockedRdiHistory = useHistoryMock();
      mockedRdiHistory.mockImplementation((_, params) => {
        if (params.key === 'switch_1') {
          return Promise.resolve({
            error: null,
            data: {
              list: [
                { ts: 1700000000000, value: 'on' },
                { ts: 1700000001000, value: 'off' }
              ]
            }
          });
        }
        return Promise.resolve({ error: null, data: { list: [] } });
      });

      const composable = createComposable('C');
      await composable.loadEnergyStatistics();
      const points = composable.historyChartData.value.switch_1 || [];
      expect(points.length).toBe(2);
      expect(points[0].value).toBe(1);
      expect(points[1].value).toBe(0);
    });

    it('数据点按时间戳升序排序', async () => {
      const mockedRdiHistory = useHistoryMock();
      mockedRdiHistory.mockImplementation((_, params) => {
        if (params.key === 'electricity_consumption') {
          return Promise.resolve({
            error: null,
            data: {
              list: [
                { ts: 1700000003000, value: 3 },
                { ts: 1700000001000, value: 1 },
                { ts: 1700000002000, value: 2 }
              ]
            }
          });
        }
        return Promise.resolve({ error: null, data: { list: [] } });
      });

      const composable = createComposable('C');
      await composable.loadEnergyStatistics();
      const points = composable.historyChartData.value.electricity_consumption || [];
      expect(points.map(p => p.value)).toEqual([1, 2, 3]);
    });

    it('无效时间戳的数据点被过滤', async () => {
      const mockedRdiHistory = useHistoryMock();
      mockedRdiHistory.mockImplementation((_, params) => {
        if (params.key === 'electricity_consumption') {
          return Promise.resolve({
            error: null,
            data: {
              list: [
                { ts: 1700000000000, value: 1 },
                { ts: 'invalid', value: 2 },
                { ts: null, value: 3 }
              ]
            }
          });
        }
        return Promise.resolve({ error: null, data: { list: [] } });
      });

      const composable = createComposable('C');
      await composable.loadEnergyStatistics();
      const points = composable.historyChartData.value.electricity_consumption || [];
      expect(points.length).toBe(1);
      expect(points[0].value).toBe(1);
    });
  });

  describe('energyStats 统计(通过 loadEnergyStatistics 间接测试)', () => {
    beforeEach(() => {
      vi.clearAllMocks();
    });

    it('正确计算数据点数、最新值、最小值、最大值、增量', async () => {
      const mockedRdiHistory = useHistoryMock();
      mockedRdiHistory.mockImplementation((_, params) => {
        if (params.key === 'electricity_consumption') {
          return Promise.resolve({
            error: null,
            data: {
              list: [
                { ts: 1000, value: 10 },
                { ts: 2000, value: 20 },
                { ts: 3000, value: 15 }
              ]
            }
          });
        }
        return Promise.resolve({ error: null, data: { list: [] } });
      });

      const composable = createComposable('C');
      await composable.loadEnergyStatistics();
      expect(composable.energyStats.sample_count).toBe(3);
      expect(composable.energyStats.latest).toBe(15);
      expect(composable.energyStats.min).toBe(10);
      expect(composable.energyStats.max).toBe(20);
      // delta = max(0, last - first) = max(0, 15 - 10) = 5
      expect(composable.energyStats.delta).toBe(5);
    });

    it('空数据时统计值归零', async () => {
      const mockedRdiHistory = useHistoryMock();
      mockedRdiHistory.mockResolvedValue({ error: null, data: { list: [] } });

      const composable = createComposable('C');
      await composable.loadEnergyStatistics();
      expect(composable.energyStats.sample_count).toBe(0);
      expect(composable.energyStats.latest).toBeNull();
      expect(composable.energyStats.min).toBeNull();
      expect(composable.energyStats.max).toBeNull();
      expect(composable.energyStats.delta).toBeNull();
    });
  });
});
