import { App as AntApp, ConfigProvider, Result, Spin } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import type { ReactElement } from 'react';
import { Navigate, Route, Routes, useLocation } from 'react-router-dom';
import { AuthProvider, useAuth } from './auth/AuthContext';
import { canManageMerchant } from './auth/permissions';
import { AppLayout } from './components/AppLayout';
import { CatalogConfigPage } from './pages/CatalogConfigPage';
import { CustomersPage } from './pages/CustomersPage';
import { DashboardPage } from './pages/DashboardPage';
import { DecorationPage } from './pages/DecorationPage';
import { LoginPage } from './pages/LoginPage';
import { MembershipPage } from './pages/MembershipPage';
import { OrdersPage } from './pages/OrdersPage';
import { PaymentsPage } from './pages/PaymentsPage';
import { PrintersPage } from './pages/PrintersPage';
import { ProductsPage } from './pages/ProductsPage';
import { SettingsPage } from './pages/SettingsPage';
import { StaffPage } from './pages/StaffPage';
import { StoredValuePage } from './pages/StoredValuePage';
import { TableCodesPage } from './pages/TableCodesPage';
import { BusinessPrintTemplatePage } from './pages/BusinessPrintTemplatePage';
import { FastFoodPlatesPage } from './pages/FastFoodPlatesPage';
import { CouponsPage } from './pages/CouponsPage';
import { LotteryPage } from './pages/LotteryPage';
import { MarketingAppsPage } from './pages/MarketingAppsPage';
import { PopupAdsPage } from './pages/PopupAdsPage';
import { MediaLibraryPage } from './pages/MediaLibraryPage';

function ProtectedLayout() {
  const { user, loading } = useAuth();
  const location = useLocation();
  if (loading) return <div className="app-loading"><Spin size="large" /><span>正在进入摊伴工作台...</span></div>;
  if (!user) return <Navigate to="/login" state={{ from: location.pathname }} replace />;
  return <AppLayout />;
}

function ManagementOnly({ children }: { children: ReactElement }) {
  const { user } = useAuth();
  return canManageMerchant(user) ? children : <Navigate to="/dashboard" replace />;
}

function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route element={<ProtectedLayout />}>
        <Route path="/dashboard" element={<DashboardPage />} />
        <Route path="/orders" element={<Navigate to="/dine-in/orders" replace />} />
        <Route path="/dine-in/orders" element={<OrdersPage businessType="DINE_IN" sceneMode="DINE_IN" />} />
        <Route path="/dine-in/fast-food-orders" element={<OrdersPage businessType="DINE_IN" sceneMode="TAKEOUT" />} />
        <Route path="/dine-in/table-codes" element={<ManagementOnly><TableCodesPage /></ManagementOnly>} />
        <Route path="/dine-in/fast-food-plates" element={<ManagementOnly><FastFoodPlatesPage /></ManagementOnly>} />
        <Route path="/dine-in/print-template" element={<ManagementOnly><BusinessPrintTemplatePage key="dine-in-print-template" businessType="DINE_IN" /></ManagementOnly>} />
        <Route path="/delivery/orders" element={<ManagementOnly><OrdersPage businessType="DELIVERY" unavailable /></ManagementOnly>} />
        <Route path="/delivery/print-template" element={<ManagementOnly><BusinessPrintTemplatePage key="delivery-print-template" businessType="DELIVERY" /></ManagementOnly>} />
        <Route path="/print-jobs" element={<PrintersPage jobsOnly />} />
        <Route path="/products" element={<ManagementOnly><ProductsPage /></ManagementOnly>} />
        <Route path="/catalog" element={<ManagementOnly><CatalogConfigPage /></ManagementOnly>} />
        <Route path="/media-library" element={<ManagementOnly><MediaLibraryPage /></ManagementOnly>} />
        <Route path="/customers" element={<ManagementOnly><CustomersPage /></ManagementOnly>} />
        <Route path="/membership" element={<ManagementOnly><MembershipPage /></ManagementOnly>} />
        <Route path="/stored-value" element={<ManagementOnly><StoredValuePage /></ManagementOnly>} />
        <Route path="/decoration" element={<ManagementOnly><DecorationPage /></ManagementOnly>} />
        <Route path="/marketing" element={<ManagementOnly><MarketingAppsPage /></ManagementOnly>} />
        <Route path="/marketing/coupons" element={<ManagementOnly><CouponsPage /></ManagementOnly>} />
        <Route path="/marketing/popup-ads" element={<ManagementOnly><PopupAdsPage /></ManagementOnly>} />
        <Route path="/marketing/lottery" element={<ManagementOnly><LotteryPage /></ManagementOnly>} />
        <Route path="/payments" element={<ManagementOnly><PaymentsPage /></ManagementOnly>} />
        <Route path="/printers" element={<ManagementOnly><PrintersPage /></ManagementOnly>} />
        <Route path="/staff" element={<ManagementOnly><StaffPage /></ManagementOnly>} />
        <Route path="/settings" element={<ManagementOnly><SettingsPage /></ManagementOnly>} />
      </Route>
      <Route path="/" element={<Navigate to="/dashboard" replace />} />
      <Route path="*" element={<Result status="404" title="页面不存在" subTitle="这个页面可能已经移动" extra={<a href="/dashboard">返回经营总览</a>} />} />
    </Routes>
  );
}

export default function App() {
  return (
    <ConfigProvider
      locale={zhCN}
      theme={{
        token: {
          colorPrimary: '#a5683f',
          colorInfo: '#a5683f',
          colorSuccess: '#4f8a63',
          colorWarning: '#d78b3e',
          colorError: '#c6564e',
          colorText: '#332c28',
          colorTextSecondary: '#7a716c',
          colorBgLayout: '#f5f3f1',
          borderRadius: 10,
          borderRadiusLG: 14,
          fontFamily: "Inter, 'PingFang SC', 'Microsoft YaHei', system-ui, sans-serif",
        },
        components: {
          Button: { controlHeight: 38, fontWeight: 500 },
          Card: { paddingLG: 22 },
          Table: { headerBg: '#faf8f6', headerColor: '#665b54' },
          Tabs: { itemSelectedColor: '#8b5635', inkBarColor: '#a5683f' },
        },
      }}
    >
      <AntApp>
        <AuthProvider><AppRoutes /></AuthProvider>
      </AntApp>
    </ConfigProvider>
  );
}
