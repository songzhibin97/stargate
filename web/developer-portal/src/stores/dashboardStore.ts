import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { DashboardState, AnalyticsSummaryResponse, Application } from '../types';
import { apiService } from '../services/api';

interface DashboardStore extends DashboardState {
  // Data
  analyticsData: AnalyticsSummaryResponse | null;
  applications: Application[];
  loading: boolean;
  error: string | null;
  lastUpdated: Date | null;

  // Actions
  setTimeRange: (timeRange: string) => void;
  setSelectedApplications: (applicationIds: string[]) => void;
  setGranularity: (granularity: string) => void;
  setAutoRefresh: (autoRefresh: boolean) => void;
  setRefreshInterval: (interval: number) => void;
  
  // Data fetching
  fetchAnalyticsData: () => Promise<void>;
  fetchApplications: () => Promise<void>;
  refreshData: () => Promise<void>;
  
  // Utility
  clearError: () => void;
  reset: () => void;
}

const initialState: DashboardState = {
  timeRange: '24h',
  selectedApplications: [],
  granularity: '1h',
  autoRefresh: true,
  refreshInterval: 30000, // 30 seconds
};

export const useDashboardStore = create<DashboardStore>()(
  persist(
    (set, get) => ({
      // Initial state
      ...initialState,
      analyticsData: null,
      applications: [],
      loading: false,
      error: null,
      lastUpdated: null,

      // Actions
      setTimeRange: (timeRange: string) => {
        set({ timeRange });
        // Auto-refresh data when time range changes
        get().fetchAnalyticsData();
      },

      setSelectedApplications: (selectedApplications: string[]) => {
        set({ selectedApplications });
        // Auto-refresh data when selection changes
        get().fetchAnalyticsData();
      },

      setGranularity: (granularity: string) => {
        set({ granularity });
        // Auto-refresh data when granularity changes
        get().fetchAnalyticsData();
      },

      setAutoRefresh: (autoRefresh: boolean) => {
        set({ autoRefresh });
      },

      setRefreshInterval: (refreshInterval: number) => {
        set({ refreshInterval });
      },

      // Data fetching
      fetchAnalyticsData: async () => {
        const state = get();
        set({ loading: true, error: null });

        try {
          const params = {
            time_range: state.timeRange,
            granularity: state.granularity,
            application_ids: state.selectedApplications.length > 0 ? state.selectedApplications : undefined,
          };

          const data = await apiService.getAnalyticsSummary(params);
          set({ 
            analyticsData: data, 
            loading: false, 
            lastUpdated: new Date(),
            error: null 
          });
        } catch (error: any) {
          set({ 
            loading: false, 
            error: error.message || 'Failed to fetch analytics data',
            analyticsData: null 
          });
        }
      },

      fetchApplications: async () => {
        try {
          const response = await apiService.getApplications();
          set({ applications: response.applications });
        } catch (error: any) {
          console.error('Failed to fetch applications:', error);
          // Don't set error state for applications fetch failure
          // as it's not critical for dashboard functionality
        }
      },

      refreshData: async () => {
        await Promise.all([
          get().fetchAnalyticsData(),
          get().fetchApplications(),
        ]);
      },

      // Utility
      clearError: () => {
        set({ error: null });
      },

      reset: () => {
        set({
          ...initialState,
          analyticsData: null,
          applications: [],
          loading: false,
          error: null,
          lastUpdated: null,
        });
      },
    }),
    {
      name: 'dashboard-storage',
      partialize: (state) => ({
        timeRange: state.timeRange,
        selectedApplications: state.selectedApplications,
        granularity: state.granularity,
        autoRefresh: state.autoRefresh,
        refreshInterval: state.refreshInterval,
      }),
    }
  )
);

// Auto-refresh hook
let refreshTimer: NodeJS.Timeout | null = null;

