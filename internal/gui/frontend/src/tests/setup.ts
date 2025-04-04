// src/tests/setup.ts
import { beforeAll, afterAll, vi } from 'vitest';

// Mock browser globals
global.window = {
  EventsOn: vi.fn(),
  go: {
    gui: {
      App: {
        RecordWasmLog: vi.fn(),
        RecordWasmState: vi.fn()
      }
    }
  }
} as any;

// Mock EventsOn
vi.stubGlobal('EventsOn', vi.fn());

// Mock console methods to reduce test noise
const originalConsole = { ...console };
beforeAll(() => {
  console.debug = vi.fn();
  console.info = vi.fn();
  console.warn = vi.fn();
  // Keep console.error for debugging test failures
});

// Restore console methods after tests
afterAll(() => {
  console.debug = originalConsole.debug;
  console.info = originalConsole.info;
  console.warn = originalConsole.warn;
});

// Mock localStorage
const localStorageMock = (() => {
  let store: Record<string, string> = {};
  return {
    getItem: (key: string) => store[key] || null,
    setItem: (key: string, value: string) => { store[key] = value.toString(); },
    removeItem: (key: string) => { delete store[key]; },
    clear: () => { store = {}; }
  };
})();

vi.stubGlobal('localStorage', localStorageMock);

// Mock WebAssembly API
vi.stubGlobal('WebAssembly', {
  instantiate: vi.fn(),
  compile: vi.fn(),
  Memory: vi.fn(),
  Module: vi.fn()
});