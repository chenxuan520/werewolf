import '@testing-library/jest-dom/vitest'

class MockEventSource {
  url: string
  onerror: (() => void) | null = null
  constructor(url: string) {
    this.url = url
  }
  addEventListener() {}
  close() {}
}

Object.defineProperty(globalThis, 'EventSource', {
  writable: true,
  value: MockEventSource,
})
