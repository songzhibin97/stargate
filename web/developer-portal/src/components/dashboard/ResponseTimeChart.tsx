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
  ReferenceLine,
} from 'recharts';
import { AnalyticsSummaryResponse } from '../../types';

const { Title } = Typography;

interface ResponseTimeChartProps {
  data: AnalyticsSummaryResponse | null;
  loading?: boolean;
  height?: number;
}

const ResponseTimeChart: React.FC<ResponseTimeChartProps> = ({
  data,
  loading = false,
  height = 300,
}) => {
  const formatTimestamp = (timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleTimeString('zh-CN', {
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  const formatDuration = (value: number) => {
    if (value < 1000) {
      return `${value.toFixed(0)}ms`;
    }
    return `${(value / 1000).toFixed(2)}s`;
  };

  const chartData = data?.time_series?.map(item => ({
    time: formatTimestamp(item.timestamp),
    fullTime: item.timestamp,
    avgResponseTime: item.avg_response_time,
  })) || [];

  // Calculate average response time for reference line
  const avgResponseTime = chartData.length > 0 
    ? chartData.reduce((sum, item) => sum + item.avgResponseTime, 0) / chartData.length
    : 0;

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
          <p style={{ margin: '4px 0', color: '#722ed1' }}>
            响应时间: {formatDuration(data.avgResponseTime)}
          </p>
        </div>
      );
    }
    return null;
  };

  const getResponseTimeColor = (time: number): string => {
    if (time <= 100) return '#52c41a';
    if (time <= 500) return '#faad14';
    if (time <= 1000) return '#fa8c16';
    return '#ff4d4f';
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
        <Title level={4}>响应时间趋势</Title>
        <Empty description="暂无数据" />
      </Card>
    );
  }

  return (
    <Card>
      <Title level={4}>响应时间趋势</Title>
      <div style={{ marginBottom: 16 }}>
        <div style={{ display: 'flex', gap: 16, fontSize: 12, color: '#666' }}>
          <span>
            <span style={{ 
              display: 'inline-block', 
              width: 12, 
              height: 12, 
              backgroundColor: '#52c41a', 
              marginRight: 4 
            }}></span>
            优秀 (&lt;100ms)
          </span>
          <span>
            <span style={{ 
              display: 'inline-block', 
              width: 12, 
              height: 12, 
              backgroundColor: '#faad14', 
              marginRight: 4 
            }}></span>
            良好 (100-500ms)
          </span>
          <span>
            <span style={{ 
              display: 'inline-block', 
              width: 12, 
              height: 12, 
              backgroundColor: '#fa8c16', 
              marginRight: 4 
            }}></span>
            一般 (500ms-1s)
          </span>
          <span>
            <span style={{ 
              display: 'inline-block', 
              width: 12, 
              height: 12, 
              backgroundColor: '#ff4d4f', 
              marginRight: 4 
            }}></span>
            需要优化 (&gt;1s)
          </span>
        </div>
      </div>
      
      <ResponsiveContainer width="100%" height={height}>
        <LineChart data={chartData} margin={{ top: 5, right: 30, left: 20, bottom: 5 }}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis 
            dataKey="time" 
            tick={{ fontSize: 12 }}
            interval="preserveStartEnd"
          />
          <YAxis 
            tick={{ fontSize: 12 }}
            tickFormatter={formatDuration}
          />
          <Tooltip content={<CustomTooltip />} />
          <Legend />
          
          {/* Reference lines for performance thresholds */}
          <ReferenceLine
            y={100}
            stroke="#52c41a"
            strokeDasharray="5 5"
            label="100ms"
          />
          <ReferenceLine
            y={500}
            stroke="#faad14"
            strokeDasharray="5 5"
            label="500ms"
          />
          <ReferenceLine
            y={1000}
            stroke="#ff4d4f"
            strokeDasharray="5 5"
            label="1s"
          />

          {/* Average response time reference line */}
          {avgResponseTime > 0 && (
            <ReferenceLine
              y={avgResponseTime}
              stroke="#722ed1"
              strokeDasharray="2 2"
              label={`平均: ${formatDuration(avgResponseTime)}`}
            />
          )}
          
          <Line
            type="monotone"
            dataKey="avgResponseTime"
            stroke="#722ed1"
            strokeWidth={3}
            name="平均响应时间"
            dot={{ r: 4, fill: '#722ed1' }}
            activeDot={{ r: 6, fill: '#722ed1' }}
            connectNulls={false}
          />
        </LineChart>
      </ResponsiveContainer>
      
      <div style={{ 
        marginTop: 16, 
        padding: '12px', 
        backgroundColor: '#fafafa', 
        borderRadius: '6px',
        fontSize: 12,
        color: '#666'
      }}>
        <div style={{ display: 'flex', justifyContent: 'space-between' }}>
          <span>当前平均响应时间: <strong style={{ color: getResponseTimeColor(avgResponseTime) }}>
            {formatDuration(avgResponseTime)}
          </strong></span>
          <span>性能状态: <strong style={{ color: getResponseTimeColor(avgResponseTime) }}>
            {avgResponseTime <= 100 ? '优秀' : 
             avgResponseTime <= 500 ? '良好' : 
             avgResponseTime <= 1000 ? '一般' : '需要优化'}
          </strong></span>
        </div>
      </div>
    </Card>
  );
};

export default ResponseTimeChart;
