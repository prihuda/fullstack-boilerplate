import { createFileRoute, redirect } from '@tanstack/react-router';
import { DashboardPage } from '@/components/pages/dashboard-page';
import { get } from '@/lib/api';
import type { User } from '@/types/auth';

export const Route = createFileRoute('/')({
  beforeLoad: async ({ context }) => {
    try {
      await context.queryClient.ensureQueryData({
        queryKey: ['auth', 'me'],
        queryFn: () => get<User>('/auth/me'),
      });
    } catch {
      throw redirect({ to: '/login', search: { redirect: '/' } });
    }
  },
  head: () => ({
    meta: [
      { title: 'Dashboard - Boilerplate App' },
      { name: 'description', content: 'Dashboard dan ringkasan akun Anda.' },
      { property: 'og:title', content: 'Dashboard - Boilerplate App' },
      { name: 'twitter:title', content: 'Dashboard - Boilerplate App' },
    ],
  }),
  component: DashboardPage,
});
