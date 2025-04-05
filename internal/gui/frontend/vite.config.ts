import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import path from 'path';

export default defineConfig({
  plugins: [svelte()],
  resolve: {
    alias: {
      'svelte-portal/src/Portal.svelte': path.resolve(__dirname, 'node_modules/svelte-portal/src/Portal.svelte'),
      '@wasm': path.resolve(__dirname, 'src/wasm/pkg')
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
  // Add optimized WebAssembly handling
  optimizeDeps: {
    // Exclude WebAssembly files from dependency optimization
    exclude: ['log_engine']
  },
  build: {
    // Ensure WebAssembly files are properly handled in production builds
    rollupOptions: {
      external: [
        // Prevent Vite from trying to process these files
        /\/wasm\/log_engine\.js/,
        /\/wasm\/log_engine_bg\.wasm/
      ],
      output: {
        // Preserve the WebAssembly module structure
        manualChunks: {}
      }
    },
    // Increase chunk size limit for WASM files
    chunkSizeWarningLimit: 1000
  }
})