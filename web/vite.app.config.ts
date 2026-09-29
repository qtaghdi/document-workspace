import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';
import { viteSingleFile } from 'vite-plugin-singlefile';

export default defineConfig({
  plugins: [tailwindcss(), viteSingleFile({ removeViteModuleLoader: true })],
  esbuild: {
    charset: 'ascii',
  },
  build: {
    emptyOutDir: false,
    minify: 'terser',
    outDir: '../internal/httpapi/static',
    rollupOptions: {
      input: 'app.html',
    },
    terserOptions: {
      format: {
        ascii_only: true,
      },
    },
  },
});
