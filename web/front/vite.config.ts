import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      // Use the full build of Vue that includes the runtime compiler
      'vue': 'vue/dist/vue.esm-bundler.js'
    }
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8000',
      '/events': 'http://localhost:8000',
      '/d': 'http://localhost:8000',
      '/ios': 'http://localhost:8000',
      '/udid': 'http://localhost:8000',
      '/upload': 'http://localhost:8000',
      '/admin': 'http://localhost:8000',
      '/icon': 'http://localhost:8000',
    },
  },
  build: {
    outDir: '../../internal/web/dist/front',
    emptyOutDir: true,
    rollupOptions: {
      input: 'index.html',
      output: {
        entryFileNames: 'app.js',
        assetFileNames: (chunkInfo) => {
          if (chunkInfo.name && chunkInfo.name.endsWith('.css')) return 'style.css'
          return '[name][extname]'
        },
      },
    },
  },
})

