import { createFileRoute, redirect } from '@tanstack/react-router';
import { LoginPage } from '@/components/pages/login-page';

export const Route = createFileRoute('/login')({
  validateSearch: (search: Record<string, unknown>) => ({
    redirect: (search.redirect as string) || '/',
  }),
  beforeLoad: ({ context, search }) => {
    // Check cache only — no network request on every navigation to /login.
    // The dashboard route's beforeLoad handles full auth verification.
    if (context.queryClient.getQueryData(['auth', 'me'])) {
      throw redirect({ to: search.redirect });
    }
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
