export type Rule = {
  name: string
  description?: string
  enabled: boolean
  type: 'words' | 'regex'
  patterns: string[]
  keep?: string
  placeholder: string
  // number each dot-separated placeholder part by its own value
  per_part?: boolean
  // redact a match only if it looks generated: 16+ characters, letters and digits mixed, no words
  random?: boolean
  // lines the Rules page redacts with this rule alone while editing; redaction ignores them
  samples?: string[]
}

export type Ruleset = {
  name: string
  description?: string
  enabled: boolean
  rules: Rule[]
  // fingerprint of the factory version a preset was copied from
  preset?: string
}

export type MapEntry = {
  token: string
  original: string
  ruleset: string
  rule: string
  count: number
  // the rule's placeholder and the numbers put into it; sent back as known, they keep numbering going
  placeholder: string
  numbers?: number[]
}

// a rule that works but probably redacts too much or too little
export type Warning = { ruleset: string; rule: string; message: string }

// a value that is redacted somewhere but still appears in the redacted text as it is
export type Leak = { value: string; ruleset: string; rule: string; count: number }

export type PreviewResult = {
  redacted: string
  map: MapEntry[]
  warnings?: Warning[]
  leaks?: Leak[]
}

// Same origin: the Go server serves the UI; `npm run dev` proxies these paths (vite.config.ts).
export const exportUrl = '/v1/rulesets?format=yaml'

async function call<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, init)
  if (!response.ok) {
    throw new Error((await response.text()).trim() || `${response.status} ${response.statusText}`)
  }
  return response.json() as Promise<T>
}

export const api = {
  rulesets: () => call<Ruleset[]>('/v1/rulesets'),
  defaults: () => call<Ruleset[]>('/v1/rulesets/defaults'),
  reset: () => call<Ruleset[]>('/v1/rulesets/reset', { method: 'POST' }),
  save: (sets: Ruleset[]) =>
    call<Ruleset[]>('/v1/rulesets', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(sets) }),
  // checks a rules file (YAML or JSON) and returns its rulesets without saving them
  parse: (file: string) => call<Ruleset[]>('/v1/rulesets/parse', { method: 'POST', headers: { 'Content-Type': 'application/yaml' }, body: file }),
  // known is the conversation so far: its values keep their placeholders
  preview: (text: string, rulesets: Ruleset[], known: MapEntry[], signal: AbortSignal) =>
    call<PreviewResult>('/v1/preview', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ text, rulesets, known }),
      signal,
    }),
}

export function errorText(error: unknown) {
  return error instanceof Error ? error.message : String(error)
}
