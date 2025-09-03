import React from 'react';
import {
  Card,
  Row,
  Col,
  Select,
  Button,
  Switch,
  Space,
  Typography,
  Tooltip,
  DatePicker,
} from 'antd';
import {
  ReloadOutlined,
  SettingOutlined,
  CalendarOutlined,
  AppstoreOutlined,
  ClockCircleOutlined,
} from '@ant-design/icons';
import { useDashboardStore, TIME_RANGE_OPTIONS, GRANULARITY_OPTIONS, REFRESH_INTERVAL_OPTIONS } from '../../stores/dashboardStore';
import { Application } from '../../types';
import dayjs from 'dayjs';

const { Title, Text } = Typography;
const { RangePicker } = DatePicker;
const { Option } = Select;

interface DashboardControlsProps {
  applications: Application[];
  onRefresh: () => void;
  loading?: boolean;
}

const DashboardControls: React.FC<DashboardControlsProps> = ({
  applications,
  onRefresh,
  loading = false,
}) => {
  const {
    timeRange,
    selectedApplications,
    granularity,
    autoRefresh,
    refreshInterval,
    setTimeRange,
    setSelectedApplications,
    setGranularity,
    setAutoRefresh,
    setRefreshInterval,
  } = useDashboardStore();

  const [customTimeMode, setCustomTimeMode] = React.useState(false);
  const [customTimeRange, setCustomTimeRange] = React.useState<[dayjs.Dayjs, dayjs.Dayjs] | null>(null);

  const handleTimeRangeChange = (value: string) => {
    if (value === 'custom') {
      setCustomTimeMode(true);
    } else {
      setCustomTimeMode(false);
      setTimeRange(value);
    }
  };

  const handleCustomTimeRangeChange = (dates: any) => {
    setCustomTimeRange(dates);
    if (dates && dates.length === 2) {
      // Convert to API format and update store
      // This would require extending the store to handle custom time ranges
      console.log('Custom time range:', dates[0].toISOString(), dates[1].toISOString());
    }
  };

  const handleApplicationChange = (values: string[]) => {
    setSelectedApplications(values);
  };

  const handleGranularityChange = (value: string) => {
    setGranularity(value);
  };

  const handleAutoRefreshChange = (checked: boolean) => {
    setAutoRefresh(checked);
  };

  const handleRefreshIntervalChange = (value: number) => {
    setRefreshInterval(value);
  };

  return (
    <Card>
      <Row gutter={[16, 16]} align="middle">
        {/* 标题和刷新按钮 */}
        <Col xs={24} sm={12} md={8}>
          <Space align="center">
            <Title level={4} style={{ margin: 0 }}>
              仪表盘控制
            </Title>
            <Tooltip title="刷新数据">
              <Button
                type="primary"
                icon={<ReloadOutlined />}
                onClick={onRefresh}
                loading={loading}
                size="small"
              >
                刷新
              </Button>
            </Tooltip>
          </Space>
        </Col>

        {/* 时间范围选择 */}
        <Col xs={24} sm={12} md={4}>
          <div>
            <Text strong style={{ fontSize: 12, color: '#666' }}>
              <CalendarOutlined style={{ marginRight: 4 }} />
              时间范围
            </Text>
            <Select
              value={customTimeMode ? 'custom' : timeRange}
              onChange={handleTimeRangeChange}
              style={{ width: '100%', marginTop: 4 }}
              size="small"
            >
              {TIME_RANGE_OPTIONS.map(option => (
                <Option key={option.value} value={option.value}>
                  {option.label}
                </Option>
              ))}
              <Option value="custom">自定义</Option>
            </Select>
            {customTimeMode && (
              <RangePicker
                value={customTimeRange}
                onChange={handleCustomTimeRangeChange}
                showTime
                style={{ width: '100%', marginTop: 8 }}
                size="small"
                placeholder={['开始时间', '结束时间']}
              />
            )}
          </div>
        </Col>

        {/* 应用筛选 */}
        <Col xs={24} sm={12} md={4}>
          <div>
            <Text strong style={{ fontSize: 12, color: '#666' }}>
              <AppstoreOutlined style={{ marginRight: 4 }} />
              应用筛选
            </Text>
            <Select
              mode="multiple"
              value={selectedApplications}
              onChange={handleApplicationChange}
              placeholder="选择应用"
              style={{ width: '100%', marginTop: 4 }}
              size="small"
              maxTagCount={2}
              maxTagTextLength={10}
              allowClear
            >
              {applications.map(app => (
                <Option key={app.id} value={app.id}>
                  {app.name}
                </Option>
              ))}
            </Select>
          </div>
        </Col>

        {/* 数据粒度 */}
        <Col xs={24} sm={12} md={3}>
          <div>
            <Text strong style={{ fontSize: 12, color: '#666' }}>
              <SettingOutlined style={{ marginRight: 4 }} />
              数据粒度
            </Text>
            <Select
              value={granularity}
              onChange={handleGranularityChange}
              style={{ width: '100%', marginTop: 4 }}
              size="small"
            >
              {GRANULARITY_OPTIONS.map(option => (
                <Option key={option.value} value={option.value}>
                  {option.label}
                </Option>
              ))}
            </Select>
          </div>
        </Col>

        {/* 自动刷新设置 */}
        <Col xs={24} sm={12} md={5}>
          <div>
            <Text strong style={{ fontSize: 12, color: '#666' }}>
              <ClockCircleOutlined style={{ marginRight: 4 }} />
              自动刷新
            </Text>
            <div style={{ marginTop: 4 }}>
              <Space>
                <Switch
                  checked={autoRefresh}
                  onChange={handleAutoRefreshChange}
                  size="small"
                />
                <Select
                  value={refreshInterval}
                  onChange={handleRefreshIntervalChange}
                  disabled={!autoRefresh}
                  style={{ width: 80 }}
                  size="small"
                >
                  {REFRESH_INTERVAL_OPTIONS.map(option => (
                    <Option key={option.value} value={option.value}>
                      {option.label}
                    </Option>
                  ))}
                </Select>
              </Space>
            </div>
          </div>
        </Col>
      </Row>

      {/* 当前选择状态显示 */}
      <Row style={{ marginTop: 16, padding: '12px', backgroundColor: '#fafafa', borderRadius: '6px' }}>
        <Col span={24}>
          <Space wrap>
            <Text type="secondary" style={{ fontSize: 12 }}>
              当前设置:
            </Text>
            <Text style={{ fontSize: 12 }}>
              时间范围: <Text code>{customTimeMode ? '自定义' : TIME_RANGE_OPTIONS.find(opt => opt.value === timeRange)?.label}</Text>
            </Text>
            <Text style={{ fontSize: 12 }}>
              应用: <Text code>
                {selectedApplications.length === 0 ? '全部' : `${selectedApplications.length}个应用`}
              </Text>
            </Text>
            <Text style={{ fontSize: 12 }}>
              粒度: <Text code>{GRANULARITY_OPTIONS.find(opt => opt.value === granularity)?.label}</Text>
            </Text>
            <Text style={{ fontSize: 12 }}>
              自动刷新: <Text code>
                {autoRefresh ? REFRESH_INTERVAL_OPTIONS.find(opt => opt.value === refreshInterval)?.label : '关闭'}
              </Text>
            </Text>
          </Space>
        </Col>
      </Row>
    </Card>
  );
};

export default DashboardControls;
