import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { api, AUTH_UNAUTHORIZED_EVENT, SERVICE_EXPIRED_EVENT, TOKEN_KEY } from '../api/client';
import type { Id, MerchantUser, MerchantWorkspace } from '../types';

const SELECTION_TOKEN_KEY = 'tanban_merchant_selection_token';
const SELECTION_WORKSPACES_KEY = 'tanban_merchant_selection_workspaces';

type LoginResult = 'authenticated' | 'selection-required';

interface LoginResponse {
  token?: string;
  accessToken?: string;
  access_token?: string;
  user?: MerchantUser;
  selectionRequired?: boolean;
  selection_required?: boolean;
  selectionToken?: string;
  selection_token?: string;
  workspaces?: unknown[];
}

interface AuthValue {
  user: MerchantUser | null;
  workspaces: MerchantWorkspace[];
  pendingWorkspaces: MerchantWorkspace[];
  selectionRequired: boolean;
  loading: boolean;
  login: (account: string, password: string) => Promise<LoginResult>;
  selectWorkspace: (tenantId: Id) => Promise<void>;
  switchWorkspace: (tenantId: Id) => Promise<void>;
  logout: () => void;
  refreshMe: () => Promise<void>;
}

const AuthContext = createContext<AuthValue | null>(null);

function normalizeUser(value: unknown): MerchantUser {
  const source = (value && typeof value === 'object' && 'user' in value)
    ? (value as { user: unknown }).user
    : value;
  const user = (source ?? {}) as Partial<MerchantUser> & {
    user_id?: Id; username?: string; nickname?: string; display_name?: string; role?: string;
    tenant_id?: Id; store_id?: Id; tenant_name?: string; store_name?: string;
    service_expires_at?: string; service_expired?: boolean;
  };
  return {
    id: user.id ?? user.user_id ?? 'current',
    name: user.name ?? user.display_name ?? user.nickname ?? user.username ?? '商户管理员',
    phone: user.phone,
    avatar: user.avatar,
    storeName: user.storeName ?? user.store_name,
    tenantId: user.tenantId ?? user.tenant_id,
    storeId: user.storeId ?? user.store_id,
    tenantName: user.tenantName ?? user.tenant_name,
    merchantName: user.merchantName ?? user.tenantName ?? user.tenant_name,
    roles: user.roles ?? (user.role ? [user.role] : undefined),
    capabilities: user.capabilities,
    serviceExpiresAt: user.serviceExpiresAt ?? user.service_expires_at,
    serviceExpired: Boolean(user.serviceExpired ?? user.service_expired),
  };
}

function normalizeWorkspace(value: unknown): MerchantWorkspace {
  const item = (value ?? {}) as Partial<MerchantWorkspace> & {
    membership_id?: Id; tenant_id?: Id; tenant_name?: string; store_id?: Id; store_name?: string; store_logo_url?: string;
    service_expires_at?: string; service_expired?: boolean;
  };
  return {
    membershipId: item.membershipId ?? item.membership_id ?? '',
    tenantId: item.tenantId ?? item.tenant_id ?? '',
    tenantName: item.tenantName ?? item.tenant_name ?? '',
    storeId: item.storeId ?? item.store_id ?? '',
    storeName: item.storeName ?? item.store_name ?? '',
    storeLogoUrl: item.storeLogoUrl ?? item.store_logo_url,
    role: item.role ?? 'MERCHANT_OWNER',
    serviceExpiresAt: item.serviceExpiresAt ?? item.service_expires_at,
    serviceExpired: Boolean(item.serviceExpired ?? item.service_expired),
  };
}

