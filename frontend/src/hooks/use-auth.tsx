import {
  createContext,
  useContext,
  useState,
  useCallback,
  useEffect,
  type ReactNode,
} from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { post, get } from '@/lib/api';
import type { User, AuthResponse, LoginRequest } from '@/types/auth';

interface AuthState {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
}

interface AuthContextValue extends AuthState {
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  checkAuth: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

const AUTH_KEY = ['auth', 'me'];

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();

  const [state, setState] = useState<AuthState>({
    user: null,
    isLoading: true,
    isAuthenticated: false,
  });

  const checkAuth = useCallback(async () => {
    setState((prev) => ({ ...prev, isLoading: true }));
    try {
      const data = await get<AuthResponse>('/auth/me');
      setState({ user: data.user, isLoading: false, isAuthenticated: true });
    } catch {
      setState({ user: null, isLoading: false, isAuthenticated: false });
    }
  }, []);

  const login = useCallback(async (email: string, password: string) => {
    const body: LoginRequest = { email, password };
    const data = await post<AuthResponse>('/auth/login', body);
    setState({ user: data.user, isLoading: false, isAuthenticated: true });
    await queryClient.invalidateQueries({ queryKey: AUTH_KEY });
  }, [queryClient]);

  const logout = useCallback(async () => {
    try {
      await post('/auth/logout');
    } catch {
      // Ignore logout errors — clear local state regardless
    }
    setState({ user: null, isLoading: false, isAuthenticated: false });
    queryClient.clear();
    await navigate({ to: '/login' });
  }, [navigate, queryClient]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void checkAuth();
  }, [checkAuth]);

  return (
    <AuthContext.Provider value={{ ...state, login, logout, checkAuth }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
