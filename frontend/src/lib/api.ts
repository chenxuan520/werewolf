import type { ActionPayload, ControlPayload, CreateGamePayload, Preset, RecordSummary, ReplayDetail, Snapshot, Template } from './types'

async function parseJSON<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const payload = (await response.json().catch(() => null)) as { error?: string } | null
    throw new Error(payload?.error ?? `请求失败: ${response.status}`)
  }
  return response.json() as Promise<T>
}

export async function fetchTemplates(): Promise<Template[]> {
  const response = await fetch('/api/templates')
  const payload = await parseJSON<{ templates: Template[] }>(response)
  return payload.templates
}

export async function fetchPresets(): Promise<Preset[]> {
  const response = await fetch('/api/presets')
  const payload = await parseJSON<{ presets: Preset[] }>(response)
  return payload.presets
}

export async function createGame(payload: CreateGamePayload): Promise<Snapshot> {
  const response = await fetch('/api/games', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  return parseJSON<Snapshot>(response)
}

export async function fetchGame(id: string): Promise<Snapshot> {
  const response = await fetch(`/api/games/${encodeURIComponent(id)}`)
  return parseJSON<Snapshot>(response)
}

export async function submitGameAction(id: string, payload: ActionPayload): Promise<Snapshot> {
  const response = await fetch(`/api/games/${encodeURIComponent(id)}/actions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  return parseJSON<Snapshot>(response)
}

export async function controlGame(id: string, payload: ControlPayload): Promise<Snapshot> {
  const response = await fetch(`/api/games/${encodeURIComponent(id)}/control`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  return parseJSON<Snapshot>(response)
}

export async function fetchRecords(): Promise<RecordSummary[]> {
  const response = await fetch('/api/records')
  const payload = await parseJSON<{ records: RecordSummary[] }>(response)
  return payload.records
}

export async function fetchReplay(id: string): Promise<ReplayDetail> {
  const response = await fetch(`/api/replays/${encodeURIComponent(id)}`)
  return parseJSON<ReplayDetail>(response)
}
