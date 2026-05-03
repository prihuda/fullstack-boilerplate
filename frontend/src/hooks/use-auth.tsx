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
import type { User, TokenResponse, LoginRequest } from '@/types/auth';
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
      const user = await get<User>('/auth/me');
      setState({ user, isLoading: false, isAuthenticated: true });
    } catch {
      setState({ user: null, isLoading: false, isAuthenticated: false });
    }
  }, []);

  const login = useCallback(async (email: string, password: string) => {
    const body: LoginRequest = { email, password };
    await post<TokenResponse>('/auth/login', body);
    // Cookies are set by backend — now fetch user info
    await checkAuth();
    await queryClient.invalidateQueries({ queryKey: AUTH_KEY });
  }, [checkAuth, queryClient]);

  const logout = useCallback(async () => {
    try {
      await post('/auth/logout');
    } catch {
      // Ignore logout errors — clear local state regardless
    }
    setState({ user: null, isLoading: false, isAuthenticated: false });
    queryClient.invalidateQueries({ queryKey: AUTH_KEY });
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
