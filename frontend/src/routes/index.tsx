import { createFileRoute } from '@tanstack/react-router';
import { DashboardPage } from '@/components/pages/dashboard-page';

export const Route = createFileRoute('/')({
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
