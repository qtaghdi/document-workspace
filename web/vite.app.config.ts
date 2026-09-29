import tailwindcss from '@tailwindcss/vite';
import { defineConfig, type Plugin } from 'vite';
import { viteSingleFile } from 'vite-plugin-singlefile';

export default defineConfig({
  plugins: [
    pruneUnusedHyphenationLocales(),
    tailwindcss(),
    viteSingleFile({ removeViteModuleLoader: true }),
  ],
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

function pruneUnusedHyphenationLocales(): Plugin {
  return {
    name: 'xlsx-viewer-prune-univer-hyphenation-locales',
    enforce: 'pre',
    transform(code, id) {
      if (!id.includes('@univerjs/engine-render/lib/es/index.js')) {
        return undefined;
      }
      const tableStart = code.indexOf('const PATTERN_LOADERS = {');
      const tableEnd = code.indexOf('\n};', tableStart);
      if (tableStart === -1 || tableEnd === -1) {
        this.error('Unable to locate Univer hyphenation loaders');
      }
      const table = code.slice(tableStart, tableEnd);
      let removed = 0;
      const pruned = table.replace(
        /\n\s*\["(?!en-us")[^"]+"\]: \(\) => import\("[^\"]+"\),?/g,
        () => {
          removed += 1;
          return '';
        },
      );
      if (removed < 50) {
        this.error(`Expected Univer locale loaders, removed only ${removed}`);
      }
      return code.slice(0, tableStart) + pruned + code.slice(tableEnd);
    },
  };
}
