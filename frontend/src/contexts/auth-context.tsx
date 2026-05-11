import { useCallback, createContext, type ReactNode } from 'react';
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

interface RouterLike {
  navigate: (opts: { to: string; search?: Record<string, string> }) => Promise<unknown>;
}

function getRouter(): RouterLike {
  const g = window as unknown as { __TSR_ROUTER__?: RouterLike };
  return g.__TSR_ROUTER__ ?? { navigate: () => Promise.resolve() };
}

export const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const router = getRouter();

  // Passive cache subscriber — never triggers network requests.
  // Populated by dashboard route's beforeLoad (ensureQueryData)
  // or by login mutation (invalidateQueries).
  const { data: user } = useQuery({
    queryKey: AUTH_ME_KEY,
    queryFn: () => get<User>('/auth/me'),
    enabled: false,
    staleTime: Infinity,
    gcTime: Infinity,
  });

  const login = useCallback(async (email: string, password: string) => {
    const body: LoginRequest = { email, password };
    await post<TokenResponse>('/auth/login', body);
    await queryClient.invalidateQueries({ queryKey: AUTH_ME_KEY });
    // Navigate to the dashboard — its beforeLoad guard will verify auth
    // and redirect back to /login if anything went wrong. Cannot use
    // router.invalidate() here because beforeLoad guards only run during
    // route transitions, not during in-place invalidation.
    await router.navigate({ to: '/' });
  }, [queryClient, router]);

  const logout = useCallback(async () => {
    try {
      await post('/auth/logout');
    } catch {
      // Best-effort
    }
    // Remove the cached user so any in-flight route guards see undefined
    // and refetch fresh (getting 401 → redirect to login).
    queryClient.removeQueries({ queryKey: AUTH_ME_KEY });
    // Navigate to /login — the login route's beforeLoad will try to fetch
    // /auth/me, get a 401, and stay on the login page.
    await router.navigate({ to: '/login', search: { redirect: '/' } });
  }, [queryClient, router]);

  return (
    <AuthContext.Provider
      value={{
        user: user ?? null,
        isLoading: false,
        isAuthenticated: !!user,
        login,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}
