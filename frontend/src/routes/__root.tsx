import { createRootRouteWithContext, HeadContent, Outlet, useRouter } from '@tanstack/react-router';
import { useEffect } from 'react';
import { AuthProvider } from '@/contexts/auth-context';
import { useAuth } from '@/hooks/use-auth';
import { useIdleTimeout } from '@/hooks/use-idle-timeout';
import type { QueryClient } from '@tanstack/react-query';

interface RouterContext {
  queryClient: QueryClient;
}

function AuthLoader() {
  const { isLoading, isAuthenticated, logout } = useAuth();
  const router = useRouter();
  const currentPath = router.state.location.pathname;

  useIdleTimeout(isAuthenticated ? logout : () => {});

  useEffect(() => {
    if (!isAuthenticated && currentPath !== '/login') {
      void router.navigate({ to: '/login', search: { redirect: currentPath } });
    }
  }, [isAuthenticated, currentPath, router]);

  if (isLoading || (!isAuthenticated && currentPath !== '/login')) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    );
  }

  return (
    <>
      <HeadContent />
      <Outlet />
    </>
  );
}

export const Route = createRootRouteWithContext<RouterContext>()({
  head: () => ({
    meta: [
      { charSet: 'UTF-8' },
      { name: 'viewport', content: 'width=device-width, initial-scale=1.0' },
      { title: 'Boilerplate App — Fullstack Modern' },
      { name: 'description', content: 'Aplikasi full-stack boilerplate dengan autentikasi dan dashboard.' },
      { name: 'theme-color', content: '#ffffff' },
      { property: 'og:title', content: 'Boilerplate App — Fullstack Modern' },
      { property: 'og:description', content: 'Aplikasi full-stack boilerplate dengan autentikasi dan dashboard.' },
      { property: 'og:type', content: 'website' },
      { property: 'og:image', content: '/og-image.webp' },
      { property: 'og:url', content: 'https://boilerplate.app' },
      { name: 'twitter:card', content: 'summary_large_image' },
      { name: 'twitter:title', content: 'Boilerplate App — Fullstack Modern' },
      { name: 'twitter:description', content: 'Aplikasi full-stack boilerplate dengan autentikasi dan dashboard.' },
      { name: 'twitter:image', content: '/og-image.webp' },
    ],
    links: [
      { rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' },
      { rel: 'canonical', href: 'https://boilerplate.app' },
    ],
  }),
  component: RootLayout,
});

export function RootLayout() {
  return (
    <AuthProvider>
      <AuthLoader />
    </AuthProvider>
  );
}