export const useAutoRefresh = () => {
  const { autoRefresh, refreshInterval, fetchAnalyticsData } = useDashboardStore();

  const startAutoRefresh = () => {
    if (refreshTimer) {
      clearInterval(refreshTimer);
    }

    if (autoRefresh && refreshInterval > 0) {
      refreshTimer = setInterval(() => {
        fetchAnalyticsData();
      }, refreshInterval);
    }
  };

  const stopAutoRefresh = () => {
    if (refreshTimer) {
      clearInterval(refreshTimer);
      refreshTimer = null;
    }
  };

  return { startAutoRefresh, stopAutoRefresh };
};

// Time range options
export const TIME_RANGE_OPTIONS = [
  { label: '1小时', value: '1h' },
  { label: '6小时', value: '6h' },
  { label: '12小时', value: '12h' },
  { label: '24小时', value: '24h' },
  { label: '7天', value: '7d' },
  { label: '30天', value: '30d' },
];

// Granularity options
export const GRANULARITY_OPTIONS = [
  { label: '1分钟', value: '1m' },
  { label: '5分钟', value: '5m' },
  { label: '15分钟', value: '15m' },
  { label: '30分钟', value: '30m' },
  { label: '1小时', value: '1h' },
  { label: '6小时', value: '6h' },
  { label: '1天', value: '1d' },
];

// Refresh interval options
export const REFRESH_INTERVAL_OPTIONS = [
  { label: '10秒', value: 10000 },
  { label: '30秒', value: 30000 },
  { label: '1分钟', value: 60000 },
  { label: '5分钟', value: 300000 },
  { label: '关闭', value: 0 },
];

// Utility functions
export const formatTimeRange = (timeRange: string): string => {
  const option = TIME_RANGE_OPTIONS.find(opt => opt.value === timeRange);
  return option ? option.label : timeRange;
};

export const formatGranularity = (granularity: string): string => {
  const option = GRANULARITY_OPTIONS.find(opt => opt.value === granularity);
  return option ? option.label : granularity;
};

export const formatRefreshInterval = (interval: number): string => {
  const option = REFRESH_INTERVAL_OPTIONS.find(opt => opt.value === interval);
  return option ? option.label : `${interval / 1000}秒`;
};

// Data transformation utilities
export const transformTimeSeriesData = (data: AnalyticsSummaryResponse) => {
  if (!data.time_series || data.time_series.length === 0) {
    return { labels: [], datasets: [] };
  }

  const labels = data.time_series.map(item => {
    const date = new Date(item.timestamp);
    return date.toLocaleTimeString('zh-CN', { 
      hour: '2-digit', 
      minute: '2-digit' 
    });
  });

  const requestData = data.time_series.map(item => item.request_count);
  const successData = data.time_series.map(item => item.success_count);
  const errorData = data.time_series.map(item => item.error_count);

  return {
    labels,
    datasets: [
      {
        label: '总请求数',
        data: requestData,
        borderColor: '#1890ff',
        backgroundColor: 'rgba(24, 144, 255, 0.1)',
        fill: true,
      },
      {
        label: '成功请求',
        data: successData,
        borderColor: '#52c41a',
        backgroundColor: 'rgba(82, 196, 26, 0.1)',
        fill: true,
      },
      {
        label: '失败请求',
        data: errorData,
        borderColor: '#ff4d4f',
        backgroundColor: 'rgba(255, 77, 79, 0.1)',
        fill: true,
      },
    ],
  };
};

export const transformResponseTimeData = (data: AnalyticsSummaryResponse) => {
  if (!data.time_series || data.time_series.length === 0) {
    return { labels: [], datasets: [] };
  }

  const labels = data.time_series.map(item => {
    const date = new Date(item.timestamp);
    return date.toLocaleTimeString('zh-CN', { 
      hour: '2-digit', 
      minute: '2-digit' 
    });
  });

  const responseTimeData = data.time_series.map(item => item.avg_response_time);

  return {
    labels,
    datasets: [
      {
        label: '平均响应时间 (ms)',
        data: responseTimeData,
        borderColor: '#722ed1',
        backgroundColor: 'rgba(114, 46, 209, 0.1)',
        fill: true,
      },
    ],
  };
};
