import { LockOutlined, SafetyCertificateOutlined, UserOutlined } from '@ant-design/icons';
import { Alert, Button, Checkbox, Form, Input, Typography, message } from 'antd';
import { useState } from 'react';
import { Navigate, useLocation, useNavigate } from 'react-router-dom';
import { Brand } from '../components/Brand';
import { useAuth } from '../context/AuthContext';
import { ApiError } from '../lib/api';

interface LoginValues {
  account: string;
  password: string;
  remember?: boolean;
}

export function LoginPage() {
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');
  const { login, authenticated } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [messageApi, contextHolder] = message.useMessage();

  if (authenticated) return <Navigate to="/dashboard" replace />;

  const handleSubmit = async (values: LoginValues) => {
    setSubmitting(true);
    setError('');
    try {
      await login(values.account.trim(), values.password);
      messageApi.success('欢迎回来');
      const from = (location.state as { from?: string } | null)?.from || '/dashboard';
      navigate(from, { replace: true });
    } catch (requestError) {
      setError(requestError instanceof ApiError || requestError instanceof Error ? requestError.message : '登录失败');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="login-page">
      {contextHolder}
      <section className="login-showcase">
        <div className="login-showcase__glow" />
        <Brand />
        <div className="login-showcase__content">
          <span className="login-kicker">餐饮经营，从容一点</span>
          <h1>每一个小摊，<br />都值得一套好系统。</h1>
          <p>统一管理商户、门店和支付接入，让经营数据清楚可见，让服务可靠抵达。</p>
          <div className="login-feature">
            <SafetyCertificateOutlined />
            <div><strong>平台级安全管理</strong><small>权限隔离 · 操作审计 · 支付状态补偿</small></div>
          </div>
        </div>
        <footer>© {new Date().getFullYear()} 摊伴 Tanban · 系统管理中心</footer>
      </section>

      <section className="login-panel">
        <div className="login-card">
          <div className="login-card__mobile-brand"><Brand /></div>
          <Typography.Title level={2}>登录系统管理端</Typography.Title>
          <Typography.Paragraph type="secondary">请输入平台管理员账户，继续管理摊伴 SaaS。</Typography.Paragraph>
          {error && <Alert type="error" showIcon message={error} closable onClose={() => setError('')} />}
          <Form<LoginValues>
            layout="vertical"
            size="large"
            initialValues={{ remember: true }}
            onFinish={handleSubmit}
            requiredMark={false}
          >
            <Form.Item label="登录账号" name="account" rules={[{ required: true, message: '请输入登录账号' }]}>
              <Input prefix={<UserOutlined />} placeholder="手机号 / 用户名" autoComplete="username" />
            </Form.Item>
            <Form.Item label="登录密码" name="password" rules={[{ required: true, message: '请输入登录密码' }]}>
              <Input.Password prefix={<LockOutlined />} placeholder="请输入密码" autoComplete="current-password" />
            </Form.Item>
            <Form.Item name="remember" valuePropName="checked" className="login-remember">
              <Checkbox>保持登录状态</Checkbox>
            </Form.Item>
            <Button type="primary" htmlType="submit" block loading={submitting} className="login-submit">
              进入管理中心
            </Button>
          </Form>
          <p className="login-help">无法登录？请联系平台超级管理员重置账户。</p>
        </div>
      </section>
    </div>
  );
}
