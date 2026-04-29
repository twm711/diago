import { Alert, Badge, Button, Card, Col, Divider, Layout, List, Progress, Row, Space, Statistic, Tag, Timeline, Typography } from 'antd'
import { AudioOutlined, CloudServerOutlined, FieldTimeOutlined, PhoneOutlined, PlayCircleOutlined, RobotOutlined, ThunderboltOutlined, WechatOutlined } from '@ant-design/icons'

const { Header, Content } = Layout
const { Title, Paragraph, Text } = Typography

const callEvents = [
  { time: '09:02:18', title: '外呼任务进入智能路由', desc: '按技能组与优先级匹配坐席，触发 AI 预检。' },
  { time: '09:02:21', title: 'WebRTC 网关建立媒体通道', desc: '浏览器端音视频通过 Pion 网关接入。' },
  { time: '09:02:25', title: 'ASR 开始流式转写', desc: '实时识别用户首句，进入意图分析。' },
  { time: '09:02:29', title: 'LLM 触发应答策略', desc: '对话打断、问答澄清与话术推荐完成。' },
]

const pipeline = [
  'SIP/呼叫控制',
  'WebRTC 网关',
  'ASR / TTS',
  'LLM 对话策略',
  'ACD 路由',
  '录音质检',
]

const metrics = [
  { title: '在线通话', value: 1286, suffix: '路', icon: <PhoneOutlined /> },
  { title: '平均接通率', value: 96.4, suffix: '%', icon: <ThunderboltOutlined /> },
  { title: '实时转写延迟', value: 280, suffix: 'ms', icon: <FieldTimeOutlined /> },
  { title: 'AI 接管率', value: 73.2, suffix: '%', icon: <RobotOutlined /> },
]

function App() {
  return (
    <Layout className="app-shell">
      <Header className="topbar">
        <div className="brand">
          <span className="brand-mark" />
          <div>
            <div className="brand-title">AICC Platform</div>
            <div className="brand-subtitle">Go + Pion WebRTC + diago</div>
          </div>
        </div>
        <Space size={12} wrap>
          <Tag color="volcano">AI 呼叫中心</Tag>
          <Tag color="gold">MySQL</Tag>
          <Tag color="blue">WebRTC Gateway</Tag>
        </Space>
      </Header>

      <Content className="page">
        <section className="hero-panel">
          <Row gutter={[24, 24]} align="middle">
            <Col xs={24} lg={14}>
              <Space direction="vertical" size={18} className="hero-copy">
                <Badge status="processing" text="系统运行中" />
                <Title level={1} className="hero-title">
                  面向企业级 AI 呼叫中心的控制台与实时通信底座
                </Title>
                <Paragraph className="hero-text">
                  以 diago 作为 SIP/RTP 内核，前端用 React + Ant Design，后端用 Go，WebRTC 通过 Pion 接入。
                  先构建呼叫执行、AI 对话和坐席管理的最小闭环，再逐步扩展到多租户、质检、报表和运营。
                </Paragraph>
                <Space wrap>
                  <Button type="primary" size="large" icon={<PlayCircleOutlined />}>
                    进入实时话务台
                  </Button>
                  <Button size="large" icon={<CloudServerOutlined />}>
                    查看服务拓扑
                  </Button>
                </Space>
              </Space>
            </Col>
            <Col xs={24} lg={10}>
              <Card className="hero-card" bordered={false}>
                <Space direction="vertical" size={16} style={{ width: '100%' }}>
                  <Text className="section-label">核心能力图</Text>
                  <div className="flow-list">
                    {pipeline.map((step, index) => (
                      <div className="flow-item" key={step}>
                        <span className="flow-index">0{index + 1}</span>
                        <span>{step}</span>
                      </div>
                    ))}
                  </div>
                </Space>
              </Card>
            </Col>
          </Row>
        </section>

        <section className="metrics-grid">
          <Row gutter={[16, 16]}>
            {metrics.map((item) => (
              <Col xs={24} sm={12} xl={6} key={item.title}>
                <Card bordered={false} className="metric-card">
                  <Space align="start">
                    <div className="metric-icon">{item.icon}</div>
                    <div>
                      <Text className="metric-label">{item.title}</Text>
                      <div className="metric-value">
                        {item.value}
                        <span>{item.suffix}</span>
                      </div>
                    </div>
                  </Space>
                </Card>
              </Col>
            ))}
          </Row>
        </section>

        <section className="content-grid">
          <Row gutter={[24, 24]}>
            <Col xs={24} lg={14}>
              <Card title="实时呼叫事件" bordered={false} className="panel-card">
                <Timeline
                  items={callEvents.map((event) => ({
                    children: (
                      <div>
                        <div className="timeline-time">{event.time}</div>
                        <div className="timeline-title">{event.title}</div>
                        <div className="timeline-desc">{event.desc}</div>
                      </div>
                    ),
                  }))}
                />
              </Card>
            </Col>
            <Col xs={24} lg={10}>
              <Card title="AI 呼叫流程" bordered={false} className="panel-card">
                <Space direction="vertical" size={16} style={{ width: '100%' }}>
                  <Alert
                    type="info"
                    showIcon
                    message="当前策略"
                    description="呼入先走意图识别与分流，复杂问题可自动升级到人工坐席。"
                  />
                  <div className="workflow">
                    <div className="workflow-item"><PhoneOutlined /> 呼入接通</div>
                    <div className="workflow-item"><AudioOutlined /> 实时转写</div>
                    <div className="workflow-item"><RobotOutlined /> 意图识别</div>
                    <div className="workflow-item"><WechatOutlined /> 坐席接管</div>
                  </div>
                  <Divider />
                  <Space direction="vertical" size={10} style={{ width: '100%' }}>
                    <div className="progress-row">
                      <Text>语音转写覆盖率</Text>
                      <Text>92%</Text>
                    </div>
                    <Progress percent={92} strokeColor="#c96442" trailColor="#f0eee6" />
                    <div className="progress-row">
                      <Text>AI 摘要完成率</Text>
                      <Text>78%</Text>
                    </div>
                    <Progress percent={78} strokeColor="#141413" trailColor="#f0eee6" />
                  </Space>
                </Space>
              </Card>
            </Col>
          </Row>
        </section>

        <section className="content-grid bottom-grid">
          <Row gutter={[24, 24]}>
            <Col xs={24} lg={12}>
              <Card title="平台待办" bordered={false} className="panel-card">
                <List
                  itemLayout="horizontal"
                  dataSource={[
                    '接入 MySQL 会话表与租户表',
                    '实现 WebRTC Offer/Answer 会话保存',
                    '接入 ASR/TTS Provider 抽象层',
                    '补充坐席工作台与实时大屏',
                  ]}
                  renderItem={(item) => <List.Item>{item}</List.Item>}
                />
              </Card>
            </Col>
            <Col xs={24} lg={12}>
              <Card title="系统状态" bordered={false} className="panel-card">
                <Space direction="vertical" size={18} style={{ width: '100%' }}>
                  <div className="status-line">
                    <Text>Go API</Text>
                    <Tag color="green">Ready</Tag>
                  </div>
                  <div className="status-line">
                    <Text>MySQL</Text>
                    <Tag color="gold">Pending</Tag>
                  </div>
                  <div className="status-line">
                    <Text>Pion WebRTC</Text>
                    <Tag color="blue">Ready</Tag>
                  </div>
                  <div className="status-line">
                    <Text>AI Gateway</Text>
                    <Tag color="volcano">Stub</Tag>
                  </div>
                </Space>
              </Card>
            </Col>
          </Row>
        </section>
      </Content>
    </Layout>
  )
}

export default App
