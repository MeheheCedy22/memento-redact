import type { Profile } from './rulesets'

// Per-browser settings (saved reasons, profiles). Storage can be missing or full, e.g. in private mode:
// then load returns fallback and store keeps nothing, so the value lasts until reload.
function load(key: string, fallback: unknown): unknown {
  try {
    const raw = localStorage.getItem(key)
    return raw === null ? fallback : JSON.parse(raw)
  } catch {
    return fallback
  }
}

function store(key: string, value: unknown) {
  try {
    localStorage.setItem(key, JSON.stringify(value))
  } catch {
    // see above
  }
}

// The user's own debug reasons.
export function savedReasons(): string[] {
  const list = load('debugReasons', [])
  return Array.isArray(list) ? list.filter((r): r is string => typeof r === 'string') : []
}

export const saveReasons = (list: string[]) => store('debugReasons', list)

export function savedProfiles(): Profile[] {
  const list = load('profiles', [])
  return Array.isArray(list) ? list.filter((p): p is Profile => typeof p?.name === 'string' && Array.isArray(p?.switches)) : []
}

export const saveProfiles = (list: Profile[]) => store('profiles', list)
