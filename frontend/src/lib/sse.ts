export function subscribeGameStream(gameId: string, mode: 'spectator' | 'human', onEvent: () => void): () => void {
  if (typeof EventSource === 'undefined') {
    return () => {}
  }
  const source = new EventSource(`/api/games/${encodeURIComponent(gameId)}/stream?mode=${mode}`)
  const handle = () => onEvent()
  source.addEventListener('ready', handle)
  source.addEventListener('phase_started', handle)
  source.addEventListener('speech_recorded', handle)
  source.addEventListener('vote_cast', handle)
  source.addEventListener('vote_resolved', handle)
  source.addEventListener('player_eliminated', handle)
  source.addEventListener('night_resolved', handle)
  source.addEventListener('game_finished', handle)
  source.onerror = () => {
    // 浏览器会自动重连，前端不用额外处理。
  }
  return () => source.close()
}
