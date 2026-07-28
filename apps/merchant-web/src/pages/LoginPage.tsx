/* eslint-disable @next/next/no-img-element -- this Vite app imports a fingerprinted local brand asset */
import { CoffeeOutlined, LockOutlined, MobileOutlined, SafetyCertificateOutlined } from '@ant-design/icons';
import { Alert, Button, Checkbox, Form, Input, Space, Typography } from 'antd';
import { useState } from 'react';
import { Navigate, useLocation, useNavigate } from 'react-router-dom';
import tanbanIcon from '../assets/brand/tanban-icon.png';
import { useAuth } from '../auth/AuthContext';
import { errorMessage } from '../api/client';

interface LoginValues {
  account: string;
  password: string;
  remember: boolean;
}

export function LoginPage() {
  const { user, selectionRequired, login } = useAuth();
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');
  const navigate = useNavigate();
  const location = useLocation();
  if (user) return <Navigate to="/dashboard" replace />;
  if (selectionRequired) return <Navigate to="/select-store" replace />;

  const submit = async (values: LoginValues) => {
    setSubmitting(true);
    setError('');
    try {
      const result = await login(values.account.trim(), values.password);
      if (result === 'selection-required') {
        navigate('/select-store', { replace: true });
        return;
      }
      const from = (location.state as { from?: string } | null)?.from;
      navigate(from || '/dashboard', { replace: true });
    } catch (reason) {
      setError(errorMessage(reason));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="login-page">
      <div className="login-atmosphere">
        <div className="coffee-orbit orbit-one" />
        <div className="coffee-orbit orbit-two" />
        <div className="login-story">
          <div className="story-mark"><CoffeeOutlined /></div>
          <Typography.Title level={1}>让每一杯，都被认真交付。</Typography.Title>
          <Typography.Paragraph>
            从扫码点单、支付状态到出单提醒，摊伴把夜市小摊和独立门店的日常经营装进一块简单可靠的工作台。
          </Typography.Paragraph>
          <Space className="story-points" size={24} wrap>
            <span>实时订单</span><span>库存协同</span><span>可靠打印</span>
          </Space>
        </div>
      </div>
      <section className="login-panel">
        <div className="login-card">
          <div className="login-brand">
            <span className="login-brand-mark"><img src={tanbanIcon} alt="" /></span>
            <span><strong>摊伴</strong><small>TANBAN</small></span>
          </div>
          <Typography.Title level={2}>欢迎回来</Typography.Title>
          <Typography.Paragraph type="secondary">登录商户运营后台，开始今天的营业</Typography.Paragraph>
          {error && <Alert className="login-error" message={error} type="error" showIcon closable onClose={() => setError('')} />}
          <Form<LoginValues> layout="vertical" initialValues={{ remember: true }} onFinish={submit} requiredMark={false}>
            <Form.Item label="账号" name="account" rules={[{ required: true, message: '请输入手机号或登录账号' }]}>
              <Input size="large" prefix={<MobileOutlined />} placeholder="手机号 / 登录账号" autoComplete="username" />
            </Form.Item>
            <Form.Item label="密码" name="password" rules={[{ required: true, message: '请输入登录密码' }]}>
              <Input.Password size="large" prefix={<LockOutlined />} placeholder="请输入密码" autoComplete="current-password" />
            </Form.Item>
            <Form.Item name="remember" valuePropName="checked">
              <Checkbox>保持登录</Checkbox>
            </Form.Item>
            <Button block size="large" type="primary" htmlType="submit" loading={submitting}>进入工作台</Button>
          </Form>
          <div className="login-security"><SafetyCertificateOutlined /> 账号信息已加密传输</div>
        </div>
        <footer>© 2026 摊伴 · 小生意，也值得好系统</footer>
      </section>
    </div>
  );
}
