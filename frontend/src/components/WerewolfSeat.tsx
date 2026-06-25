import type { CSSProperties } from 'react'

type SeatBubble = {
  title: string
  detail?: string
}

type Props = {
  style: CSSProperties
  name: string
  seat: number
  subtitle: string
  roleText?: string
  revealedRole?: string
  note?: string
  isAlive: boolean
  isActive: boolean
  isHuman: boolean
  bubble?: SeatBubble | null
  bubbleDirection?: 'up' | 'down'
}

export function WerewolfSeat({ style, name, seat, subtitle, roleText, revealedRole, note, isAlive, isActive, isHuman, bubble, bubbleDirection = 'up' }: Props) {
  const classes = ['council-seat', isAlive ? '' : 'is-dead', isActive ? 'is-active' : '', isHuman ? 'is-human' : 'is-ai']
    .filter(Boolean)
    .join(' ')

  return (
    <article className={classes} style={style} data-testid={isActive ? 'current-turn-seat' : undefined} data-bubble-direction={bubble ? bubbleDirection : undefined}>
      {bubble ? (
        <div className="council-seat-bubble">
          <strong>{bubble.title}</strong>
          {bubble.detail ? <span>{bubble.detail}</span> : null}
        </div>
      ) : null}
      <div className="council-seat-body">
        <div className="council-seat-avatar" aria-hidden="true">
          <span>{avatarInitial(name)}</span>
          {isActive ? <span className="council-seat-ring" /> : null}
        </div>
        <div className="council-seat-info">
          <div className="council-seat-head">
            <strong className="council-seat-name">{name}</strong>
            <span className="council-seat-number">#{seat}</span>
          </div>
          <div className="council-seat-subtitle">{subtitle}</div>
          <div className="council-seat-tags">
            <span className={`council-seat-tag ${isAlive ? 'is-alive' : 'is-dead'}`}>{isAlive ? '存活' : '出局'}</span>
            {isHuman ? <span className="council-seat-tag is-human">YOU</span> : null}
            {roleText ? <span className="council-seat-tag is-role">{roleText}</span> : null}
            {!roleText && revealedRole ? <span className="council-seat-tag is-revealed">翻牌 {revealedRole}</span> : null}
          </div>
          <div className={`council-seat-note ${note ? '' : 'is-empty'}`}>{note || '等待本轮公开信息'}</div>
        </div>
      </div>
    </article>
  )
}

function avatarInitial(name: string) {
  const trimmed = (name || '').trim()
  if (!trimmed) return '?'
  return trimmed[0]!.toUpperCase()
}
