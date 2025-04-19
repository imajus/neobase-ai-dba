import react from '@vitejs/plugin-react';
import path from 'path';
import { defineConfig } from 'vite';

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  optimizeDeps: {
    include: [
      'rehype-raw',
      'rehype-sanitize',
      'rehype-slug',
      'rehype-autolink-headings',
      'rehype-highlight',
      'highlight.js/styles/github.css',
      'react',
      'react-dom',
      'lucide-react'
    ]
  },
  build: {
    commonjsOptions: {
      include: [/rehype-.*/, /highlight\.js/, /node_modules/],
      transformMixedEsModules: true
    },
    rollupOptions: {
      onwarn(warning, warn) {
        // Ignore "Module level directives" warnings from lucide-react
        if (warning.code === 'MODULE_LEVEL_DIRECTIVE') {
          return;
        }
        warn(warning);
      }
    }
  },
  publicDir: 'public',
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
      '@public': path.resolve(__dirname, './public')
    }
  }
});
