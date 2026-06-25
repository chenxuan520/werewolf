import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../src/App'

const templates = [
  { id: 'classic-6', name: '6 人基础板', seats: 6, roles: ['werewolf', 'werewolf', 'seer', 'witch', 'villager', 'villager'] },
]

const presets = [
  { id: 'steady-reader', name: '稳健派', style: 'steady', persona: 'test' },
]

const records: never[] = []

const snapshot = {
  id: 'game-1',
  status: 'running',
  mode: 'spectator',
  template: templates[0],
  day: 1,
  phase: 'night_wolf',
  message: '第 1 夜开始',
  players: [
    { seat: 0, name: '稳健派', alive: true, isHuman: false, presetId: 'steady-reader', role: '狼人' },
  ],
  events: [],
  createdAt: new Date().toISOString(),
  updatedAt: new Date().toISOString(),
}

describe('App', () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
      const url = typeof input === 'string' ? input : input.toString()
      if (url === '/api/templates') return ok({ templates })
      if (url === '/api/presets') return ok({ presets })
      if (url === '/api/records') return ok({ records })
      if (url === '/api/games') return ok(snapshot, 201)
      if (url === '/api/games/game-1') return ok(snapshot)
      throw new Error(`Unhandled fetch: ${url}`)
    }) as typeof fetch
  })

  it('renders lobby and can create a spectator game', async () => {
    const user = userEvent.setup()
    render(<App />)

    await waitFor(() => expect(screen.getByText('AI 狼人杀工作台')).toBeInTheDocument())
    await user.click(screen.getByRole('button', { name: '开始观战' }))

    await waitFor(() => expect(screen.getByText('实时事件')).toBeInTheDocument())
  })
})

function ok(payload: unknown, status = 200) {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}
