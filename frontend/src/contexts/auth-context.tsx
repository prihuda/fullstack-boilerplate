import {
  createContext,
  useCallback,
  type ReactNode,
} from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
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

  const { data: user, isLoading, isError } = useQuery({
    queryKey: AUTH_ME_KEY,
    queryFn: () => get<User>('/auth/me'),
    retry: false,
    staleTime: 30_000,
  });

  const login = useCallback(async (email: string, password: string) => {
    const body: LoginRequest = { email, password };
    await post<TokenResponse>('/auth/login', body);
    // Refetch auth state — AuthLoader/LoginPage handles the redirect
    await queryClient.invalidateQueries({ queryKey: AUTH_ME_KEY });
  }, [queryClient]);

  const logout = useCallback(async () => {
    try {
      await post('/auth/logout');
    } catch {
      // Ignore logout errors — clear local state regardless
    }
    queryClient.setQueryData(AUTH_ME_KEY, null);
    // AuthLoader's useEffect detects !isAuthenticated and redirects to /login
  }, [queryClient]);

  return (
    <AuthContext.Provider
      value={{
        user: user ?? null,
        isLoading,
        isAuthenticated: !isLoading && !isError && !!user,
        login,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}
