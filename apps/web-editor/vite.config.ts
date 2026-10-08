import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [tailwindcss()],
  esbuild: {
    charset: 'ascii',
  },
  build: {
    emptyOutDir: true,
    minify: 'terser',
    outDir: '../../internal/transport/httpapi/static',
    terserOptions: {
      format: {
        ascii_only: true,
      },
    },
  },
});
