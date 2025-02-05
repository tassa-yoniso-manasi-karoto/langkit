import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import path from 'path';

export default defineConfig({
  plugins: [svelte()],
  resolve: {
    alias: {
      'svelte-portal/src/Portal.svelte': path.resolve(__dirname, 'node_modules/svelte-portal/src/Portal.svelte')
    }
  },
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