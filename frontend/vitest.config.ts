import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./test/setup.ts'],
    include: ['**/*.{test,spec}.{js,mjs,cjs,ts,mts,cts,jsx,tsx}'],
    exclude: ['**/node_modules/**', '**/e2e/**'],
    coverage: {
      reporter: ['text', 'json', 'html'],
      exclude: [
        'node_modules/',
        'test/',
        '**/*.d.ts',
        '**/*.config.*',
        '**/index.ts',
      ],
    },
  },
  resolve: {
    alias: [
      { find: '@', replacement: path.resolve(__dirname, './apps/earth/src') },
      { find: '@respondent/core', replacement: path.resolve(__dirname, './packages/core/src/index.ts') },
      { find: /^@respondent\/earth\/(.+)$/, replacement: path.resolve(__dirname, './apps/earth/src/$1') },
      { find: '@respondent/earth', replacement: path.resolve(__dirname, './apps/earth/src/index.ts') },
    ],
  },
})
