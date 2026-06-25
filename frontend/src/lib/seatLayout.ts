export type SeatStyle = {
  top: string
  left: string
  bubbleDirection: 'up' | 'down'
}

const RADIUS_X = 35
const RADIUS_Y_BOTTOM = 34
const RADIUS_Y_TOP = 44
const TOP_MIN = 15
const CENTER_X = 50
const CENTER_Y = 50
const START_DEGREES = 90

export function seatLayout(count: number): SeatStyle[] {
  if (count <= 0) return []
  const step = 360 / count
  const layouts: SeatStyle[] = []
  for (let i = 0; i < count; i += 1) {
    const angleDeg = START_DEGREES + step * i
    const angleRad = (angleDeg * Math.PI) / 180
    const sinValue = Math.sin(angleRad)
    const isUpper = sinValue < 0
    const radiusY = isUpper ? RADIUS_Y_TOP : RADIUS_Y_BOTTOM
    const x = CENTER_X + RADIUS_X * Math.cos(angleRad)
    let y = CENTER_Y + radiusY * sinValue
    if (isUpper) y = Math.max(y, TOP_MIN)
    layouts.push({ top: `${y}%`, left: `${x}%`, bubbleDirection: isUpper ? 'down' : 'up' })
  }
  return layouts
}
