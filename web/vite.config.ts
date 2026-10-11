import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

// Dev server proxies /api to the Go backend (default http://localhost:8080; override with XP_API_ORIGIN).
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: Number(process.env.XP_WEB_PORT ?? 5173),
    proxy: { '/api': process.env.XP_API_ORIGIN ?? 'http://localhost:8080' },
  },
});
