import React, { useState } from 'react';
import {
  Card,
  Form,
  Input,
  Button,
  Space,
  Typography,
  Avatar,
  Row,
  Col,
  Divider,
  message,
  Modal
} from 'antd';
import {
  UserOutlined,
  MailOutlined,
  EditOutlined,
  SaveOutlined,
  LockOutlined
} from '@ant-design/icons';
import { useAuthStore } from '../stores/authStore';
import { validateName, validateEmail } from '../utils/validation';

const { Title, Text } = Typography;

const Profile: React.FC = () => {
  const { user } = useAuthStore();
  const [editing, setEditing] = useState(false);
  const [loading, setLoading] = useState(false);
  const [form] = Form.useForm();

  if (!user) {
    return null;
  }

  const handleEdit = () => {
    form.setFieldsValue({
      name: user.name,
      email: user.email,
    });
    setEditing(true);
  };

  const handleCancel = () => {
    setEditing(false);
    form.resetFields();
  };

  const handleSave = async (_values: { name: string; email: string }) => {
    setLoading(true);
    try {
      // TODO: Implement profile update API
      message.success('个人资料更新成功');
      setEditing(false);
    } catch (error: any) {
      message.error(error.message || '更新失败');
    } finally {
      setLoading(false);
    }
  };

  const handleChangePassword = () => {
    Modal.info({
      title: '修改密码',
      content: '密码修改功能正在开发中，敬请期待。',
    });
  };

  return (
    <div>
      <Row gutter={[24, 24]}>
        <Col span={24}>
          <Card>
            <Space direction="vertical" size="large" style={{ width: '100%' }}>
              <div style={{ textAlign: 'center' }}>
                <Avatar
                  size={80}
                  icon={<UserOutlined />}
                  style={{ backgroundColor: '#1890ff', marginBottom: 16 }}
                />
                <Title level={3} style={{ margin: 0 }}>
                  {user.name}
                </Title>
                <Text type="secondary">{user.email}</Text>
              </div>

              <Divider />

              <div>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
                  <Title level={4} style={{ margin: 0 }}>
                    个人信息
                  </Title>
                  {!editing && (
                    <Button
                      type="primary"
                      icon={<EditOutlined />}
                      onClick={handleEdit}
                    >
                      编辑资料
                    </Button>
                  )}
                </div>

                {editing ? (
                  <Form
                    form={form}
                    layout="vertical"
                    onFinish={handleSave}
                    disabled={loading}
                  >
                    <Form.Item
                      name="name"
                      label="姓名"
                      rules={[
                        { required: true, message: '请输入姓名' },
                        { validator: (_, value) => {
                          const error = validateName(value);
                          return error ? Promise.reject(error) : Promise.resolve();
                        }}
                      ]}
                    >
                      <Input prefix={<UserOutlined />} placeholder="请输入姓名" />
                    </Form.Item>

                    <Form.Item
                      name="email"
                      label="邮箱地址"
                      rules={[
                        { required: true, message: '请输入邮箱地址' },
                        { validator: (_, value) => {
                          const error = validateEmail(value);
                          return error ? Promise.reject(error) : Promise.resolve();
                        }}
                      ]}
                    >
                      <Input prefix={<MailOutlined />} placeholder="请输入邮箱地址" />
                    </Form.Item>

                    <Form.Item>
                      <Space>
                        <Button
                          type="primary"
                          htmlType="submit"
                          icon={<SaveOutlined />}
                          loading={loading}
                        >
                          保存
                        </Button>
                        <Button onClick={handleCancel}>
                          取消
                        </Button>
                      </Space>
                    </Form.Item>
                  </Form>
                ) : (
                  <div>
                    <Row gutter={[16, 16]}>
                      <Col span={12}>
                        <Text strong>姓名：</Text>
                        <Text>{user.name}</Text>
                      </Col>
                      <Col span={12}>
                        <Text strong>邮箱：</Text>
                        <Text>{user.email}</Text>
                      </Col>
                      <Col span={12}>
                        <Text strong>角色：</Text>
                        <Text>{user.role === 'developer' ? '开发者' : user.role}</Text>
                      </Col>
                      <Col span={12}>
                        <Text strong>状态：</Text>
                        <Text>{user.status === 'active' ? '活跃' : user.status}</Text>
                      </Col>
                    </Row>
                  </div>
                )}
              </div>

              <Divider />

              <div>
                <Title level={4}>安全设置</Title>
                <Space direction="vertical" style={{ width: '100%' }}>
                  <Button
                    icon={<LockOutlined />}
                    onClick={handleChangePassword}
                  >
                    修改密码
                  </Button>
                </Space>
              </div>
            </Space>
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default Profile;
