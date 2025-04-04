import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import path from 'path';

export default defineConfig({
  plugins: [svelte({ hot: false })],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/tests/setup.ts'],
    include: ['./src/tests/**/*.{test,spec}.{js,ts}'],
    coverage: {
      reporter: ['text', 'json', 'html'],
      exclude: ['**/node_modules/**', '**/tests/**', '**/*.d.ts']
    }
  },
  resolve: {
    alias: {
      'svelte-portal/src/Portal.svelte': path.resolve(__dirname, 'node_modules/svelte-portal/src/Portal.svelte')
    }
  }
});