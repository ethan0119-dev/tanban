import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { authService } from '../lib/services';
import {
  AUTH_UNAUTHORIZED_EVENT,
  clearToken,
  getToken,
  setToken as persistToken,
} from '../lib/auth-storage';
import type { CurrentUser } from '../types';

interface AuthContextValue {
  user: CurrentUser | null;
  loading: boolean;
  authenticated: boolean;
  login: (account: string, password: string) => Promise<void>;
  logout: () => void;
  refreshUser: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<CurrentUser | null>(null);
  const [loading, setLoading] = useState(Boolean(getToken()));

  const logout = useCallback(() => {
    clearToken();
    setUser(null);
    setLoading(false);
  }, []);

  const refreshUser = useCallback(async () => {
    if (!getToken()) {
      setUser(null);
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      setUser(await authService.me());
    } catch {
      logout();
    } finally {
      setLoading(false);
    }
  }, [logout]);

  const login = useCallback(async (account: string, password: string) => {
    const result = await authService.login(account, password);
    const token = result.token || result.accessToken || result.access_token;
    if (!token) throw new Error('登录响应缺少访问令牌');
    persistToken(token);
    try {
      const currentUser = result.user || (await authService.me());
      setUser(currentUser);
    } catch (error) {
      clearToken();
      throw error;
    }
  }, []);

  useEffect(() => {
    void refreshUser();
    window.addEventListener(AUTH_UNAUTHORIZED_EVENT, logout);
    return () => window.removeEventListener(AUTH_UNAUTHORIZED_EVENT, logout);
  }, [logout, refreshUser]);

  const value = useMemo(
    () => ({ user, loading, authenticated: Boolean(user && getToken()), login, logout, refreshUser }),
    [user, loading, login, logout, refreshUser],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);
  if (!context) throw new Error('useAuth 必须在 AuthProvider 内使用');
  return context;
}
