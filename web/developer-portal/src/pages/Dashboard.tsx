import React, { useEffect } from 'react';
import { Layout, Row, Col, Alert, Typography, Space, Spin, Button, Card } from 'antd';
import {
  DashboardOutlined,
  ExclamationCircleOutlined,
  CheckCircleOutlined,
} from '@ant-design/icons';
import { useDashboardStore, useAutoRefresh } from '../stores/dashboardStore';
import DashboardControls from '../components/dashboard/DashboardControls';
import MetricsCard from '../components/dashboard/MetricsCard';
import TimeSeriesChart from '../components/dashboard/TimeSeriesChart';
import ResponseTimeChart from '../components/dashboard/ResponseTimeChart';
import ApplicationTable from '../components/dashboard/ApplicationTable';

const { Content } = Layout;
const { Title, Text } = Typography;

const Dashboard: React.FC = () => {
  const {
    analyticsData,
    loading,
    error,
    lastUpdated,
    refreshData,
    clearError,
  } = useDashboardStore();

  // TODO: Integrate with actual application store when available
  const applications: any[] = [];
  const { startAutoRefresh, stopAutoRefresh } = useAutoRefresh();

  // Initialize data on component mount
  useEffect(() => {
    const initializeDashboard = async () => {
      await refreshData();
    };

    initializeDashboard();
  }, []);

  // Setup auto-refresh
  useEffect(() => {
    startAutoRefresh();
    return () => stopAutoRefresh();
  }, [startAutoRefresh, stopAutoRefresh]);

  const handleRefresh = async () => {
    await refreshData();
  };

  const renderHeader = () => (
    <div style={{ marginBottom: 24 }}>
      <Space align="center" style={{ marginBottom: 16 }}>
        <DashboardOutlined style={{ fontSize: 24, color: '#1890ff' }} />
        <Title level={2} style={{ margin: 0 }}>
          API 使用分析仪表盘
        </Title>
      </Space>

      <Row justify="space-between" align="middle">
        <Col>
          <Text type="secondary">
            实时监控您的API使用情况、性能指标和应用分析
          </Text>
        </Col>
        <Col>
          {lastUpdated && (
            <Text type="secondary" style={{ fontSize: 12 }}>
              最后更新: {lastUpdated.toLocaleString('zh-CN')}
            </Text>
          )}
        </Col>
      </Row>
    </div>
  );

  const renderError = () => {
    if (!error) return null;

    return (
      <Alert
        message="数据加载失败"
        description={error}
        type="error"
        showIcon
        icon={<ExclamationCircleOutlined />}
        action={
          <Space>
            <Button size="small" onClick={clearError}>
              忽略
            </Button>
            <Button size="small" type="primary" onClick={handleRefresh}>
              重试
            </Button>
          </Space>
        }
        closable
        onClose={clearError}
        style={{ marginBottom: 16 }}
      />
    );
  };

  const renderLoadingState = () => {
    if (!loading || analyticsData) return null;

    return (
      <div style={{
        textAlign: 'center',
        padding: '100px 0',
        backgroundColor: '#fafafa',
        borderRadius: '8px',
        margin: '24px 0'
      }}>
        <Spin size="large" />
        <div style={{ marginTop: 16 }}>
          <Text>正在加载分析数据...</Text>
        </div>
      </div>
    );
  };

  const renderEmptyState = () => {
    if (loading || analyticsData || error) return null;

    return (
      <div style={{
        textAlign: 'center',
        padding: '100px 0',
        backgroundColor: '#fafafa',
        borderRadius: '8px',
        margin: '24px 0'
      }}>
        <ExclamationCircleOutlined style={{ fontSize: 48, color: '#d9d9d9' }} />
        <div style={{ marginTop: 16 }}>
          <Title level={4} type="secondary">暂无数据</Title>
          <Text type="secondary">
            请确保您有活跃的应用并且已经产生了API请求
          </Text>
        </div>
        <Button type="primary" onClick={handleRefresh} style={{ marginTop: 16 }}>
          重新加载
        </Button>
      </div>
    );
  };

  const renderDashboardContent = () => {
    if (!analyticsData) return null;

    return (
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        {/* 概览指标卡片 */}
        <MetricsCard summary={analyticsData.summary} loading={loading} />

        {/* 图表区域 */}
        <Row gutter={[16, 16]}>
          <Col xs={24} lg={12}>
            <TimeSeriesChart
              data={analyticsData}
              loading={loading}
              title="请求趋势"
              type="area"
              height={350}
            />
          </Col>
          <Col xs={24} lg={12}>
            <ResponseTimeChart
              data={analyticsData}
              loading={loading}
              height={350}
            />
          </Col>
        </Row>

        {/* 应用分析表格 */}
        <ApplicationTable
          applications={analyticsData.applications}
          loading={loading}
        />

        {/* 数据元信息 */}
        <Card size="small" style={{ backgroundColor: '#f6f6f6' }}>
          <Row gutter={16}>
            <Col span={6}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                数据时间范围: {new Date(analyticsData.metadata.start_time).toLocaleString('zh-CN')} - {new Date(analyticsData.metadata.end_time).toLocaleString('zh-CN')}
              </Text>
            </Col>
            <Col span={6}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                数据粒度: {analyticsData.metadata.granularity}
              </Text>
            </Col>
            <Col span={6}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                应用数量: {analyticsData.metadata.application_count}
              </Text>
            </Col>
            <Col span={6}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                查询耗时: {analyticsData.metadata.query_duration}ms
              </Text>
            </Col>
          </Row>
        </Card>
      </Space>
    );
  };

  return (
    <Layout>
      <Content style={{ padding: '24px' }}>
        {renderHeader()}

        {/* 控制面板 */}
        <DashboardControls
          applications={applications}
          onRefresh={handleRefresh}
          loading={loading}
        />

        {/* 错误提示 */}
        {renderError()}

        {/* 加载状态 */}
        {renderLoadingState()}

        {/* 空状态 */}
        {renderEmptyState()}

        {/* 仪表盘内容 */}
        {renderDashboardContent()}

        {/* 成功状态指示 */}
        {analyticsData && !loading && !error && (
          <div style={{
            position: 'fixed',
            bottom: 24,
            right: 24,
            zIndex: 1000,
            backgroundColor: '#f6ffed',
            border: '1px solid #b7eb8f',
            borderRadius: '6px',
            padding: '8px 12px',
            fontSize: 12,
            color: '#52c41a'
          }}>
            <CheckCircleOutlined style={{ marginRight: 4 }} />
            数据已更新
          </div>
        )}
      </Content>
    </Layout>
  );
};

export default Dashboard;