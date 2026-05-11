import { createFileRoute } from '@tanstack/react-router';
import { LoginPage } from '@/components/pages/login-page';

export const Route = createFileRoute('/login')({
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
