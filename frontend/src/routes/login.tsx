import { createFileRoute, redirect } from '@tanstack/react-router';
import { LoginPage } from '@/components/pages/login-page';
import { get } from '@/lib/api';
import type { User } from '@/types/auth';

export const Route = createFileRoute('/login')({
  validateSearch: (search: Record<string, unknown>) => ({
    redirect: (search.redirect as string) || '/',
  }),
  beforeLoad: async ({ context, search }) => {
    try {
      await context.queryClient.ensureQueryData({
        queryKey: ['auth', 'me'],
        queryFn: () => get<User>('/auth/me'),
      });
    } catch {
      // Not authenticated — stay on login page
      return;
    }
    // Already authenticated — redirect away from login
    // NOTE: redirect() throw must be OUTSIDE the try/catch so it
    // propagates to the router's internal handler instead of being swallowed
    throw redirect({ to: search.redirect });
  },
  head: () => ({
    meta: [
      { title: 'Masuk - Boilerplate App' },
      { name: 'description', content: 'Masuk ke akun Boilerplate App Anda.' },
      { property: 'og:title', content: 'Masuk - Boilerplate App' },
      { name: 'twitter:title', content: 'Masuk - Boilerplate App' },
    ],
  }),
  component: LoginPage,
});