function storedPendingWorkspaces(): MerchantWorkspace[] {
  try {
    const raw = sessionStorage.getItem(SELECTION_WORKSPACES_KEY);
    const parsed = raw ? JSON.parse(raw) : [];
    return Array.isArray(parsed) ? parsed.map(normalizeWorkspace) : [];
  } catch {
    return [];
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<MerchantUser | null>(null);
  const [workspaces, setWorkspaces] = useState<MerchantWorkspace[]>([]);
  const [pendingWorkspaces, setPendingWorkspaces] = useState<MerchantWorkspace[]>(storedPendingWorkspaces);
  const [loading, setLoading] = useState(true);

  const clearSelection = useCallback(() => {
    sessionStorage.removeItem(SELECTION_TOKEN_KEY);
    sessionStorage.removeItem(SELECTION_WORKSPACES_KEY);
    setPendingWorkspaces([]);
  }, []);

  const logout = useCallback(() => {
    localStorage.removeItem(TOKEN_KEY);
    clearSelection();
    setUser(null);
    setWorkspaces([]);
  }, [clearSelection]);

  const refreshWorkspaces = useCallback(async () => {
    const items = await api.get<unknown[]>('/auth/workspaces');
    setWorkspaces((items ?? []).map(normalizeWorkspace));
  }, []);

  const refreshMe = useCallback(async () => {
    const me = await api.get<MerchantUser | { user: MerchantUser }>('/auth/me');
    setUser(normalizeUser(me));
    await refreshWorkspaces();
  }, [refreshWorkspaces]);

  useEffect(() => {
    let active = true;
    const bootstrap = async () => {
      if (!localStorage.getItem(TOKEN_KEY)) {
        setLoading(false);
        return;
      }
      try {
        const me = await api.get<MerchantUser | { user: MerchantUser }>('/auth/me');
        if (active) {
          setUser(normalizeUser(me));
          const items = await api.get<unknown[]>('/auth/workspaces');
          if (active) setWorkspaces((items ?? []).map(normalizeWorkspace));
        }
      } catch {
        if (active) logout();
      } finally {
        if (active) setLoading(false);
      }
    };
    void bootstrap();
    const refreshServiceState = () => void refreshMe().catch(() => undefined);
    window.addEventListener(AUTH_UNAUTHORIZED_EVENT, logout);
    window.addEventListener(SERVICE_EXPIRED_EVENT, refreshServiceState);
    return () => {
      active = false;
      window.removeEventListener(AUTH_UNAUTHORIZED_EVENT, logout);
      window.removeEventListener(SERVICE_EXPIRED_EVENT, refreshServiceState);
    };
  }, [logout, refreshMe]);

  const login = useCallback(async (account: string, password: string) => {
    localStorage.removeItem(TOKEN_KEY);
    clearSelection();
    const result = await api.post<LoginResponse>('/auth/login', {
      username: account,
      password,
      portal: 'merchant',
    });
    if (result.selectionRequired || result.selection_required) {
      const selectionToken = result.selectionToken ?? result.selection_token;
      const options = (result.workspaces ?? []).map(normalizeWorkspace);
      if (!selectionToken || options.length === 0) throw new Error('未能取得可管理的店铺，请联系平台管理员');
      sessionStorage.setItem(SELECTION_TOKEN_KEY, selectionToken);
      sessionStorage.setItem(SELECTION_WORKSPACES_KEY, JSON.stringify(options));
      setPendingWorkspaces(options);
      return 'selection-required';
    }
    const token = result.token ?? result.accessToken ?? result.access_token;
    if (!token) throw new Error('登录成功响应中缺少访问令牌');
    localStorage.setItem(TOKEN_KEY, token);
    if (result.user) setUser(normalizeUser(result.user));
    else await refreshMe();
    await refreshWorkspaces();
    return 'authenticated';
  }, [clearSelection, refreshMe, refreshWorkspaces]);

  const acceptAccessResponse = useCallback(async (result: LoginResponse) => {
    const token = result.token ?? result.accessToken ?? result.access_token;
    if (!token) throw new Error('登录成功响应中缺少访问令牌');
    localStorage.setItem(TOKEN_KEY, token);
    clearSelection();
    if (result.user) setUser(normalizeUser(result.user));
    else await refreshMe();
    await refreshWorkspaces();
  }, [clearSelection, refreshMe, refreshWorkspaces]);

  const selectWorkspace = useCallback(async (tenantId: Id) => {
    const selectionToken = sessionStorage.getItem(SELECTION_TOKEN_KEY);
    if (!selectionToken) throw new Error('选店登录已过期，请重新登录');
    const result = await api.post<LoginResponse>('/auth/select-tenant', { selection_token: selectionToken, tenant_id: Number(tenantId) });
    await acceptAccessResponse(result);
  }, [acceptAccessResponse]);

  const switchWorkspace = useCallback(async (tenantId: Id) => {
    const result = await api.post<LoginResponse>('/auth/switch-tenant', { tenant_id: Number(tenantId) });
    await acceptAccessResponse(result);
  }, [acceptAccessResponse]);

  const value = useMemo<AuthValue>(() => ({
    user, workspaces, pendingWorkspaces, selectionRequired: !user && pendingWorkspaces.length > 0,
    loading, login, selectWorkspace, switchWorkspace, logout, refreshMe,
  }), [user, workspaces, pendingWorkspaces, loading, login, selectWorkspace, switchWorkspace, logout, refreshMe]);
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthValue {
  const value = useContext(AuthContext);
  if (!value) throw new Error('useAuth must be used within AuthProvider');
  return value;
}
