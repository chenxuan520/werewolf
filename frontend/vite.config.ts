import fs from 'node:fs'
import path from 'node:path'
import { defineConfig } from 'vite'

function loadRuntimeConfig() {
  const candidates = [path.resolve(process.cwd(), '../config/app.json'), path.resolve(process.cwd(), 'config/app.json')]
  const configPath = candidates.find((candidate) => fs.existsSync(candidate))
  if (!configPath) {
    return { frontend: { host: '127.0.0.1', port: 5174, apiTarget: 'http://127.0.0.1:18131' } }
  }
  return JSON.parse(fs.readFileSync(configPath, 'utf-8')) as {
    frontend?: { host?: string; port?: number; apiTarget?: string }
  }
}

const runtimeConfig = loadRuntimeConfig()

export default defineConfig({
  server: {
    host: runtimeConfig.frontend?.host ?? '127.0.0.1',
    port: runtimeConfig.frontend?.port ?? 5174,
    proxy: {
      '/api': {
        target: runtimeConfig.frontend?.apiTarget ?? 'http://127.0.0.1:18131',
        changeOrigin: true,
      },
    },
  },
})
