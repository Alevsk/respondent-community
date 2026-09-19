import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import reactScan from '@react-scan/vite-plugin-react-scan';
import cesium from 'vite-plugin-cesium';
import path from 'path';

export default defineConfig({
  envDir: path.resolve(__dirname, '../../../'),
  plugins: [
    react({ babel: { plugins: [['babel-plugin-react-compiler', { target: '18' }]] } }),
    reactScan({
      enable: process.env.NODE_ENV !== 'production',
      autoDisplayNames: true,
    }),
    cesium({
      cesiumBuildRootPath: path.resolve(__dirname, '../../node_modules/cesium/Build'),
      cesiumBuildPath: path.resolve(__dirname, '../../node_modules/cesium/Build/Cesium/'),
    }),
  ],
  resolve: {
    alias: [
      { find: '@', replacement: path.resolve(__dirname, './src') },
      {
        find: '@respondent/core',
        replacement: path.resolve(__dirname, '../../packages/core/src/index.ts'),
      },
    ],
    dedupe: ['react', 'react-dom', 'zustand'],
  },
  build: {
    chunkSizeWarningLimit: 5000,
    rollupOptions: {
      output: {
        manualChunks: (id) => {
          if (!id.includes('node_modules')) return undefined;
          if (
            id.includes('/node_modules/react/') ||
            id.includes('/node_modules/react-dom/') ||
            id.includes('/node_modules/scheduler/')
          ) {
            return 'vendor-react';
          }
          if (id.includes('/node_modules/@mui/') || id.includes('/node_modules/@emotion/')) {
            return 'vendor-mui';
          }
          return undefined;
        },
      },
    },
  },
  server: {
    port: 3301,
    allowedHosts: ['.ngrok-free.app', '.ngrok.app', '.ngrok.io', '.tailscale.net'],
    proxy: {
      '/v1': {
        target: process.env.VITE_API_TARGET || 'http://localhost:8090',
        changeOrigin: true,
      },
      '/api': {
        target: process.env.VITE_API_TARGET || 'http://localhost:8090',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, ''),
      },
      '/ws': {
        target: process.env.VITE_WS_TARGET || 'ws://localhost:8092',
        ws: true,
      },
    },
  },
});
