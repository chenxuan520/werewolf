export type Preset = {
  id: string
  name: string
  style?: string
  persona?: string
  endpoint?: string
  model?: string
}

export type Template = {
  id: string
  name: string
  seats: number
  roles: string[]
}

export type ActionOption = {
  value: string
  label: string
}

export type PendingAction = {
  kind: string
  actorSeat: number
  prompt: string
  options?: ActionOption[]
  allowText?: boolean
  placeholder?: string
  allowPass?: boolean
  allowHeal?: boolean
}

export type EventItem = {
  sequence: number
  type: string
  visibility: string
  timestamp: string
  payload: unknown
}

export type Player = {
  seat: number
  name: string
  alive: boolean
  isHuman: boolean
  presetId?: string
  role?: string
  revealedRole?: string
}

export type HeroState = {
  role?: string
  notes?: string[]
}

export type ControlState = {
  spectatorMode: boolean
  semiAutoMode: boolean
  paused: boolean
  manualMode: boolean
  canStep: boolean
  stopped: boolean
  running: boolean
}

export type Snapshot = {
  id: string
  status: string
  mode: string
  template: Template
  day: number
  phase: string
  message: string
  players: Player[]
  heroSeat?: number
  heroState?: HeroState
  control: ControlState
  pendingAction?: PendingAction
  events: EventItem[]
  winnerSide?: string
  createdAt: string
  updatedAt: string
  finishedAt?: string
}

export type RecordSummary = {
  id: string
  status: string
  mode: string
  templateName: string
  day: number
  winnerSide?: string
  createdAt: string
  updatedAt: string
  finishedAt?: string
}

export type ReplayPlayer = {
  seat: number
  name: string
  role: string
  alive: boolean
  isHuman: boolean
  presetId?: string
}

export type ReplayDetail = {
  summary: RecordSummary
  players: ReplayPlayer[]
  events: EventItem[]
  template: Template
  createdAt: string
}

export type CreateGamePayload = {
  templateId: string
  spectatorMode: boolean
  semiAutoMode?: boolean
  manualMode?: boolean
  humanName?: string
  aiPresetIds: string[]
}

export type ControlPayload = {
  action: string
}

export type ActionPayload = {
  action: string
  text?: string
  targetSeat?: number
  useHeal?: boolean
  usePoison?: boolean
}
