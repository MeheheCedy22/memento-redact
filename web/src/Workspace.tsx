import { useState, type ReactNode } from 'react'
import AutoTextarea from './AutoTextarea'
import type { MapEntry, PreviewResult, Ruleset } from './api'
import { Count, hitKey, updateRule, updateSet } from './RulesEditor'

const example = `PS> Get-VM | Select Name, PowerState, @{N='IP';E={$_.Guest.IPAddress -join ', '}}

Name                    PowerState IP
----                    ---------- --
web01.prod.domain.tld   PoweredOn  10.20.30.41, fe80::250:56ff:fe9a:1b2c
db01.prod.domain.tld    PoweredOn  10.20.30.42
edge-gw.domain.tld      PoweredOn  192.168.1.1, 203.0.113.17
vpn.domain.tld          PoweredOn  198.51.100.7, 2001:db8:85a3::8a2e:370:7334

PS> Connect-VIServer vcenter.domain.tld -User admin@domain.tld -Password 'S3cr3t!'
JIRA_PASS=SVSIUIUFNOE890i0-x
Owner: jane.doe@domain.tld, +420 777 123 456
`

const escapeRegExp = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

// Marks every placeholder in the redacted text; hovering one shows what it replaced.
function Highlighted({ text, map }: { text: string; map: MapEntry[] }) {
  const byToken = new Map<string, MapEntry[]>()
  for (const entry of map) byToken.set(entry.token, [...(byToken.get(entry.token) ?? []), entry])
  if (byToken.size === 0) return <>{text}</>

  const pattern = new RegExp([...byToken.keys()].sort((a, b) => b.length - a.length).map(escapeRegExp).join('|'), 'g')
  const parts: ReactNode[] = []
  let last = 0
  for (const match of text.matchAll(pattern)) {
    const at = match.index ?? 0
    const entries = byToken.get(match[0]) ?? []
    parts.push(text.slice(last, at))
    parts.push(
      <mark key={at} title={entries.map((e) => `${e.original}\n${e.ruleset} › ${e.rule}`).join('\n\n')}>
        {match[0]}
      </mark>,
    )
    last = at + match[0].length
  }
  parts.push(text.slice(last))
  return <>{parts}</>
}

type Props = {
  text: string
  setText: (text: string) => void
  result: PreviewResult | null
  upToDate: boolean
  previewError: string
  draft: Ruleset[]
  setDraft: (sets: Ruleset[]) => void
  hits: Map<string, number>
}

export default function Workspace({ text, setText, result, upToDate, previewError, draft, setDraft, hits }: Props) {
  const [copyNote, setCopyNote] = useState('')
  const replacements = result?.map.reduce((sum, entry) => sum + entry.count, 0) ?? 0

  async function copy() {
    if (!result) return
    if (!navigator.clipboard) {
      setCopyNote('Copying needs HTTPS or localhost. Select the text and press Ctrl+C.')
      return
    }
    await navigator.clipboard.writeText(result.redacted)
    setCopyNote('Copied')
    setTimeout(() => setCopyNote(''), 2000)
  }

  return (
    <main className="workspace">
      {/* one scroll area for both columns, so long texts stay side by side and compact */}
      <div className="pair">
        <section>
          <div className="section-head">
            <h2>Original</h2>
            <button onClick={() => setText(example)} title="Replace the text with an example.">Example</button>
            <button onClick={() => setText('')} title="Clear the text.">Clear</button>
          </div>
          <AutoTextarea
            className="mono"
            rows={12}
            value={text}
            onChange={(e) => setText(e.target.value)}
            placeholder="Paste text here. It is redacted as you type and never stored."
            spellCheck={false}
            aria-label="Original text"
          />
        </section>

        <section>
          <div className="section-head">
            <h2>Redacted</h2>
            <span className="muted" title="Replacements made in the current text.">
              {previewError ? '' : upToDate ? `${replacements} replaced` : 'updating…'}
            </span>
            {copyNote && <span className="note">{copyNote}</span>}
            <button
              className="primary"
              onClick={copy}
              disabled={!upToDate || !result?.redacted}
              title="Copy the redacted text. Disabled while it is out of date or a rule has an error."
            >
              Copy
            </button>
          </div>
          {previewError && <p className="error" role="alert">Not redacted: {previewError}</p>}
          <div className={`output mono ${upToDate ? '' : 'stale'}`} aria-label="Redacted text" aria-live="polite">
            {result?.redacted ? <Highlighted text={result.redacted} map={result.map} /> : <span className="muted">The redacted text appears here.</span>}
          </div>
        </section>
      </div>

      <div className="below">
        <section>
          <h2>Rulesets</h2>
          <div className="scroll">
            {draft.map((set, i) => (
              <fieldset key={i} className={set.enabled ? '' : 'off'}>
                <legend>
                  <label title={set.description}>
                    <input type="checkbox" checked={set.enabled} onChange={(e) => setDraft(updateSet(draft, i, { enabled: e.target.checked }))} />
                    {set.name}
                  </label>
                  <Count n={hits.get(hitKey(set.name))} />
                </legend>
                {set.rules.map((rule, k) => (
                  <label key={k} title={rule.description}>
                    <input type="checkbox" checked={rule.enabled} onChange={(e) => setDraft(updateRule(draft, i, k, { enabled: e.target.checked }))} />
                    {rule.name}
                    <Count n={hits.get(hitKey(set.name, rule.name))} />
                  </label>
                ))}
              </fieldset>
            ))}
          </div>
        </section>

        <section>
          <h2>Map</h2>
          {result?.map.length ? (
            <div className="scroll">
              <table className="map">
                <thead>
                  <tr>
                    <th>Placeholder</th>
                    <th>Original</th>
                    <th>Rule</th>
                    <th title="Occurrences">#</th>
                  </tr>
                </thead>
                <tbody>
                  {result.map.map((entry) => (
                    <tr key={entry.token + '\u0000' + entry.original}>
                      <td className="mono">{entry.token}</td>
                      <td className="mono">{entry.original}</td>
                      <td>{entry.ruleset} › {entry.rule}</td>
                      <td>{entry.count}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <p className="muted">Each placeholder and the value it replaced.</p>
          )}
        </section>
      </div>
    </main>
  )
}
