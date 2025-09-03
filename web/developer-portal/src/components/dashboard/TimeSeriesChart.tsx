import React from 'react';
import { Card, Typography, Empty, Spin } from 'antd';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
  Area,
  AreaChart,
} from 'recharts';
import { AnalyticsSummaryResponse } from '../../types';

const { Title } = Typography;

interface TimeSeriesChartProps {
  data: AnalyticsSummaryResponse | null;
  loading?: boolean;
  title?: string;
  type?: 'line' | 'area';
  height?: number;
}

const TimeSeriesChart: React.FC<TimeSeriesChartProps> = ({
  data,
  loading = false,
  title = '请求趋势',
  type = 'area',
  height = 300,
}) => {
  const formatTimestamp = (timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleTimeString('zh-CN', {
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  const formatNumber = (value: number) => {
    if (value >= 1000000) {
      return (value / 1000000).toFixed(1) + 'M';
    }
    if (value >= 1000) {
      return (value / 1000).toFixed(1) + 'K';
    }
    return value.toString();
  };

  const chartData = data?.time_series?.map(item => ({
    time: formatTimestamp(item.timestamp),
    fullTime: item.timestamp,
    totalRequests: item.request_count,
    successRequests: item.success_count,
    errorRequests: item.error_count,
    avgResponseTime: item.avg_response_time,
    dataTransferred: item.data_transferred,
  })) || [];

  const CustomTooltip = ({ active, payload }: any) => {
    if (active && payload && payload.length) {
      const data = payload[0].payload;
      return (
        <div className="custom-tooltip" style={{
          backgroundColor: '#fff',
          padding: '12px',
          border: '1px solid #d9d9d9',
          borderRadius: '6px',
          boxShadow: '0 2px 8px rgba(0, 0, 0, 0.15)',
        }}>
          <p style={{ margin: 0, fontWeight: 'bold' }}>
            {new Date(data.fullTime).toLocaleString('zh-CN')}
          </p>
          {payload.map((entry: any, index: number) => (
            <p key={index} style={{ margin: '4px 0', color: entry.color }}>
              {entry.name}: {formatNumber(entry.value)}
              {entry.dataKey === 'avgResponseTime' && 'ms'}
            </p>
          ))}
        </div>
      );
    }
    return null;
  };

  if (loading) {
    return (
      <Card>
        <div style={{ textAlign: 'center', padding: '50px 0' }}>
          <Spin size="large" />
        </div>
      </Card>
    );
  }

  if (!data || !data.time_series || data.time_series.length === 0) {
    return (
      <Card>
        <Title level={4}>{title}</Title>
        <Empty description="暂无数据" />
      </Card>
    );
  }

  const ChartComponent = type === 'area' ? AreaChart : LineChart;
  const DataComponent = type === 'area' ? Area : Line;

  return (
    <Card>
      <Title level={4}>{title}</Title>
      <ResponsiveContainer width="100%" height={height}>
        <ChartComponent data={chartData} margin={{ top: 5, right: 30, left: 20, bottom: 5 }}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis 
            dataKey="time" 
            tick={{ fontSize: 12 }}
            interval="preserveStartEnd"
          />
          <YAxis 
            tick={{ fontSize: 12 }}
            tickFormatter={formatNumber}
          />
          <Tooltip content={<CustomTooltip />} />
          <Legend />
          
          <DataComponent
            type="monotone"
            dataKey="totalRequests"
            stroke="#1890ff"
            fill="#1890ff"
            fillOpacity={type === 'area' ? 0.1 : undefined}
            strokeWidth={2}
            name="总请求数"
            dot={{ r: 3 }}
            activeDot={{ r: 5 }}
          />
          
          <DataComponent
            type="monotone"
            dataKey="successRequests"
            stroke="#52c41a"
            fill="#52c41a"
            fillOpacity={type === 'area' ? 0.1 : undefined}
            strokeWidth={2}
            name="成功请求"
            dot={{ r: 3 }}
            activeDot={{ r: 5 }}
          />
          
          <DataComponent
            type="monotone"
            dataKey="errorRequests"
            stroke="#ff4d4f"
            fill="#ff4d4f"
            fillOpacity={type === 'area' ? 0.1 : undefined}
            strokeWidth={2}
            name="失败请求"
            dot={{ r: 3 }}
            activeDot={{ r: 5 }}
          />
        </ChartComponent>
      </ResponsiveContainer>
    </Card>
  );
};

export default TimeSeriesChart;
