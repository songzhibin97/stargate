import React from 'react';
import { Card, Typography, Result, Button } from 'antd';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeftOutlined } from '@ant-design/icons';

const { Title } = Typography;

const ApiDetail: React.FC = () => {
  const { apiId } = useParams<{ apiId: string }>();
  const navigate = useNavigate();

  return (
    <div>
      <Card
        title={
          <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
            <Button
              type="text"
              icon={<ArrowLeftOutlined />}
              onClick={() => navigate('/apis')}
            >
              返回
            </Button>
            <Title level={4} style={{ margin: 0 }}>
              API 详情
            </Title>
          </div>
        }
      >
        <Result
          title="API 详情页面"
          subTitle={`API ID: ${apiId}`}
          extra={
            <Button type="primary" onClick={() => navigate('/apis')}>
              返回 API 列表
            </Button>
          }
        />
      </Card>
    </div>
  );
};

export default ApiDetail;
