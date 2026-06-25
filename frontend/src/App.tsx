import { useEffect, useMemo, useState } from 'react'
import type { Dispatch, SetStateAction } from 'react'
import { WerewolfSeat } from './components/WerewolfSeat'
import { controlGame, createGame, fetchGame, fetchPresets, fetchRecords, fetchReplay, fetchTemplates, submitGameAction } from './lib/api'
import { subscribeGameStream } from './lib/sse'
import { seatLayout } from './lib/seatLayout'
import type { PendingAction, Preset, RecordSummary, ReplayDetail, Snapshot, Template } from './lib/types'

type Tab = 'lobby' | 'room' | 'history' | 'replay'

type ReplayStep = {
  index: number
  event: ReplayDetail['events'][number]
  title: string
  detail: string
  tone: string
  actorSeat: number | null
  phase: string
}

export default function App() {
  const [tab, setTab] = useState<Tab>('lobby')
  const [templates, setTemplates] = useState<Template[]>([])
  const [presets, setPresets] = useState<Preset[]>([])
  const [records, setRecords] = useState<RecordSummary[]>([])
  const [replay, setReplay] = useState<ReplayDetail | null>(null)
  const [game, setGame] = useState<Snapshot | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)
  const [acting, setActing] = useState(false)
  const [spectatorMode, setSpectatorMode] = useState(true)
  const [spectatorRunMode, setSpectatorRunMode] = useState<'semi' | 'auto' | 'manual'>('semi')
  const [templateId, setTemplateId] = useState('')
  const [humanName, setHumanName] = useState('你')
  const [selectedAI, setSelectedAI] = useState<string[]>([])
  const [speechText, setSpeechText] = useState('')
  const [selectedTarget, setSelectedTarget] = useState('')
  const [useHeal, setUseHeal] = useState(false)
  const [poisonTarget, setPoisonTarget] = useState('')
  const [selectedReplayStepIndex, setSelectedReplayStepIndex] = useState(0)

  useEffect(() => {
    let alive = true
    Promise.all([fetchTemplates(), fetchPresets(), fetchRecords()])
      .then(([templateItems, presetItems, recordItems]) => {
        if (!alive) return
        setTemplates(templateItems)
        setPresets(presetItems)
        setRecords(recordItems)
        if (templateItems[0]) setTemplateId((current) => current || templateItems[0].id)
      })
      .catch((err: Error) => alive && setError(err.message))
      .finally(() => alive && setLoading(false))
    return () => {
      alive = false
    }
  }, [])

  const selectedTemplate = useMemo(
    () => templates.find((item) => item.id === templateId) ?? templates[0] ?? null,
    [templateId, templates],
  )
  const presetByID = useMemo(() => new Map(presets.map((preset) => [preset.id, preset])), [presets])
  const replaySteps = useMemo(() => buildReplaySteps(replay), [replay])
  const currentReplayStep = replaySteps[selectedReplayStepIndex] ?? null

  const requiredAISeats = selectedTemplate ? selectedTemplate.seats - (spectatorMode ? 0 : 1) : 0

  useEffect(() => {
    if (!presets.length || requiredAISeats <= 0) return
    setSelectedAI((current) => {
      const next = [...current]
      while (next.length < requiredAISeats) next.push(presets[0]!.id)
      return next.slice(0, requiredAISeats)
    })
  }, [presets, requiredAISeats])

  useEffect(() => {
    if (!game) return
    const unsubscribe = subscribeGameStream(game.id, game.mode as 'spectator' | 'human', () => {
      void refreshGame(game.id)
    })
    return unsubscribe
  }, [game?.id, game?.mode])

  useEffect(() => {
    if (!game?.pendingAction) return
    const pending = game.pendingAction
    if (pending.kind === 'select' && pending.options?.[0]) {
      setSelectedTarget(pending.options[0].value)
    }
    if (pending.kind === 'witch') {
      setUseHeal(Boolean(pending.allowHeal))
      setPoisonTarget(pending.options?.[0]?.value ?? '')
    }
  }, [game?.pendingAction])

  useEffect(() => {
    setSelectedReplayStepIndex(0)
  }, [replay?.summary.id])

  async function refreshGame(gameId: string) {
    try {
      const [snapshot, recordItems] = await Promise.all([fetchGame(gameId), fetchRecords()])
      setGame(snapshot)
      setRecords(recordItems)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  async function handleCreateGame() {
    if (!selectedTemplate) return
    setCreating(true)
    setError(null)
    try {
      const snapshot = await createGame({
        templateId: selectedTemplate.id,
        spectatorMode,
        semiAutoMode: spectatorMode ? spectatorRunMode === 'semi' : undefined,
        manualMode: spectatorMode ? spectatorRunMode === 'manual' : undefined,
        humanName: spectatorMode ? undefined : humanName,
        aiPresetIds: selectedAI,
      })
      setGame(snapshot)
      setTab('room')
      setSpeechText('')
      const nextRecords = await fetchRecords()
      setRecords(nextRecords)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setCreating(false)
    }
  }

  async function handleAction(payload: { action: string; text?: string; targetSeat?: number; useHeal?: boolean; usePoison?: boolean }) {
    if (!game) return
    setActing(true)
    setError(null)
    try {
      const snapshot = await submitGameAction(game.id, payload)
      setGame(snapshot)
      const nextRecords = await fetchRecords()
      setRecords(nextRecords)
      if (payload.action === 'speech') setSpeechText('')
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setActing(false)
    }
  }

  async function handleOpenReplay(id: string) {
    try {
      const detail = await fetchReplay(id)
      setReplay(detail)
      setTab('replay')
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  async function handleControl(action: string) {
    if (!game) return
    setActing(true)
    setError(null)
    try {
      const snapshot = await controlGame(game.id, { action })
      setGame(snapshot)
      const nextRecords = await fetchRecords()
      setRecords(nextRecords)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setActing(false)
    }
  }

  return (
    <div className="shell">
      <header className="hero">
        <div>
          <span className="eyebrow">AI Werewolf Workbench</span>
          <h1>AI 狼人杀工作台</h1>
          <p>参考 holdem 的工作台形态，先跑通纯 AI 观战和 1 真人参与两种模式。</p>
        </div>
        <nav className="tabs">
          <button className={tab === 'lobby' ? 'active' : ''} onClick={() => setTab('lobby')}>Lobby</button>
          <button className={tab === 'room' ? 'active' : ''} onClick={() => setTab('room')} disabled={!game}>Room</button>
          <button className={tab === 'history' ? 'active' : ''} onClick={() => setTab('history')}>History</button>
          <button className={tab === 'replay' ? 'active' : ''} onClick={() => setTab('replay')} disabled={!replay}>Replay</button>
        </nav>
      </header>

      {error ? <div className="error-banner">{error}</div> : null}

      {loading ? <section className="panel">正在加载配置...</section> : null}

      {!loading && tab === 'lobby' ? (
        <section className="panel grid-2">
          <div>
            <h2>建局设置</h2>
            <div className="mode-row">
              <button className={spectatorMode ? 'active' : ''} onClick={() => setSpectatorMode(true)}>纯 AI 观战</button>
              <button className={!spectatorMode ? 'active' : ''} onClick={() => setSpectatorMode(false)}>1 真人参与</button>
            </div>
            {spectatorMode ? (
              <div className="submode-panel">
                <div className="submode-panel-head">
                  <strong>观战推进方式</strong>
                  <span>
                    {spectatorRunMode === 'semi'
                      ? '每天投票结算后暂停一次'
                      : spectatorRunMode === 'manual'
                        ? '每次 AI 行动都要手动点下一步'
                        : '整场持续自动推进'}
                  </span>
                </div>
                <div className="mode-row secondary">
                  <button className={spectatorRunMode === 'semi' ? 'active' : ''} onClick={() => setSpectatorRunMode('semi')}>半自动（默认）</button>
                  <button className={spectatorRunMode === 'auto' ? 'active' : ''} onClick={() => setSpectatorRunMode('auto')}>全自动</button>
                  <button className={spectatorRunMode === 'manual' ? 'active' : ''} onClick={() => setSpectatorRunMode('manual')}>手动逐步</button>
                </div>
              </div>
            ) : null}
            <label>
              <span>板型</span>
              <select value={selectedTemplate?.id ?? ''} onChange={(event) => setTemplateId(event.target.value)}>
                {templates.map((template) => (
                  <option key={template.id} value={template.id}>{template.name}（{template.seats}人）</option>
                ))}
              </select>
            </label>
            {!spectatorMode ? (
              <label>
                <span>你的名字</span>
                <input value={humanName} onChange={(event) => setHumanName(event.target.value)} />
              </label>
            ) : null}
            <div className="seat-stack">
              {selectedAI.map((value, index) => (
                <label key={index}>
                  <span>AI 座位 {index + 1}</span>
                  <select value={value} onChange={(event) => updateAI(index, event.target.value, setSelectedAI)}>
                    {presets.map((preset) => (
                      <option key={preset.id} value={preset.id}>{preset.name} · {presetSubtitle(preset)}</option>
                    ))}
                </select>
                </label>
              ))}
            </div>
            <button className="primary" onClick={handleCreateGame} disabled={creating || !selectedTemplate || selectedAI.length !== requiredAISeats}>
              {creating ? '正在创建...' : spectatorMode ? '开始观战' : '开始人机局'}
            </button>
          </div>

          <div>
            <h2>当前首版板型</h2>
            <ul className="template-list">
              {templates.map((template) => (
                <li key={template.id}>
                  <strong>{template.name}</strong>
                  <span>{template.roles.map(roleLabel).join(' / ')}</span>
                </li>
              ))}
            </ul>
          </div>
        </section>
      ) : null}

      {!loading && tab === 'room' ? (
        <section className="panel room-layout">
          {!game ? <div className="empty">先创建一局。</div> : <>
            {(() => {
              const seatStyles = seatLayout(game.players.length)
              const seatNotes = buildSeatNoteMap(game.events)
              const latestSpeechBubble = buildLatestSpeechBubble(game.events)
              const currentActor = game.pendingAction ? game.players.find((player) => player.seat === game.pendingAction?.actorSeat) ?? null : null
              const turnStatus = describeWerewolfTurnStatus(game, currentActor?.name ?? null)
              const latestSpeech = [...game.events].reverse().find((event) => event.type === 'speech_recorded') ?? null
              const latestAIAction = [...game.events].reverse().find((event) => event.type === 'ai_turn_generated') ?? null
              return <>
            <section className="council-room">
              <header className="council-room-header">
                <div className="council-room-id">
                  <span className="match-eyebrow">ROOM</span>
                  <strong>{game.template.name}</strong>
                  <span className="muted-text">对局 #{game.id} · {new Date(game.createdAt).toLocaleString('zh-CN')}</span>
                </div>
                <div className="council-room-meta">
                  <span className="winner-pill subtle-pill">{game.mode === 'spectator' ? '纯 AI 观战' : '1 真人参与'}</span>
                  <span className="winner-pill subtle-pill">Day {game.day}</span>
                  <span className="winner-pill subtle-pill">{phaseLabel(game.phase)}</span>
                  <span className="winner-pill subtle-pill">{game.status}</span>
                </div>
              </header>

              <div className="council-stage">
                <div className={`council-table ${phaseThemeClass(game.phase)}`}>
                  <div className="council-table-rim" aria-hidden="true" />
                  <div className="council-table-felt">
                    <div className="council-table-logo" aria-hidden="true">
                      WEREWOLF<span className="logo-moon">☾</span>
                    </div>
                    <div className="council-table-center">
                      <span className="table-phase-chip">{game.mode === 'spectator' ? 'SPECTATE' : 'PLAYER MODE'}</span>
                      <strong>{game.message}</strong>
                      <span>Day {game.day} · {phaseLabel(game.phase)}</span>
                      {game.winnerSide ? <span className="table-center-note winner">胜方：{game.winnerSide}</span> : <span className="table-center-note">{game.pendingAction?.prompt || '等待下一步推进'}</span>}
                    </div>
                  </div>
                  {game.players.map((player, index) => (
                    <WerewolfSeat
                      key={`${game.id}-${player.seat}`}
                      style={seatStyles[index]}
                      name={player.name}
                      seat={player.seat}
                      subtitle={player.isHuman ? '真人玩家' : `AI · ${presetSubtitle(presetByID.get(player.presetId ?? '')) || player.presetId || 'scripted'}`}
                      roleText={roomSeatRoleText(game, player)}
                      revealedRole={player.revealedRole}
                      note={seatNotes.get(player.seat)}
                      isAlive={player.alive}
                      isActive={game.status === 'running' && game.pendingAction?.actorSeat === player.seat}
                      isHuman={player.isHuman}
                      bubble={latestSpeechBubble?.seat === player.seat ? { title: latestSpeechBubble.title, detail: latestSpeechBubble.detail } : null}
                      bubbleDirection={seatStyles[index]?.bubbleDirection}
                    />
                  ))}
                </div>
              </div>

              {game.mode === 'spectator' ? (
                <div className="control-bar">
                  <button className="ghost-button" onClick={() => handleControl(game.control?.paused ? 'continue' : 'pause')} disabled={acting || game.status === 'finished' || game.status === 'stopped'}>
                    {game.control?.paused ? '继续' : '暂停'}
                  </button>
                  <button className={`ghost-button ${game.control?.semiAutoMode ? 'is-active' : ''}`} onClick={() => handleControl('semi_auto_on')} disabled={acting || game.status === 'finished' || game.status === 'stopped'}>
                    半自动
                  </button>
                  <button className={`ghost-button ${!game.control?.semiAutoMode && !game.control?.manualMode ? 'is-active' : ''}`} onClick={() => handleControl('auto_on')} disabled={acting || game.status === 'finished' || game.status === 'stopped'}>
                    全自动
                  </button>
                  <button className={`ghost-button ${game.control?.manualMode ? 'is-active' : ''}`} onClick={() => handleControl('manual_on')} disabled={acting || game.status === 'finished' || game.status === 'stopped'}>
                    手动模式
                  </button>
                  <button className="primary-button inline" onClick={() => handleControl('step')} disabled={acting || !game.control?.manualMode || game.status === 'finished' || game.status === 'stopped'}>
                    下一步
                  </button>
                  <button className="primary-button inline" onClick={() => handleControl('continue')} disabled={acting || game.status === 'finished' || game.status === 'stopped' || game.control?.manualMode}>
                    {game.control?.semiAutoMode ? '继续下一天' : '继续阶段'}
                  </button>
                </div>
              ) : null}

              <footer className="council-footer">
                {game.heroState ? (
                  <div className="hero-note-card wide">
                    <strong>你的身份：{game.heroState.role || '未知'}</strong>
                    <ul>
                      {(game.heroState.notes ?? []).map((note, index) => <li key={index}>{note}</li>)}
                    </ul>
                  </div>
                ) : (
                  <div className="spectator-note-card">
                    <strong>观战模式</strong>
                    <p>你现在看到的是完整公共事件流；座位角色与回放细节会随着模式和阶段实时开放。</p>
                  </div>
                )}
              </footer>
            </section>

            <aside className="council-sidebar">
              <article className={`turn-status-card ${turnStatus.tone}`}>
                <div className="decision-header">
                  <strong>{turnStatus.title}</strong>
                  <span>{turnStatus.label}</span>
                </div>
                <p>{turnStatus.detail}</p>
              </article>

              {latestSpeech ? <article className="latest-action-card">
                <div className="decision-header">
                  <strong>最近发言</strong>
                  <span>{phaseLabel(asText(asRecord(latestSpeech.payload).phase) || game.phase)}</span>
                </div>
                <p>{describeSpeechEvent(latestSpeech)}</p>
              </article> : null}

              {latestAIAction ? <article className="latest-thought-card">
                <div className="decision-header">
                  <strong>最近模型动作</strong>
                  <span>{asText(asRecord(latestAIAction.payload).model) || 'AI'}</span>
                </div>
                <p>{describeAITurn(latestAIAction)}</p>
              </article> : null}

              <div className="action-panel">
                <h3>当前行动</h3>
                {game.pendingAction ? <PendingActionView pending={game.pendingAction} /> : <p>当前无待处理动作。</p>}
                {renderActionForm(game.pendingAction, game.mode === 'spectator', {
                  acting,
                  speechText,
                  setSpeechText,
                  selectedTarget,
                  setSelectedTarget,
                  useHeal,
                  setUseHeal,
                  poisonTarget,
                  setPoisonTarget,
                  onSubmit: handleAction,
                })}
              </div>

              <div className="event-panel">
                <div className="panel-head-inline">
                  <h3>实时事件</h3>
                  {game.winnerSide ? <span className="winner-pill">{game.winnerSide}</span> : null}
                </div>
                <ol className="event-list readable">
                  {[...game.events].reverse().map((event) => (
                    <EventTimelineItem key={event.sequence} event={event} />
                  ))}
                </ol>
              </div>
            </aside>
            </>
            })()}
          </>}
        </section>
      ) : null}

      {!loading && tab === 'history' ? (
        <section className="panel">
          <div className="panel-head-inline">
            <h2>历史记录</h2>
            <button onClick={() => fetchRecords().then(setRecords).catch((err: Error) => setError(err.message))}>刷新</button>
          </div>
          <div className="record-list">
            {records.map((record) => (
              <article key={record.id} className="record-card">
                <div>
                  <strong>{record.templateName}</strong>
                  <div>{record.mode === 'spectator' ? '纯 AI 观战' : '1 真人参与'}</div>
                  <div>状态：{record.status} · Day {record.day}</div>
                  <div>{record.winnerSide ? `胜方：${record.winnerSide}` : '尚未结束'}</div>
                </div>
                <div className="record-actions">
                  <button onClick={() => refreshGame(record.id).then(() => setTab('room'))}>打开房间</button>
                  <button onClick={() => handleOpenReplay(record.id)}>查看回放</button>
                </div>
              </article>
            ))}
            {records.length === 0 ? <div className="empty">还没有历史记录。</div> : null}
          </div>
        </section>
      ) : null}

      {!loading && tab === 'replay' ? (
        <section className="panel room-layout replay-layout-shell">
          {!replay ? <div className="empty">先从历史记录里打开一局。</div> : <>
            {(() => {
              const stepEvents = currentReplayStep ? replay.events.slice(0, selectedReplayStepIndex + 1) : replay.events
              const seatStyles = seatLayout(replay.players.length)
              const seatNotes = buildSeatNoteMap(stepEvents)
              const currentReplayBubble = currentReplayStep ? buildReplaySeatBubble(currentReplayStep) : null
              const themedPhase = currentReplayStep?.phase ?? inferLatestReplayPhase(replay.events)
              return <>
            <section className="council-room replay-room">
              <header className="council-room-header">
                <div className="council-room-id">
                  <span className="match-eyebrow">REPLAY</span>
                  <strong>{replay.summary.templateName}</strong>
                  <span className="muted-text">{new Date(replay.summary.createdAt).toLocaleString('zh-CN')}</span>
                </div>
                <div className="council-room-meta">
                  <span className="winner-pill subtle-pill">Day {replay.summary.day}</span>
                  {replay.summary.winnerSide ? <span className="winner-pill">{replay.summary.winnerSide}</span> : null}
                </div>
              </header>

              <div className="council-stage">
                <div className={`council-table replay-table ${phaseThemeClass(themedPhase)}`}>
                  <div className="council-table-rim" aria-hidden="true" />
                  <div className="council-table-felt">
                    <div className="council-table-logo" aria-hidden="true">
                      REPLAY<span className="logo-moon">☾</span>
                    </div>
                    <div className="council-table-center">
                      <span className="table-phase-chip">POST GAME</span>
                      <strong>{currentReplayStep ? currentReplayStep.title : replay.summary.winnerSide ? `胜方：${replay.summary.winnerSide}` : '对局回放'}</strong>
                      <span>{currentReplayStep ? currentReplayStep.detail : `${replay.summary.templateName} · Day ${replay.summary.day}`}</span>
                      <span className="table-center-note">回放里按步骤查看整局事件与各座位状态。</span>
                    </div>
                  </div>
                  {replay.players.map((player, index) => (
                    <WerewolfSeat
                      key={`replay-${player.seat}`}
                      style={seatStyles[index]}
                      name={player.name}
                      seat={player.seat}
                      subtitle={player.isHuman ? '真人玩家' : `AI · ${presetSubtitle(presetByID.get(player.presetId ?? '')) || player.presetId || 'scripted'}`}
                      roleText={roleLabel(player.role)}
                      revealedRole={player.role}
                      note={seatNotes.get(player.seat)}
                      isAlive={player.alive}
                      isActive={currentReplayStep?.actorSeat === player.seat}
                      isHuman={player.isHuman}
                      bubble={currentReplayBubble?.seat === player.seat ? { title: currentReplayBubble.title, detail: currentReplayBubble.detail } : null}
                      bubbleDirection={seatStyles[index]?.bubbleDirection}
                    />
                  ))}
                </div>
              </div>
            </section>

            <aside className="council-sidebar replay-sidebar">
              <div className="panel-head-inline">
                <h3>当前步骤</h3>
                <span className="winner-pill subtle-pill">{replaySteps.length} 步</span>
              </div>

              {currentReplayStep ? (
                <article className={`turn-status-card ${currentReplayStep.tone}`}>
                  <div className="decision-header">
                    <strong>{currentReplayStep.title}</strong>
                    <span>#{selectedReplayStepIndex + 1}</span>
                  </div>
                  <p>{currentReplayStep.detail}</p>
                </article>
              ) : null}

              <div className="replay-step-controls">
                <button className="ghost-button" onClick={() => setSelectedReplayStepIndex(0)} disabled={selectedReplayStepIndex <= 0}>第一步</button>
                <button className="ghost-button" onClick={() => setSelectedReplayStepIndex((current) => Math.max(current - 1, 0))} disabled={selectedReplayStepIndex <= 0}>上一步</button>
                <button className="ghost-button" onClick={() => setSelectedReplayStepIndex((current) => Math.min(current + 1, Math.max(replaySteps.length - 1, 0)))} disabled={selectedReplayStepIndex >= replaySteps.length - 1}>下一步</button>
                <button className="ghost-button" onClick={() => setSelectedReplayStepIndex(Math.max(replaySteps.length - 1, 0))} disabled={selectedReplayStepIndex >= replaySteps.length - 1}>最后一步</button>
              </div>

              <div className="action-feed replay-step-list">
                {replaySteps.map((step, index) => (
                  <button className={`feed-item replay-step-button ${index === selectedReplayStepIndex ? 'selected' : ''}`} key={`${step.event.sequence}-${index}`} onClick={() => setSelectedReplayStepIndex(index)} type="button">
                    <div>
                      <strong>{step.title}</strong>
                      <small>{step.detail}</small>
                    </div>
                    <span>#{index + 1}</span>
                  </button>
                ))}
              </div>

              <ol className="event-list replay readable">
                {replay.events.map((event) => (
                  <EventTimelineItem key={event.sequence} event={event} />
                ))}
              </ol>
            </aside>
            </>
            })()}
          </>}
        </section>
      ) : null}
    </div>
  )
}

function updateAI(index: number, value: string, setSelectedAI: Dispatch<SetStateAction<string[]>>) {
  setSelectedAI((current) => {
    const next = [...current]
    next[index] = value
    return next
  })
}

function roleLabel(role: string) {
  switch (role) {
    case 'werewolf':
      return '狼人'
    case 'seer':
      return '预言家'
    case 'witch':
      return '女巫'
    case 'hunter':
      return '猎人'
    case 'guard':
      return '守卫'
    default:
      return '平民'
  }
}

function presetSubtitle(preset?: Preset) {
  if (!preset) return 'scripted'
  if (preset.model) return preset.model
  if (preset.style) return preset.style
  return 'scripted'
}

function phaseLabel(phase: string) {
  switch (phase) {
    case 'night':
      return '夜晚开始'
    case 'night_wolf':
      return '狼人夜刀'
    case 'night_seer':
      return '预言家查验'
    case 'night_witch':
      return '女巫行动'
    case 'night_guard':
      return '守卫行动'
    case 'day_main':
      return '主发言'
    case 'day_reply':
      return '回应轮'
    case 'day_vote':
      return '投票'
    case 'hunter_shot':
      return '猎人开枪'
    default:
      return phase
  }
}

function roomSeatRoleText(game: Snapshot, player: Snapshot['players'][number]) {
  if (game.mode === 'spectator') return player.role || player.revealedRole || ''
  if (player.isHuman) return game.heroState?.role || ''
  return player.revealedRole || ''
}

function buildSeatNoteMap(events: Snapshot['events'] | ReplayDetail['events']) {
  const notes = new Map<number, string>()
  for (const event of events) {
    const payload = asRecord(event.payload)
    const seat = asNumber(payload.seat)
    if (seat === null) continue
    if (event.type === 'vote_cast') {
      const targetName = asText(payload.targetName)
      if (targetName) notes.set(seat, `投票给 ${targetName}`)
      continue
    }
    if (event.type === 'player_eliminated') {
      const reason = asText(payload.reason)
      notes.set(seat, reason ? `因 ${reason} 出局` : '已出局')
      continue
    }
    if (event.type === 'seer_checked') {
      const targetName = asText(payload.targetName)
      const result = asText(payload.result)
      if (targetName && result) notes.set(seat, `验 ${targetName} = ${result}`)
    }
  }
  return notes
}

function buildLatestSpeechBubble(events: Snapshot['events'] | ReplayDetail['events']) {
  for (let index = events.length - 1; index >= 0; index -= 1) {
    const event = events[index]
    if (event?.type !== 'speech_recorded') continue
    const payload = asRecord(event.payload)
    const seat = asNumber(payload.seat)
    const text = asText(payload.text)
    if (seat === null || !text) continue
    return {
      seat,
      title: phaseLabel(asText(payload.phase) || 'day_main'),
      detail: text,
    }
  }
  return null
}

function buildReplaySeatBubble(step: ReplayStep) {
  if (step.actorSeat === null) return null
  return {
    seat: step.actorSeat,
    title: step.title,
    detail: step.detail,
  }
}

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' ? (value as Record<string, unknown>) : {}
}

function asText(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

function asNumber(value: unknown): number | null {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  return null
}

function PendingActionView({ pending }: { pending: PendingAction }) {
  return (
    <div className="pending-box">
      <strong>{pending.prompt}</strong>
      {pending.options?.length ? <span>可选目标：{pending.options.map((item) => item.label).join('、')}</span> : null}
    </div>
  )
}

function EventTimelineItem({ event }: { event: Snapshot['events'][number] | ReplayDetail['events'][number] }) {
  const entry = formatEventEntry(event)
  return (
    <li className={`timeline-item ${entry.tone}`}>
      <div className="timeline-item-head">
        <strong>#{event.sequence} · {entry.title}</strong>
        <span>{new Date(event.timestamp).toLocaleString('zh-CN')}</span>
      </div>
      <p>{entry.detail}</p>
      {entry.meta.length ? <div className="timeline-item-meta">{entry.meta.map((item) => <span key={item}>{item}</span>)}</div> : null}
      <details>
        <summary>查看原始 payload</summary>
        <pre>{JSON.stringify(event.payload, null, 2)}</pre>
      </details>
    </li>
  )
}

function formatEventEntry(event: Snapshot['events'][number] | ReplayDetail['events'][number]) {
  const payload = asRecord(event.payload)
  const seat = asNumber(payload.seat)
  const playerName = asText(payload.playerName)
  const targetName = asText(payload.targetName)
  const summary = asText(payload.summary)
  switch (event.type) {
    case 'phase_started':
      return {
        title: `进入${phaseLabel(asText(payload.phase))}`,
        detail: `Day ${asNumber(payload.day) ?? 0} 开始推进到 ${phaseLabel(asText(payload.phase))}。`,
        meta: [event.visibility],
        tone: 'is-phase',
      }
    case 'speech_recorded':
      return {
        title: `${playerName || `${seat ?? '?'}号`}发言`,
        detail: asText(payload.text) || '本次发言为空。',
        meta: [`${phaseLabel(asText(payload.phase))}`, event.visibility],
        tone: 'is-speech',
      }
    case 'vote_cast':
      return {
        title: `${playerName || `${seat ?? '?'}号`}投票`,
        detail: targetName ? `本轮投给 ${targetName}。` : '已提交投票。',
        meta: [event.visibility],
        tone: 'is-vote',
      }
    case 'vote_resolved':
      return {
        title: '投票结算',
        detail: summary || (targetName ? `本轮放逐 ${targetName}。` : '本轮无人出局。'),
        meta: [event.visibility],
        tone: 'is-vote',
      }
    case 'player_eliminated':
      return {
        title: `${playerName || `${seat ?? '?'}号`}出局`,
        detail: `${asText(payload.reason) || '未知原因'} · 翻牌 ${asText(payload.role) || '未公开'}`,
        meta: [event.visibility],
        tone: 'is-danger',
      }
    case 'night_resolved':
      return {
        title: '夜晚结算',
        detail: summary || '夜晚结算完成。',
        meta: [event.visibility],
        tone: 'is-night',
      }
    case 'seer_checked':
      return {
        title: `${playerName || `${seat ?? '?'}号`}完成查验`,
        detail: `${targetName || '目标'} 的结果是 ${asText(payload.result) || '未知'}。`,
        meta: [event.visibility],
        tone: 'is-private',
      }
    case 'witch_action_recorded':
      return {
        title: '女巫行动',
        detail: buildWitchDetail(payload),
        meta: [event.visibility],
        tone: 'is-private',
      }
    case 'wolf_target_selected':
      return {
        title: `${playerName || `${seat ?? '?'}号`}提交刀口`,
        detail: targetName ? `选择了 ${targetName}。` : '提交了刀口。',
        meta: [event.visibility],
        tone: 'is-private',
      }
    case 'wolf_target_locked':
      return {
        title: '狼队锁定刀口',
        detail: targetName ? `最终刀口：${targetName}` : '狼队已锁定今晚刀口。',
        meta: [event.visibility],
        tone: 'is-private',
      }
    case 'game_finished':
      return {
        title: '对局结束',
        detail: summary || '胜负已结算。',
        meta: [event.visibility],
        tone: 'is-finish',
      }
    case 'game_created':
      return {
        title: '对局创建',
        detail: `新建了 ${asText(payload.template) || '当前'}，模式：${asText(payload.mode) || '未知'}。`,
        meta: [event.visibility],
        tone: 'is-generic',
      }
    case 'ai_turn_generated':
      return {
        title: `${playerName || `${seat ?? '?'}号`}完成模型决策`,
        detail: `模型 ${asText(payload.model)} 生成了 ${asText(payload.kind) || '当前'} 动作。`,
        meta: [event.visibility],
        tone: 'is-ai',
      }
    case 'ai_fallback_used':
      return {
        title: `${playerName || `${seat ?? '?'}号`}回退脚本`,
        detail: asText(payload.error) || '模型动作失败，已回退本地脚本。',
        meta: [event.visibility],
        tone: 'is-danger',
      }
    default:
      return {
        title: event.type,
        detail: summary || '查看原始 payload 获取更多信息。',
        meta: [event.visibility],
        tone: 'is-generic',
      }
  }
}

function buildWitchDetail(payload: Record<string, unknown>) {
  const useHeal = payload.useHeal === true
  const usePoison = payload.usePoison === true
  const healTarget = asNumber(payload.healTarget)
  const poisonTarget = asNumber(payload.poisonTarget)
  const parts: string[] = []
  parts.push(useHeal ? `使用了解药（目标 ${healTarget ?? '?'} 号）` : '没有使用解药')
  parts.push(usePoison ? `使用了毒药（目标 ${poisonTarget ?? '?'} 号）` : '没有使用毒药')
  return parts.join('，')
}

function buildReplaySteps(replay: ReplayDetail | null): ReplayStep[] {
  if (!replay) return []
  const steps: ReplayStep[] = []
  let currentPhase = 'night'
  for (const event of replay.events) {
    const payload = asRecord(event.payload)
    if (event.type === 'phase_started') {
      currentPhase = asText(payload.phase) || currentPhase
    }
    const entry = formatEventEntry(event)
    steps.push({
      index: steps.length,
      event,
      title: entry.title,
      detail: entry.detail,
      tone: entry.tone,
      actorSeat: asNumber(payload.seat),
      phase: currentPhase,
    })
  }
  return steps
}

function inferLatestReplayPhase(events: ReplayDetail['events']) {
  for (let i = events.length - 1; i >= 0; i -= 1) {
    const event = events[i]
    if (event?.type === 'phase_started') {
      return asText(asRecord(event.payload).phase) || 'night'
    }
  }
  return 'night'
}

function describeWerewolfTurnStatus(game: Snapshot, currentActorName: string | null) {
  if (game.status === 'finished') {
    return {
      label: '对局结束',
      title: game.winnerSide ? `胜方：${game.winnerSide}` : '比赛已结束',
      detail: '这局已经打完，可以去回放里按步骤查看整局过程。',
      tone: 'finished',
    }
  }
  if (game.status === 'stopped' || game.control?.stopped) {
    return {
      label: '已终止',
      title: '比赛已手动终止',
      detail: '当前这桌不会继续推进。',
      tone: 'stopped',
    }
  }
  if (game.mode === 'spectator' && game.control?.paused) {
    return {
      label: '已暂停',
      title: currentActorName ? `当前停在 ${currentActorName}` : '观战已暂停',
      detail: game.control?.semiAutoMode ? '点击“继续下一天”后，系统会从当前夜晚继续推进到下一次投票结算。' : '点击“继续阶段”后，系统才会继续推进。',
      tone: 'paused',
    }
  }
  if (game.mode === 'spectator' && game.control?.manualMode && !game.control?.running) {
    return {
      label: '手动模式',
      title: currentActorName ? `等待 ${currentActorName} 的下一步动作` : '等待下一步',
      detail: '当前停在单个 AI 行动边界，点击“下一步”后才会再走一步。',
      tone: 'manual',
    }
  }
  if (game.mode === 'spectator' && game.control?.semiAutoMode && !game.control?.running) {
    return {
      label: '半自动停点',
      title: currentActorName ? `当前停在 ${phaseLabel(game.phase)}，下一步由 ${currentActorName}` : '当前天已结算',
      detail: '当前是半自动观战：每天投票结算后停下，点击“继续下一天”才会进入下一轮。',
      tone: 'semi',
    }
  }
  if (game.status === 'awaiting_human') {
    return {
      label: '等待你',
      title: '现在轮到你操作',
      detail: '右侧操作区已经可用；你提交后，牌局会继续推进。',
      tone: 'human',
    }
  }
  if (game.mode === 'spectator') {
    return {
      label: game.control?.semiAutoMode ? '半自动推进中' : game.control?.manualMode ? '手动观战' : '全自动推进中',
      title: currentActorName ? `当前由 ${currentActorName} 行动` : '系统正在推进',
      detail: spectatorDescFor(game),
      tone: 'ai',
    }
  }
  return {
    label: '等待 AI',
    title: currentActorName ? `当前等待 ${currentActorName}` : '系统正在推进',
    detail: '系统正在替其他座位完成动作或切换到下一阶段。',
    tone: 'progress',
  }
}

function spectatorDescFor(game: Snapshot) {
  if (game.control?.manualMode) {
    return '当前是手动逐步观战：每次 AI 动作都需要你点“下一步”。'
  }
  if (game.control?.semiAutoMode) {
    return '当前是半自动观战：系统会自动跑完整个夜晚和白天，直到当天投票结算后再停下。'
  }
  return '当前是全自动观战：系统会持续推进直到下一次手动暂停或比赛结束。'
}

function describeSpeechEvent(event: Snapshot['events'][number]) {
  const payload = asRecord(event.payload)
  const playerName = asText(payload.playerName) || `${asNumber(payload.seat) ?? '?'}号`
  const text = asText(payload.text)
  return `${playerName}：${text || '（空发言）'}`
}

function describeAITurn(event: Snapshot['events'][number]) {
  const payload = asRecord(event.payload)
  const playerName = asText(payload.playerName) || `${asNumber(payload.seat) ?? '?'}号`
  const kind = asText(payload.kind) || '动作'
  if (kind === 'target') {
    return `${playerName} 已完成选目标决策。`
  }
  if (kind === 'witch') {
    return `${playerName} 已完成女巫药剂决策。`
  }
  return `${playerName} 已生成发言内容。`
}

function phaseThemeClass(phase: string) {
  if (phase.startsWith('night')) return 'theme-night'
  if (phase === 'day_vote') return 'theme-vote'
  if (phase.startsWith('day')) return 'theme-day'
  if (phase === 'hunter_shot') return 'theme-vote'
  return 'theme-night'
}

function renderActionForm(
  pending: PendingAction | undefined,
  spectatorMode: boolean,
  state: {
    acting: boolean
    speechText: string
    setSpeechText: (value: string) => void
    selectedTarget: string
    setSelectedTarget: (value: string) => void
    useHeal: boolean
    setUseHeal: (value: boolean) => void
    poisonTarget: string
    setPoisonTarget: (value: string) => void
    onSubmit: (payload: { action: string; text?: string; targetSeat?: number; useHeal?: boolean; usePoison?: boolean }) => void
  },
) {
  if (!pending) return null
  if (spectatorMode) {
    return null
  }
  if (pending.kind === 'speech') {
    return (
      <div className="form-stack">
        <textarea value={state.speechText} placeholder={pending.placeholder} onChange={(event) => state.setSpeechText(event.target.value)} />
        <button className="primary" disabled={state.acting || !state.speechText.trim()} onClick={() => state.onSubmit({ action: 'speech', text: state.speechText })}>提交发言</button>
      </div>
    )
  }
  if (pending.kind === 'select') {
    return (
      <div className="form-stack">
        <select value={state.selectedTarget} onChange={(event) => state.setSelectedTarget(event.target.value)}>
          {(pending.options ?? []).map((option) => (
            <option key={option.value} value={option.value}>{option.label}</option>
          ))}
        </select>
        <button className="primary" disabled={state.acting || state.selectedTarget === ''} onClick={() => state.onSubmit({ action: 'select', targetSeat: Number(state.selectedTarget) })}>提交目标</button>
      </div>
    )
  }
  if (pending.kind === 'witch') {
    return (
      <div className="form-stack">
        <label className="checkbox-row">
          <input type="checkbox" checked={state.useHeal} disabled={!pending.allowHeal} onChange={(event) => state.setUseHeal(event.target.checked)} />
          <span>使用解药</span>
        </label>
        <select value={state.poisonTarget} onChange={(event) => state.setPoisonTarget(event.target.value)}>
          <option value="">不使用毒药</option>
          {(pending.options ?? []).map((option) => (
            <option key={option.value} value={option.value}>{option.label}</option>
          ))}
        </select>
        <div className="action-row">
          <button disabled={state.acting} onClick={() => state.onSubmit({ action: 'pass' })}>今晚不动药</button>
          <button
            className="primary"
            disabled={state.acting || (!state.useHeal && state.poisonTarget === '')}
            onClick={() =>
              state.onSubmit({
                action: 'witch',
                useHeal: state.useHeal,
                usePoison: state.poisonTarget !== '',
                targetSeat: state.poisonTarget ? Number(state.poisonTarget) : undefined,
              })
            }
          >
            提交药剂
          </button>
        </div>
      </div>
    )
  }
  return null
}
