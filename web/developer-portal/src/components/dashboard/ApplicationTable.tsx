import React from 'react';
import { Card, Table, Typography, Tag, Progress, Space } from 'antd';
import { ColumnsType } from 'antd/es/table';
import {
  AppstoreOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  CloudUploadOutlined,
} from '@ant-design/icons';
import { ApplicationAnalytics } from '../../types';

const { Title, Text } = Typography;

interface ApplicationTableProps {
  applications: ApplicationAnalytics[];
  loading?: boolean;
}

const ApplicationTable: React.FC<ApplicationTableProps> = ({
  applications,
  loading = false,
}) => {
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
    if (rate >= 99) return 'success';
    if (rate >= 95) return 'warning';
    return 'error';
  };

  const getResponseTimeStatus = (time: number): { status: 'success' | 'normal' | 'exception', text: string } => {
    if (time <= 100) return { status: 'success', text: '优秀' };
    if (time <= 500) return { status: 'normal', text: '良好' };
    return { status: 'exception', text: '需要优化' };
  };

  const columns: ColumnsType<ApplicationAnalytics> = [
    {
      title: '应用名称',
      dataIndex: 'application_name',
      key: 'application_name',
      width: 200,
      fixed: 'left',
      render: (name: string, record: ApplicationAnalytics) => (
        <Space>
          <AppstoreOutlined style={{ color: '#1890ff' }} />
          <div>
            <div style={{ fontWeight: 500 }}>{name}</div>
            <Text type="secondary" style={{ fontSize: 12 }}>
              {record.application_id}
            </Text>
          </div>
        </Space>
      ),
    },
    {
      title: '请求统计',
      key: 'requests',
      width: 150,
      render: (_, record: ApplicationAnalytics) => (
        <div>
          <div style={{ fontWeight: 500, marginBottom: 4 }}>
            {formatNumber(record.total_requests)}
          </div>
          <div style={{ fontSize: 12, color: '#666' }}>
            <Space size={8}>
              <span style={{ color: '#52c41a' }}>
                <CheckCircleOutlined /> {formatNumber(record.successful_requests)}
              </span>
              <span style={{ color: '#ff4d4f' }}>
                <CloseCircleOutlined /> {formatNumber(record.failed_requests)}
              </span>
            </Space>
          </div>
        </div>
      ),
    },
    {
      title: '成功率',
      dataIndex: 'success_rate',
      key: 'success_rate',
      width: 120,
      sorter: (a, b) => a.success_rate - b.success_rate,
      render: (rate: number) => (
        <div>
          <Progress
            percent={rate}
            size="small"
            status={rate >= 95 ? 'success' : rate >= 90 ? 'normal' : 'exception'}
            format={(percent) => `${percent?.toFixed(1)}%`}
          />
          <Tag color={getSuccessRateColor(rate)} style={{ marginTop: 4 }}>
            {rate >= 99 ? '优秀' : rate >= 95 ? '良好' : '需要关注'}
          </Tag>
        </div>
      ),
    },
    {
      title: '响应时间',
      key: 'response_time',
      width: 120,
      sorter: (a, b) => a.avg_response_time - b.avg_response_time,
      render: (_, record: ApplicationAnalytics) => {
        const status = getResponseTimeStatus(record.avg_response_time);
        return (
          <div>
            <div style={{ fontWeight: 500, marginBottom: 4 }}>
              {formatDuration(record.avg_response_time)}
            </div>
            <Tag color={status.status === 'success' ? 'green' : status.status === 'normal' ? 'orange' : 'red'}>
              {status.text}
            </Tag>
            <div style={{ fontSize: 12, color: '#666', marginTop: 4 }}>
              P95: {formatDuration(record.p95_response_time)}
            </div>
          </div>
        );
      },
    },
    {
      title: '数据传输',
      dataIndex: 'data_transferred',
      key: 'data_transferred',
      width: 120,
      sorter: (a, b) => a.data_transferred - b.data_transferred,
      render: (bytes: number) => (
        <div>
          <CloudUploadOutlined style={{ color: '#722ed1', marginRight: 4 }} />
          <span style={{ fontWeight: 500 }}>{formatBytes(bytes)}</span>
        </div>
      ),
    },
    {
      title: '热门端点',
      key: 'top_endpoints',
      width: 250,
      render: (_, record: ApplicationAnalytics) => (
        <div>
          {record.top_endpoints.slice(0, 3).map((endpoint, index) => (
            <div key={index} style={{ marginBottom: 4 }}>
              <Space size={4}>
                <Tag color="blue" style={{ fontSize: 10, padding: '0 4px' }}>
                  {endpoint.method}
                </Tag>
                <Text style={{ fontSize: 12 }} ellipsis={{ tooltip: endpoint.path }}>
                  {endpoint.path.length > 20 ? endpoint.path.substring(0, 20) + '...' : endpoint.path}
                </Text>
                <Text type="secondary" style={{ fontSize: 11 }}>
                  ({formatNumber(endpoint.request_count)})
                </Text>
              </Space>
            </div>
          ))}
          {record.top_endpoints.length > 3 && (
            <Text type="secondary" style={{ fontSize: 11 }}>
              +{record.top_endpoints.length - 3} 更多
            </Text>
          )}
        </div>
      ),
    },
    {
      title: '错误分析',
      key: 'error_breakdown',
      width: 150,
      render: (_, record: ApplicationAnalytics) => {
        if (!record.error_breakdown || record.error_breakdown.length === 0) {
          return <Text type="secondary">无错误</Text>;
        }
        
        return (
          <div>
            {record.error_breakdown.slice(0, 2).map((error, index) => (
              <div key={index} style={{ marginBottom: 4 }}>
                <Space size={4}>
                  <Tag color="red" style={{ fontSize: 10 }}>
                    {error.status_code}
                  </Tag>
                  <Text style={{ fontSize: 12 }}>
                    {formatNumber(error.count)} ({error.percentage.toFixed(1)}%)
                  </Text>
                </Space>
              </div>
            ))}
            {record.error_breakdown.length > 2 && (
              <Text type="secondary" style={{ fontSize: 11 }}>
                +{record.error_breakdown.length - 2} 更多
              </Text>
            )}
          </div>
        );
      },
    },
  ];

  return (
    <Card>
      <Title level={4}>应用分析</Title>
      <Table
        columns={columns}
        dataSource={applications}
        loading={loading}
        rowKey="application_id"
        pagination={{
          pageSize: 10,
          showSizeChanger: true,
          showQuickJumper: true,
          showTotal: (total, range) => `第 ${range[0]}-${range[1]} 条，共 ${total} 条`,
        }}
        scroll={{ x: 1200 }}
        size="middle"
        expandable={{
          expandedRowRender: (record) => (
            <div style={{ padding: '16px 0' }}>
              <Title level={5}>详细信息</Title>
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(300px, 1fr))', gap: 16 }}>
                <div>
                  <Text strong>请求统计</Text>
                  <div style={{ marginTop: 8 }}>
                    <div>总请求数: {formatNumber(record.total_requests)}</div>
                    <div>成功请求: {formatNumber(record.successful_requests)}</div>
                    <div>失败请求: {formatNumber(record.failed_requests)}</div>
                    <div>成功率: {record.success_rate.toFixed(2)}%</div>
                  </div>
                </div>
                <div>
                  <Text strong>性能指标</Text>
                  <div style={{ marginTop: 8 }}>
                    <div>平均响应时间: {formatDuration(record.avg_response_time)}</div>
                    <div>P95响应时间: {formatDuration(record.p95_response_time)}</div>
                    <div>数据传输量: {formatBytes(record.data_transferred)}</div>
                  </div>
                </div>
                <div>
                  <Text strong>端点统计</Text>
                  <div style={{ marginTop: 8 }}>
                    {record.top_endpoints.map((endpoint, index) => (
                      <div key={index} style={{ marginBottom: 4 }}>
                        <Tag color="blue">{endpoint.method}</Tag>
                        {endpoint.path} ({formatNumber(endpoint.request_count)})
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            </div>
          ),
          rowExpandable: (record) => record.top_endpoints.length > 0,
        }}
      />
    </Card>
  );
};

export default ApplicationTable;
