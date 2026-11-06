import type { ReactNode, TextareaHTMLAttributes } from 'react'
import AutoTextarea from './AutoTextarea'
import type { Leak, MapEntry } from './api'
import { escapeRegExp } from './debug'

// A run of text, optionally wrapped (e.g. in a <mark>) when shown.
export type Part = { text: string; wrap?: (text: string, key: number) => ReactNode }

// NumberedText shows parts as numbered lines. Each line is its own block, so a long line wraps and keeps
// its number on the first row; a part that spans lines is split at the line breaks.
export function NumberedText({ parts }: { parts: Part[] }) {
  const lines: ReactNode[][] = [[]]
  let key = 0
  for (const part of parts) {
    part.text.split('\n').forEach((piece, i) => {
      if (i > 0) lines.push([])
      if (piece) lines[lines.length - 1].push(part.wrap ? part.wrap(piece, key++) : piece)
    })
  }
  return (
    <div className="lines">
      {lines.map((line, i) => (
        <div key={i}>{line}</div>
      ))}
    </div>
  )
}

// NumberedTextarea draws the same text, invisible, behind a transparent textarea. The copy wraps exactly
// like the textarea, so the line numbers drawn next to it stay at the right rows.
export function NumberedTextarea(props: TextareaHTMLAttributes<HTMLTextAreaElement> & { value: string }) {
  return (
    <div className="numbered">
      <div className={`lines ${props.className ?? ''}`} aria-hidden="true">
        {props.value.split('\n').map((line, i) => (
          <div key={i}>{line}</div>
        ))}
      </div>
      <AutoTextarea {...props} />
    </div>
  )
}

// cut splits text at every match of pattern; wrap gives how each match is shown.
function cut(text: string, pattern: RegExp, wrap: (match: string) => Part['wrap']): Part[] {
  const parts: Part[] = []
  let last = 0
  for (const match of text.matchAll(pattern)) {
    const at = match.index ?? 0
    parts.push({ text: text.slice(last, at) }, { text: match[0], wrap: wrap(match[0]) })
    last = at + match[0].length
  }
  parts.push({ text: text.slice(last) })
  return parts
}

// longest first, so [DOMAIN_1].[TLD_1] wins over [DOMAIN_1]
const anyOf = (values: string[], flags: string) => new RegExp([...values].sort((a, b) => b.length - a.length).map(escapeRegExp).join('|'), flags)

// Splits the redacted text into parts and marks every placeholder; hovering one shows what it replaced.
// Leaks, values redacted elsewhere but still in the text, are outlined.
export function highlight(text: string, map: MapEntry[], leaks: Leak[] = []): Part[] {
  const byToken = new Map<string, MapEntry[]>()
  for (const entry of map) byToken.set(entry.token, [...(byToken.get(entry.token) ?? []), entry])
  let parts: Part[] = [{ text }]
  if (byToken.size > 0) {
    parts = cut(text, anyOf([...byToken.keys()], 'g'), (token) => {
      const title = (byToken.get(token) ?? []).map((e) => `${e.original}\n${e.ruleset} › ${e.rule}`).join('\n\n')
      return (t, key) => <mark key={key} data-token={token} title={title}>{t}</mark>
    })
  }
  if (leaks.length > 0) {
    const byValue = new Map(leaks.map((l) => [l.value.toLowerCase(), l]))
    const pattern = anyOf([...byValue.keys()], 'gi')
    parts = parts.flatMap((part) =>
      part.wrap
        ? [part]
        : cut(part.text, pattern, (value) => {
            const leak = byValue.get(value.toLowerCase())
            const title = `Not redacted here, but redacted elsewhere by ${leak?.ruleset} › ${leak?.rule}`
            return (t, key) => <mark key={key} className="leak" title={title}>{t}</mark>
          }),
    )
  }
  return parts
}
