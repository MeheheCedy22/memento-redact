import type { MapEntry } from './api'
import { escapeRegExp } from './debug.ts'

// A piece of the restored answer: plain text, a restored value, or a placeholder that could not be restored.
export type Piece =
  | { kind: 'plain'; text: string }
  | { kind: 'restored'; text: string; token: string } // token as written in the map
  | { kind: 'ambiguous'; text: string; values: string[] }
  | { kind: 'unknown'; text: string }

// values maps every placeholder (lower case) to the values it stands for. A per-part token like
// [DOMAIN_1].[TLD_1] for acme.sk also gives [DOMAIN_1] -> acme and [TLD_1] -> sk, since an LLM may use the parts alone.
function values(map: MapEntry[]): Map<string, { token: string; values: Set<string> }> {
  const out = new Map<string, { token: string; values: Set<string> }>()
  const add = (token: string, value: string) => {
    const known = out.get(token.toLowerCase()) ?? { token, values: new Set<string>() }
    out.set(token.toLowerCase(), { token: known.token, values: known.values.add(value) })
  }
  for (const { token, original } of map) {
    add(token, original)
    const parts = token.split('.')
    const labels = original.split('.')
    if (parts.length < 2 || labels.length < parts.length || !parts.every((p) => /^\[[^\]]+\]$/.test(p))) continue
    // extra leading labels belong to the first part, as in the server's per-part numbering
    const extra = labels.length - parts.length
    const split = [labels.slice(0, extra + 1).join('.'), ...labels.slice(extra + 1)]
    parts.forEach((part, i) => add(part, split[i]))
  }
  return out
}

// restore replaces the placeholders of map in answer with the values they stand for. Matching ignores case and
// Markdown-escaped brackets (\[HOST_1\]). A placeholder that stands for several values, or a [NAME_9]-like one
// that is not in the map, is left as it is and reported.
export function restore(answer: string, map: MapEntry[]): Piece[] {
  const known = values(map)
  const escaped = [...known.keys()]
    .sort((a, b) => b.length - a.length)
    .map((t) => escapeRegExp(t).replace(/\\\[/g, '\\\\?\\[').replace(/\\\]/g, '\\\\?\\]'))
  const pattern = new RegExp([...escaped, '\\\\?\\[[A-Z][A-Z0-9_]*\\\\?\\]'].join('|'), 'gi')
  const pieces: Piece[] = []
  let last = 0
  for (const match of answer.matchAll(pattern)) {
    const at = match.index ?? 0
    if (at > last) pieces.push({ kind: 'plain', text: answer.slice(last, at) })
    const hit = known.get(match[0].replace(/\\/g, '').toLowerCase())
    const found = [...(hit?.values ?? [])]
    if (hit && found.length === 1) pieces.push({ kind: 'restored', text: found[0], token: hit.token })
    else if (found.length > 1) pieces.push({ kind: 'ambiguous', text: match[0], values: found })
    else pieces.push({ kind: 'unknown', text: match[0] })
    last = at + match[0].length
  }
  if (last < answer.length) pieces.push({ kind: 'plain', text: answer.slice(last) })
  return pieces
}
