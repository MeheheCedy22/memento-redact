import type { MapEntry, Ruleset } from './api'

// Reasons a placeholder can be wrong, shown in the map in debug mode; the user can save more.
export const reasons = new Map([
  ['false positive', 'should not be redacted'],
  ['too much', 'the redaction covers more than the sensitive part'],
  ['too little', 'part of the sensitive value stays visible'],
  ['wrong type', 'right to redact, but the wrong rule or placeholder'],
  ['wrong number', 'one value got several numbers, or several values share one'],
  ['other', 'something else'],
])
// reason is a key of reasons or one of the user's own reasons; note is an optional description
export type Mark = { entry: MapEntry; reason: string; note?: string }

// The user's own reasons live in this browser only.
export function savedReasons(): string[] {
  try {
    const list: unknown = JSON.parse(localStorage.getItem('debugReasons') ?? '[]')
    return Array.isArray(list) ? list.filter((r): r is string => typeof r === 'string') : []
  } catch {
    return []
  }
}

export function saveReasons(list: string[]) {
  try {
    localStorage.setItem('debugReasons', JSON.stringify(list))
  } catch {
    // storage unavailable (private mode): the reasons last until reload
  }
}

export const escapeRegExp = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

// debugReport describes the marked placeholders and the missed texts, with up to 3 lines each occurs in.
// Only the marked values are restored in those lines, so everything else stays redacted.
export function debugReport(redacted: string, map: MapEntry[], marks: Mark[], missed: string[], sets: Ruleset[]): string {
  const lines = redacted.split('\n')
  // a token that stands for several values (fixed placeholders) cannot be restored per occurrence
  const restore = new Map(
    marks.filter(({ entry }) => map.filter((e) => e.token === entry.token).length === 1).map(({ entry }) => [entry.token, entry.original]),
  )
  const tokens = new RegExp([...restore.keys()].sort((a, b) => b.length - a.length).map(escapeRegExp).join('|') || '$^', 'g')
  const context = (s: string) =>
    lines
      .filter((line) => line.includes(s))
      .slice(0, 3)
      .flatMap((line) => {
        const unredacted = line.replace(tokens, (t) => restore.get(t) ?? t)
        return unredacted === line ? [`   line:       ${line}`] : [`   redacted:   ${line}`, `   unredacted: ${unredacted}`]
      })

  const out = ['memento-redact debug report', '']
  let n = 0
  for (const { entry, reason, note } of marks) {
    const rule = sets.find((s) => s.name === entry.ruleset)?.rules.find((r) => r.name === entry.rule)
    const meaning = reasons.has(reason) ? ` (${reasons.get(reason)})` : ''
    out.push(`${++n}. ${reason}${meaning}: ${entry.token} = ${JSON.stringify(entry.original)}, ${entry.count}x`)
    if (note?.trim()) out.push(`   note: ${note.trim()}`)
    out.push(`   rule: ${entry.ruleset} › ${entry.rule}` + (rule ? `, ${rule.type}, placeholder ${rule.placeholder}${rule.per_part ? ', number each part' : ''}` : ''))
    // a words list is your list of sensitive names, so only regexes go into the report
    if (rule?.type === 'regex') {
      for (const pattern of rule.patterns) out.push(`   pattern: ${pattern}`)
      if (rule.keep) out.push(`   keep: ${rule.keep}`)
    }
    out.push(...context(entry.token), '')
  }
  for (const text of missed) {
    out.push(`${++n}. false negative (not redacted): ${JSON.stringify(text)}`, ...context(text), '')
  }
  return out.join('\n').trimEnd() + '\n'
}
