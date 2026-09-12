// 浏览器端麦克风录音 + WAV 编码。
// 采集麦克风 PCM，停止时下采样到 16k 单声道 16bit，编码成 WAV 并返回 base64，
// 交给后端 /api/transcribe（参考 voice2text 的 mimo provider：wav base64 + input_audio）。

const TARGET_RATE = 16000
const MAX_DURATION_MS = 60_000

type VoiceRecorder = {
  stop: () => Promise<string>
  cancel: () => void
}

export function isRecordingSupported(): boolean {
  return typeof navigator !== 'undefined' && !!navigator.mediaDevices?.getUserMedia && typeof window !== 'undefined' && !!(window.AudioContext || (window as unknown as { webkitAudioContext?: unknown }).webkitAudioContext)
}

export async function startRecording(): Promise<VoiceRecorder> {
  if (!isRecordingSupported()) {
    throw new Error('当前浏览器不支持麦克风录音')
  }
  const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
  const AudioCtx = window.AudioContext || (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext
  const audioContext = new AudioCtx()
  const source = audioContext.createMediaStreamSource(stream)
  const processor = audioContext.createScriptProcessor(4096, 1, 1)
  // 用一个静音 gain 把 processor 接到 destination，保证它持续触发 onaudioprocess，
  // 又不会把麦克风声音外放造成回声。
  const silentGain = audioContext.createGain()
  silentGain.gain.value = 0

  const chunks: Float32Array[] = []
  let length = 0
  let stopped = false
  const startedAt = Date.now()

  processor.onaudioprocess = (event) => {
    if (stopped) return
    const input = event.inputBuffer.getChannelData(0)
    chunks.push(new Float32Array(input))
    length += input.length
    if (Date.now() - startedAt >= MAX_DURATION_MS) {
      stopped = true
    }
  }

  source.connect(processor)
  processor.connect(silentGain)
  silentGain.connect(audioContext.destination)

  const cleanup = () => {
    stopped = true
    try {
      processor.disconnect()
      silentGain.disconnect()
      source.disconnect()
    } catch {
      // ignore
    }
    stream.getTracks().forEach((track) => track.stop())
    void audioContext.close().catch(() => undefined)
  }

  return {
    async stop() {
      const inputRate = audioContext.sampleRate
      cleanup()
      const merged = mergeChunks(chunks, length)
      const downsampled = downsample(merged, inputRate, TARGET_RATE)
      const wav = encodeWav(downsampled, TARGET_RATE)
      return bytesToBase64(wav)
    },
    cancel() {
      cleanup()
    },
  }
}

function mergeChunks(chunks: Float32Array[], length: number): Float32Array {
  const out = new Float32Array(length)
  let offset = 0
  for (const chunk of chunks) {
    out.set(chunk, offset)
    offset += chunk.length
  }
  return out
}

function downsample(input: Float32Array, inputRate: number, targetRate: number): Float32Array {
  if (targetRate >= inputRate) return input
  const ratio = inputRate / targetRate
  const outLength = Math.floor(input.length / ratio)
  const out = new Float32Array(outLength)
  for (let i = 0; i < outLength; i += 1) {
    // 用区间平均做简单抗混叠，比直接取点更少毛刺。
    const start = Math.floor(i * ratio)
    const end = Math.min(input.length, Math.floor((i + 1) * ratio))
    let sum = 0
    let count = 0
    for (let j = start; j < end; j += 1) {
      sum += input[j] ?? 0
      count += 1
    }
    out[i] = count > 0 ? sum / count : 0
  }
  return out
}

function encodeWav(samples: Float32Array, sampleRate: number): Uint8Array {
  const bytesPerSample = 2
  const blockAlign = bytesPerSample // mono
  const dataSize = samples.length * bytesPerSample
  const buffer = new ArrayBuffer(44 + dataSize)
  const view = new DataView(buffer)

  writeAscii(view, 0, 'RIFF')
  view.setUint32(4, 36 + dataSize, true)
  writeAscii(view, 8, 'WAVE')
  writeAscii(view, 12, 'fmt ')
  view.setUint32(16, 16, true)
  view.setUint16(20, 1, true) // PCM
  view.setUint16(22, 1, true) // mono
  view.setUint32(24, sampleRate, true)
  view.setUint32(28, sampleRate * blockAlign, true)
  view.setUint16(32, blockAlign, true)
  view.setUint16(34, 16, true) // bits
  writeAscii(view, 36, 'data')
  view.setUint32(40, dataSize, true)

  let offset = 44
  for (let i = 0; i < samples.length; i += 1) {
    const s = Math.max(-1, Math.min(1, samples[i] ?? 0))
    view.setInt16(offset, s < 0 ? s * 0x8000 : s * 0x7fff, true)
    offset += 2
  }
  return new Uint8Array(buffer)
}

function writeAscii(view: DataView, offset: number, text: string) {
  for (let i = 0; i < text.length; i += 1) {
    view.setUint8(offset + i, text.charCodeAt(i))
  }
}

function bytesToBase64(bytes: Uint8Array): string {
  let binary = ''
  const chunkSize = 0x8000
  for (let i = 0; i < bytes.length; i += chunkSize) {
    const chunk = bytes.subarray(i, i + chunkSize)
    binary += String.fromCharCode(...chunk)
  }
  return btoa(binary)
}
