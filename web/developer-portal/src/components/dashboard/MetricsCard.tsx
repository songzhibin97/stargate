import React from 'react';
import { Card, Statistic, Row, Col, Typography, Tooltip } from 'antd';
import {
  ArrowUpOutlined,
  ArrowDownOutlined,
  InfoCircleOutlined,
} from '@ant-design/icons';
import { AnalyticsSummary } from '../../types';

const { Text } = Typography;

interface MetricsCardProps {
  summary: AnalyticsSummary;
  loading?: boolean;
}

const MetricsCard: React.FC<MetricsCardProps> = ({ summary, loading = false }) => {
  const formatNumber = (num: number): string => {
    if (num >= 1000000) {
      return (num / 1000000).toFixed(1) + 'M';
    }
    if (num >= 1000) {
      return (num / 1000).toFixed(1) + 'K';
    }
    return num.toString();
  };

  const formatBytes = (bytes: number): string => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const formatDuration = (ms: number): string => {
    if (ms < 1000) {
      return `${ms.toFixed(0)}ms`;
    }
    return `${(ms / 1000).toFixed(2)}s`;
  };

  const getSuccessRateColor = (rate: number): string => {
    if (rate >= 99) return '#52c41a';
    if (rate >= 95) return '#faad14';
    return '#ff4d4f';
  };

  const getResponseTimeColor = (time: number): string => {
    if (time <= 100) return '#52c41a';
    if (time <= 500) return '#faad14';
    return '#ff4d4f';
  };

  return (
    <Row gutter={[16, 16]}>
      {/* 总请求数 */}
      <Col xs={24} sm={12} md={6}>
        <Card loading={loading}>
          <Statistic
            title={
              <span>
                总请求数
                <Tooltip title="统计时间范围内的所有API请求总数">
                  <InfoCircleOutlined style={{ marginLeft: 4, color: '#999' }} />
                </Tooltip>
              </span>
            }
            value={summary.total_requests}
            formatter={(value) => formatNumber(Number(value))}
            valueStyle={{ color: '#1890ff' }}
            prefix={<ArrowUpOutlined />}
          />
          <div style={{ marginTop: 8 }}>
            <Text type="secondary" style={{ fontSize: 12 }}>
              成功: {formatNumber(summary.successful_requests)} | 
              失败: {formatNumber(summary.failed_requests)}
            </Text>
          </div>
        </Card>
      </Col>

      {/* 成功率 */}
      <Col xs={24} sm={12} md={6}>
        <Card loading={loading}>
          <Statistic
            title={
              <span>
                成功率
                <Tooltip title="成功请求数占总请求数的百分比">
                  <InfoCircleOutlined style={{ marginLeft: 4, color: '#999' }} />
                </Tooltip>
              </span>
            }
            value={summary.success_rate}
            precision={2}
            suffix="%"
            valueStyle={{ color: getSuccessRateColor(summary.success_rate) }}
            prefix={
              summary.success_rate >= 95 ? (
                <ArrowUpOutlined />
              ) : (
                <ArrowDownOutlined />
              )
            }
          />
          <div style={{ marginTop: 8 }}>
            <Text type="secondary" style={{ fontSize: 12 }}>
              {summary.success_rate >= 99 ? '优秀' : 
               summary.success_rate >= 95 ? '良好' : '需要关注'}
            </Text>
          </div>
        </Card>
      </Col>

      {/* 平均响应时间 */}
      <Col xs={24} sm={12} md={6}>
        <Card loading={loading}>
          <Statistic
            title={
              <span>
                平均响应时间
                <Tooltip title="所有请求的平均响应时间">
                  <InfoCircleOutlined style={{ marginLeft: 4, color: '#999' }} />
                </Tooltip>
              </span>
            }
            value={summary.avg_response_time}
            formatter={(value) => formatDuration(Number(value))}
            valueStyle={{ color: getResponseTimeColor(summary.avg_response_time) }}
            prefix={
              summary.avg_response_time <= 200 ? (
                <ArrowDownOutlined />
              ) : (
                <ArrowUpOutlined />
              )
            }
          />
          <div style={{ marginTop: 8 }}>
            <Text type="secondary" style={{ fontSize: 12 }}>
              P95: {formatDuration(summary.p95_response_time)} | 
              P99: {formatDuration(summary.p99_response_time)}
            </Text>
          </div>
        </Card>
      </Col>

      {/* 数据传输量 */}
      <Col xs={24} sm={12} md={6}>
        <Card loading={loading}>
          <Statistic
            title={
              <span>
                数据传输量
                <Tooltip title="统计时间范围内传输的总数据量">
                  <InfoCircleOutlined style={{ marginLeft: 4, color: '#999' }} />
                </Tooltip>
              </span>
            }
            value={summary.total_data_transferred}
            formatter={(value) => formatBytes(Number(value))}
            valueStyle={{ color: '#722ed1' }}
            prefix={<ArrowUpOutlined />}
          />
          <div style={{ marginTop: 8 }}>
            <Text type="secondary" style={{ fontSize: 12 }}>
              端点数: {summary.unique_endpoints}
            </Text>
          </div>
        </Card>
      </Col>
    </Row>
  );
};

export default MetricsCard;
