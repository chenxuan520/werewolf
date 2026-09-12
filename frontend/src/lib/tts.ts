// 浏览器内置的免费文本转语音（Web Speech API / speechSynthesis）。
// 不需要任何 key、不产生费用、离线可用。用于把 AI 发言朗读出来增强代入感。
//
// 关键：多条发言要按顺序排队逐条朗读，不能后一条把前一条打断（否则只有最后一句有声音）。

export function isSpeechSupported(): boolean {
  return typeof window !== 'undefined' && 'speechSynthesis' in window && typeof window.SpeechSynthesisUtterance !== 'undefined'
}

let cachedZhVoice: SpeechSynthesisVoice | null | undefined

function pickZhVoice(): SpeechSynthesisVoice | null {
  if (cachedZhVoice !== undefined) return cachedZhVoice
  const voices = window.speechSynthesis.getVoices()
  cachedZhVoice = voices.find((v) => v.lang?.toLowerCase().startsWith('zh')) ?? null
  return cachedZhVoice
}

// 语音列表在部分浏览器里是异步加载的，监听一次把缓存清空以便重新挑选。
if (isSpeechSupported()) {
  window.speechSynthesis.onvoiceschanged = () => {
    cachedZhVoice = undefined
  }
}

export type SpeechItem = {
  speaker: string
  text: string
}

// 当前朗读音量（0~1），可由 UI 调节。
let currentVolume = 1
// 待朗读队列 + 是否正在播放，保证多条发言逐条读完不互相打断。
const queue: SpeechItem[] = []
let speaking = false
// UI 订阅“当前正在朗读谁”，用于在界面上高亮/展示说话人。
let nowSpeaking: SpeechItem | null = null
let listener: ((item: SpeechItem | null) => void) | null = null

export function onSpeakingChange(cb: ((item: SpeechItem | null) => void) | null) {
  listener = cb
}

function setNowSpeaking(item: SpeechItem | null) {
  nowSpeaking = item
  listener?.(item)
}

export function setSpeechVolume(volume: number) {
  currentVolume = Math.max(0, Math.min(1, volume))
}

function playNext() {
  if (speaking) return
  const item = queue.shift()
  if (item === undefined) {
    setNowSpeaking(null)
    return
  }
  speaking = true
  setNowSpeaking(item)
  const utterance = new SpeechSynthesisUtterance(item.text)
  utterance.lang = 'zh-CN'
  utterance.rate = 1.05
  utterance.pitch = 1
  utterance.volume = currentVolume
  const voice = pickZhVoice()
  if (voice) utterance.voice = voice
  const advance = () => {
    speaking = false
    playNext()
  }
  utterance.onend = advance
  utterance.onerror = advance
  window.speechSynthesis.speak(utterance)
}

// 把一条发言加入朗读队列（按调用顺序逐条朗读）。
export function speak(text: string, speaker = '') {
  if (!isSpeechSupported()) return
  const trimmed = text.trim()
  if (!trimmed) return
  // 队列过长时只保留较新的几条，避免观战快速推进时语音严重滞后牌面。
  if (queue.length > 8) queue.splice(0, queue.length - 8)
  queue.push({ speaker, text: trimmed })
  playNext()
}

export function stopSpeaking() {
  if (!isSpeechSupported()) return
  queue.length = 0
  speaking = false
  setNowSpeaking(null)
  window.speechSynthesis.cancel()
}

export function currentSpeaking(): SpeechItem | null {
  return nowSpeaking
}
