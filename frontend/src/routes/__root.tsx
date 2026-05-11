import { createRootRouteWithContext, HeadContent, Outlet } from '@tanstack/react-router';
import type { QueryClient } from '@tanstack/react-query';

interface RouterContext {
  queryClient: QueryClient;
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
    <>
      <HeadContent />
      <Outlet />
    </>
  );
}
