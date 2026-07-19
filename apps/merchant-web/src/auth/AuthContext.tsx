import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { api, AUTH_UNAUTHORIZED_EVENT, TOKEN_KEY } from '../api/client';
import type { Id, MerchantUser } from '../types';

interface LoginResponse {
  token?: string;
  accessToken?: string;
  access_token?: string;
  user?: MerchantUser;
}

interface AuthValue {
  user: MerchantUser | null;
  loading: boolean;
  login: (account: string, password: string) => Promise<void>;
  logout: () => void;
  refreshMe: () => Promise<void>;
}

const AuthContext = createContext<AuthValue | null>(null);

function normalizeUser(value: unknown): MerchantUser {
  const source = (value && typeof value === 'object' && 'user' in value)
    ? (value as { user: unknown }).user
    : value;
  const user = (source ?? {}) as Partial<MerchantUser> & { user_id?: Id; username?: string; nickname?: string; display_name?: string; role?: string };
  return {
    id: user.id ?? user.user_id ?? 'current',
    name: user.name ?? user.display_name ?? user.nickname ?? user.username ?? '商户管理员',
    phone: user.phone,
    avatar: user.avatar,
    merchantName: user.merchantName,
    storeName: user.storeName,
    roles: user.roles ?? (user.role ? [user.role] : undefined),
  };
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<MerchantUser | null>(null);
  const [loading, setLoading] = useState(true);

  const logout = useCallback(() => {
    localStorage.removeItem(TOKEN_KEY);
    setUser(null);
  }, []);

  const refreshMe = useCallback(async () => {
    const me = await api.get<MerchantUser | { user: MerchantUser }>('/auth/me');
    setUser(normalizeUser(me));
  }, []);

  useEffect(() => {
    let active = true;
    const bootstrap = async () => {
      if (!localStorage.getItem(TOKEN_KEY)) {
        setLoading(false);
        return;
      }
      try {
        const me = await api.get<MerchantUser | { user: MerchantUser }>('/auth/me');
        if (active) setUser(normalizeUser(me));
      } catch {
        if (active) logout();
      } finally {
        if (active) setLoading(false);
      }
    };
    void bootstrap();
    window.addEventListener(AUTH_UNAUTHORIZED_EVENT, logout);
    return () => {
      active = false;
      window.removeEventListener(AUTH_UNAUTHORIZED_EVENT, logout);
    };
  }, [logout]);

  const login = useCallback(async (account: string, password: string) => {
    const result = await api.post<LoginResponse>('/auth/login', {
      username: account,
      password,
      portal: 'merchant',
    });
    const token = result.token ?? result.accessToken ?? result.access_token;
    if (!token) throw new Error('登录成功响应中缺少访问令牌');
    localStorage.setItem(TOKEN_KEY, token);
    if (result.user) setUser(normalizeUser(result.user));
    else await refreshMe();
  }, [refreshMe]);

  const value = useMemo<AuthValue>(() => ({ user, loading, login, logout, refreshMe }), [user, loading, login, logout, refreshMe]);
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthValue {
  const value = useContext(AuthContext);
  if (!value) throw new Error('useAuth must be used within AuthProvider');
  return value;
}
