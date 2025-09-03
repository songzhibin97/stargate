import React, { useState } from 'react';
import {
  Card,
  Button,
  Table,
  Space,
  Typography,
  Modal,
  Form,
  Input,
  message,
  Popconfirm,
  Tag,
  Tooltip,
  Row,
  Col,
  Statistic,
  Empty
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  CopyOutlined,
  ReloadOutlined,
  AppstoreOutlined,
  KeyOutlined,
  CalendarOutlined
} from '@ant-design/icons';
import { ColumnType } from 'antd/es/table';
import { Application, CreateApplicationRequest, UpdateApplicationRequest } from '../types';
import { 
  useApplications, 
  useCreateApplication, 
  useUpdateApplication, 
  useDeleteApplication,
  useRegenerateAPIKey 
} from '../hooks/useApplications';
import { validateApplicationName, validateApplicationDescription } from '../utils/validation';
import LoadingSpinner from '../components/common/LoadingSpinner';

const { Title, Text, Paragraph } = Typography;
const { TextArea } = Input;

const MyApplications: React.FC = () => {
  const [createModalVisible, setCreateModalVisible] = useState(false);
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [editingApp, setEditingApp] = useState<Application | null>(null);
  const [createForm] = Form.useForm();
  const [editForm] = Form.useForm();

  // API hooks
  const { data: applicationsData, isLoading, refetch } = useApplications();
  const createMutation = useCreateApplication();
  const updateMutation = useUpdateApplication();
  const deleteMutation = useDeleteApplication();
  const regenerateKeyMutation = useRegenerateAPIKey();

  const applications = applicationsData?.applications || [];
  const total = applicationsData?.total || 0;

  // Handle create application
  const handleCreate = async (values: CreateApplicationRequest) => {
    try {
      await createMutation.mutateAsync(values);
      setCreateModalVisible(false);
      createForm.resetFields();
      message.success('应用创建成功');
    } catch (error) {
      // Error is handled by the hook
    }
  };

  // Handle edit application
  const handleEdit = (app: Application) => {
    setEditingApp(app);
    editForm.setFieldsValue({
      name: app.name,
      description: app.description,
    });
    setEditModalVisible(true);
  };

  const handleUpdate = async (values: UpdateApplicationRequest) => {
    if (!editingApp) return;
    
    try {
      await updateMutation.mutateAsync({ id: editingApp.id, data: values });
      setEditModalVisible(false);
      setEditingApp(null);
      editForm.resetFields();
      message.success('应用更新成功');
    } catch (error) {
      // Error is handled by the hook
    }
  };

  // Handle delete application
  const handleDelete = async (id: string) => {
    try {
      await deleteMutation.mutateAsync(id);
      message.success('应用删除成功');
    } catch (error) {
      // Error is handled by the hook
    }
  };

  // Handle copy API key
  const handleCopyAPIKey = (apiKey: string) => {
    navigator.clipboard.writeText(apiKey).then(() => {
      message.success('API Key 已复制到剪贴板');
    }).catch(() => {
      message.error('复制失败，请手动复制');
    });
  };

  // Handle regenerate API key
  const handleRegenerateAPIKey = async (id: string) => {
    try {
      const result = await regenerateKeyMutation.mutateAsync(id);
      message.success('API Key 重新生成成功');
      // Show the new API key in a modal
      Modal.info({
        title: '新的 API Key',
        content: (
          <div>
            <Paragraph>
              您的新 API Key 已生成，请妥善保存：
            </Paragraph>
            <Paragraph copyable={{ text: result.api_key }}>
              <code>{result.api_key}</code>
            </Paragraph>
            <Paragraph type="warning">
              注意：出于安全考虑，此 API Key 只会显示一次，请立即保存。
            </Paragraph>
          </div>
        ),
        width: 600,
      });
    } catch (error) {
      // Error is handled by the hook
    }
  };

  // Table columns
  const columns: ColumnType<Application>[] = [
    {
      title: '应用名称',
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: Application) => (
        <Space direction="vertical" size={0}>
          <Text strong>{name}</Text>
          <Text type="secondary" style={{ fontSize: '12px' }}>
            ID: {record.id}
          </Text>
        </Space>
      ),
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
      render: (description: string) => (
        <Tooltip title={description}>
          <Text>{description}</Text>
        </Tooltip>
      ),
    },
    {
      title: 'API Key',
      dataIndex: 'api_key',
      key: 'api_key',
      render: (apiKey: string) => (
        <Space>
          <Text code style={{ fontSize: '12px' }}>
            {apiKey.substring(0, 8)}...{apiKey.substring(apiKey.length - 8)}
          </Text>
          <Button
            type="text"
            size="small"
            icon={<CopyOutlined />}
            onClick={() => handleCopyAPIKey(apiKey)}
            title="复制 API Key"
          />
        </Space>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => (
        <Tag color={status === 'active' ? 'green' : status === 'inactive' ? 'orange' : 'red'}>
          {status === 'active' ? '活跃' : status === 'inactive' ? '未激活' : '已暂停'}
        </Tag>
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (date: string) => new Date(date).toLocaleDateString('zh-CN'),
    },
    {
      title: '操作',
      key: 'actions',
      render: (_, record: Application) => (
        <Space>
          <Button
            type="text"
            size="small"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
            title="编辑应用"
          />
          <Button
            type="text"
            size="small"
            icon={<ReloadOutlined />}
            onClick={() => handleRegenerateAPIKey(record.id)}
            title="重新生成 API Key"
          />
          <Popconfirm
            title="确定要删除这个应用吗？"
            description="删除后无法恢复，请谨慎操作。"
            onConfirm={() => handleDelete(record.id)}
            okText="确定"
            cancelText="取消"
          >
            <Button
              type="text"
              size="small"
              icon={<DeleteOutlined />}
              danger
              title="删除应用"
            />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  if (isLoading) {
    return <LoadingSpinner tip="加载应用列表中..." />;
  }

  return (
    <div>
      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        <Col span={24}>
          <Card>
            <Row gutter={16}>
              <Col span={8}>
                <Statistic
                  title="总应用数"
                  value={total}
                  prefix={<AppstoreOutlined />}
                />
              </Col>
              <Col span={8}>
                <Statistic
                  title="活跃应用"
                  value={applications.filter(app => app.status === 'active').length}
                  prefix={<KeyOutlined />}
                />
              </Col>
              <Col span={8}>
                <Statistic
                  title="本月创建"
                  value={applications.filter(app => {
                    const created = new Date(app.created_at);
                    const now = new Date();
                    return created.getMonth() === now.getMonth() && created.getFullYear() === now.getFullYear();
                  }).length}
                  prefix={<CalendarOutlined />}
                />
              </Col>
            </Row>
          </Card>
        </Col>
      </Row>

      <Card
        title={
          <Space>
            <AppstoreOutlined />
            <Title level={4} style={{ margin: 0 }}>我的应用</Title>
          </Space>
        }
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} onClick={() => refetch()}>
              刷新
            </Button>
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => setCreateModalVisible(true)}
            >
              创建应用
            </Button>
          </Space>
        }
      >
        {applications.length === 0 ? (
          <Empty
            description="您还没有创建任何应用"
            image={Empty.PRESENTED_IMAGE_SIMPLE}
          >
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => setCreateModalVisible(true)}
            >
              创建第一个应用
            </Button>
          </Empty>
        ) : (
          <Table
            columns={columns}
            dataSource={applications}
            rowKey="id"
            pagination={{
              total,
              pageSize: 10,
              showSizeChanger: true,
              showQuickJumper: true,
              showTotal: (total, range) => `第 ${range[0]}-${range[1]} 条，共 ${total} 条`,
            }}
          />
        )}
      </Card>

      {/* Create Application Modal */}
      <Modal
        title="创建新应用"
        open={createModalVisible}
        onCancel={() => {
          setCreateModalVisible(false);
          createForm.resetFields();
        }}
        footer={null}
        width={600}
      >
        <Form
          form={createForm}
          layout="vertical"
          onFinish={handleCreate}
          disabled={createMutation.isLoading}
        >
          <Form.Item
            name="name"
            label="应用名称"
            rules={[
              { required: true, message: '请输入应用名称' },
              { validator: (_, value) => {
                const error = validateApplicationName(value);
                return error ? Promise.reject(error) : Promise.resolve();
              }}
            ]}
          >
            <Input placeholder="请输入应用名称" />
          </Form.Item>

          <Form.Item
            name="description"
            label="应用描述"
            rules={[
              { required: true, message: '请输入应用描述' },
              { validator: (_, value) => {
                const error = validateApplicationDescription(value);
                return error ? Promise.reject(error) : Promise.resolve();
              }}
            ]}
          >
            <TextArea
              rows={4}
              placeholder="请描述您的应用用途和功能"
              showCount
              maxLength={500}
            />
          </Form.Item>

          <Form.Item style={{ marginBottom: 0, textAlign: 'right' }}>
            <Space>
              <Button onClick={() => {
                setCreateModalVisible(false);
                createForm.resetFields();
              }}>
                取消
              </Button>
              <Button
                type="primary"
                htmlType="submit"
                loading={createMutation.isLoading}
              >
                创建应用
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      {/* Edit Application Modal */}
      <Modal
        title="编辑应用"
        open={editModalVisible}
        onCancel={() => {
          setEditModalVisible(false);
          setEditingApp(null);
          editForm.resetFields();
        }}
        footer={null}
        width={600}
      >
        <Form
          form={editForm}
          layout="vertical"
          onFinish={handleUpdate}
          disabled={updateMutation.isLoading}
        >
          <Form.Item
            name="name"
            label="应用名称"
            rules={[
              { required: true, message: '请输入应用名称' },
              { validator: (_, value) => {
                const error = validateApplicationName(value);
                return error ? Promise.reject(error) : Promise.resolve();
              }}
            ]}
          >
            <Input placeholder="请输入应用名称" />
          </Form.Item>

          <Form.Item
            name="description"
            label="应用描述"
            rules={[
              { required: true, message: '请输入应用描述' },
              { validator: (_, value) => {
                const error = validateApplicationDescription(value);
                return error ? Promise.reject(error) : Promise.resolve();
              }}
            ]}
          >
            <TextArea
              rows={4}
              placeholder="请描述您的应用用途和功能"
              showCount
              maxLength={500}
            />
          </Form.Item>

          <Form.Item style={{ marginBottom: 0, textAlign: 'right' }}>
            <Space>
              <Button onClick={() => {
                setEditModalVisible(false);
                setEditingApp(null);
                editForm.resetFields();
              }}>
                取消
              </Button>
              <Button
                type="primary"
                htmlType="submit"
                loading={updateMutation.isLoading}
              >
                更新应用
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default MyApplications;
