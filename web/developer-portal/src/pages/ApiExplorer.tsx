import React from "react";
import { Card, Typography, Result, Button } from "antd";
import { ApiOutlined } from "@ant-design/icons";
import { Link } from "react-router-dom";

const { Title } = Typography;

const ApiExplorer: React.FC = () => {
  return (
    <div>
      <Card
        title={
          <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
            <ApiOutlined />
            <Title level={4} style={{ margin: 0 }}>
              API 浏览器
            </Title>
          </div>
        }
      >
        <Result
          icon={<ApiOutlined style={{ color: "#1890ff" }} />}
          title="API 浏览器"
          subTitle="探索和发现可用的API接口"
          extra={
            <div>
              <Button type="primary" onClick={() => window.open("/docs", "_blank")}>
                查看API文档
              </Button>
              <Button style={{ marginLeft: 8 }}>
                <Link to="/tester">API 测试器</Link>
              </Button>
            </div>
          }
        />
      </Card>
    </div>
  );
};

export default ApiExplorer;
