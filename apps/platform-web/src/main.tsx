import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { App as AntdApp, ConfigProvider } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import dayjs from 'dayjs';
import 'dayjs/locale/zh-cn';
import { AuthProvider } from './context/AuthContext';
import App from './App';
import './styles.css';

dayjs.locale('zh-cn');

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ConfigProvider
      locale={zhCN}
      theme={{
        token: {
          colorPrimary: '#f56a1d',
          colorInfo: '#f56a1d',
          colorSuccess: '#2b9a66',
          colorWarning: '#e7a33e',
          colorError: '#df4b4b',
          borderRadius: 10,
          borderRadiusLG: 16,
          fontFamily: "Inter, 'PingFang SC', 'Microsoft YaHei', system-ui, sans-serif",
          colorBgLayout: '#f5f5f3',
        },
        components: {
          Button: { controlHeightLG: 46, fontWeight: 600 },
          Card: { headerFontSize: 16 },
          Table: { headerBg: '#faf9f7', headerColor: '#57534e' },
          Menu: { itemBorderRadius: 8 },
        },
      }}
    >
      <AntdApp>
        <BrowserRouter>
          <AuthProvider><App /></AuthProvider>
        </BrowserRouter>
      </AntdApp>
    </ConfigProvider>
  </StrictMode>,
);
