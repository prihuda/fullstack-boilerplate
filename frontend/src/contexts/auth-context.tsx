import {
  createContext,
  useCallback,
  type ReactNode,
} from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { post, get } from '@/lib/api';
import type { User, TokenResponse, LoginRequest } from '@/types/auth';

export interface AuthContextValue {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
}

const AUTH_ME_KEY = ['auth', 'me'] as const;

export const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();

  const { data: user, isLoading, isError } = useQuery({
    queryKey: AUTH_ME_KEY,
    queryFn: () => get<User>('/auth/me'),
    retry: false,
    staleTime: 30_000,
  });

  const login = useCallback(async (email: string, password: string) => {
    const body: LoginRequest = { email, password };
    await post<TokenResponse>('/auth/login', body);
    queryClient.clear();
  }, [queryClient]);

  const logout = useCallback(async () => {
    try {
      await post('/auth/logout');
    } catch {
      // Ignore logout errors — clear local state regardless
    }
    queryClient.clear();
    await navigate({ to: '/login' });
  }, [navigate, queryClient]);

  return (
    <AuthContext.Provider
      value={{
        user: user ?? null,
        isLoading,
        isAuthenticated: !isLoading && !isError && user !== undefined,
        login,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}
