import { Spin } from 'antd';
import { lazy, Suspense } from 'react';
import type { ReactNode } from 'react';
import { Navigate, Route, Routes, useLocation } from 'react-router-dom';
import { useAuth } from './context/AuthContext';
import { canManagePlatformUsers } from './lib/permissions';
import { LoginPage } from './pages/LoginPage';

const AppShell = lazy(() => import('./layouts/AppShell').then((module) => ({ default: module.AppShell })));
const DashboardPage = lazy(() => import('./pages/DashboardPage').then((module) => ({ default: module.DashboardPage })));
const UsersPage = lazy(() => import('./pages/UsersPage').then((module) => ({ default: module.UsersPage })));
const TenantsPage = lazy(() => import('./pages/TenantsPage').then((module) => ({ default: module.TenantsPage })));
const StoresPage = lazy(() => import('./pages/StoresPage').then((module) => ({ default: module.StoresPage })));
const PaymentSettingsPage = lazy(() => import('./pages/PaymentSettingsPage').then((module) => ({ default: module.PaymentSettingsPage })));
const SystemSettingsPage = lazy(() => import('./pages/SystemSettingsPage').then((module) => ({ default: module.SystemSettingsPage })));
const PrinterProvidersPage = lazy(() => import('./pages/PrinterProvidersPage').then((module) => ({ default: module.PrinterProvidersPage })));
const AuditLogsPage = lazy(() => import('./pages/AuditLogsPage').then((module) => ({ default: module.AuditLogsPage })));
const AnnouncementsPage = lazy(() => import('./pages/AnnouncementsPage').then((module) => ({ default: module.AnnouncementsPage })));

function RouteLoader() {
  return <div className="route-loading"><Spin size="large" /></div>;
}

function ProtectedLayout() {
  const { authenticated, loading } = useAuth();
  const location = useLocation();
  if (loading) return <div className="app-loading"><Spin size="large" tip="正在验证登录状态…" /></div>;
  if (!authenticated) return <Navigate to="/login" replace state={{ from: location.pathname }} />;
  return <AppShell />;
}

function PlatformAdminOnly({ children }: { children: ReactNode }) {
  const { user } = useAuth();
  return canManagePlatformUsers(user) ? children : <Navigate to="/dashboard" replace />;
}

export default function App() {
  return (
    <Suspense fallback={<RouteLoader />}>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route element={<ProtectedLayout />}>
          <Route path="/dashboard" element={<DashboardPage />} />
          <Route path="/users" element={<PlatformAdminOnly><UsersPage /></PlatformAdminOnly>} />
          <Route path="/tenants" element={<TenantsPage />} />
          <Route path="/stores" element={<StoresPage />} />
          <Route path="/announcements" element={<AnnouncementsPage />} />
          <Route path="/settings/payment" element={<PaymentSettingsPage />} />
          <Route path="/settings/system" element={<SystemSettingsPage />} />
          <Route path="/settings/printers" element={<PrinterProvidersPage />} />
          <Route path="/audit-logs" element={<AuditLogsPage />} />
        </Route>
        <Route path="*" element={<Navigate to="/dashboard" replace />} />
      </Routes>
    </Suspense>
  );
}
