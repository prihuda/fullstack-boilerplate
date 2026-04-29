import { createRootRouteWithContext, Outlet, useRouter } from '@tanstack/react-router';
import { AuthProvider, useAuth } from '@/hooks/use-auth';
import { useIdleTimeout } from '@/hooks/use-idle-timeout';
import type { QueryClient } from '@tanstack/react-query';

interface RouterContext {
  queryClient: QueryClient;
}

function AuthLoader() {
  const { isLoading, isAuthenticated, logout } = useAuth();
  const router = useRouter();

  useIdleTimeout(logout);

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    );
  }

  if (!isAuthenticated && router.state.location.pathname !== '/login') {
    void router.navigate({ to: '/login' });
    return null;
  }

  return <Outlet />;
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: RootLayout,
});

function RootLayout() {
  return (
    <AuthProvider>
      <AuthLoader />
    </AuthProvider>
  );
}
