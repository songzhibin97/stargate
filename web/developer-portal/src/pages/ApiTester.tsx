import React from "react";
import { Card, Typography, Result, Button } from "antd";
import { RocketOutlined } from "@ant-design/icons";
import { Link } from "react-router-dom";

const { Title } = Typography;

const ApiTester: React.FC = () => {
  return (
    <div>
      <Card
        title={
          <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
            <RocketOutlined />
            <Title level={4} style={{ margin: 0 }}>
              API 测试器
            </Title>
          </div>
        }
      >
        <Result
          icon={<RocketOutlined style={{ color: "#722ed1" }} />}
          title="API 测试器"
          subTitle="在线测试和调试API接口"
          extra={
            <div>
              <Button type="primary" onClick={() => window.open("/docs", "_blank")}>
                查看API文档
              </Button>
              <Button style={{ marginLeft: 8 }}>
                <Link to="/apis">浏览API</Link>
              </Button>
            </div>
          }
        />
      </Card>
    </div>
  );
};

export default ApiTester;
