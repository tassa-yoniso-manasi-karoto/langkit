import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte()],
  server: {
    watch: {
      usePolling: true,
      interval: 1000,
    },
    host: true,
    strictPort: true,
    port: 34115,
    hmr: {
      protocol: 'ws',
      host: 'localhost',
    },
  },
  css: {
    postcss: './postcss.config.cjs',
    devSourcemap: true,
  },
})