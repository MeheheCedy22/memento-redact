import { useRef } from 'react'
import AutoTextarea from './AutoTextarea'
import { exportUrl, type Rule, type Ruleset } from './api'

export const updateSet = (sets: Ruleset[], i: number, patch: Partial<Ruleset>) =>
  sets.map((set, j) => (j === i ? { ...set, ...patch } : set))

export const updateRule = (sets: Ruleset[], i: number, k: number, patch: Partial<Rule>) =>
  updateSet(sets, i, { rules: sets[i].rules.map((rule, j) => (j === k ? { ...rule, ...patch } : rule)) })

export const hitKey = (set: string, rule?: string) => (rule === undefined ? set : `${set}\u0000${rule}`)

export function Count({ n }: { n?: number }) {
  return n ? <span className="count" title="Replacements in the current text">{n}</span> : null
}

// Single-line fields are wrapping textareas so long values stay fully visible; Enter must not add a line.
const oneLine = (value: string) => value.replace(/\r?\n/g, '')

const newRule = (): Rule => ({ name: 'New rule', enabled: true, type: 'words', patterns: [], placeholder: '[NAME_#]' })

type Props = {
  draft: Ruleset[]
  setDraft: (sets: Ruleset[]) => void
  defaults: Ruleset[]
  hits: Map<string, number>
  error: string
  busy: boolean
  onReset: () => void
  onImport: (yaml: string) => void
}

export default function RulesEditor({ draft, setDraft, defaults, hits, error, busy, onReset, onImport }: Props) {
  const fileInput = useRef<HTMLInputElement>(null)
  const missingPresets = defaults.filter((preset) => !draft.some((set) => set.name === preset.name))

  return (
    <main className="rules-page">
      <div className="toolbar">
        <button onClick={() => setDraft([...draft, { name: `Ruleset ${draft.length + 1}`, enabled: true, rules: [newRule()] }])}>
          New ruleset
        </button>
        {missingPresets.length > 0 && (
          <select value="" onChange={(e) => setDraft([...draft, ...missingPresets.filter((p) => p.name === e.target.value)])} title="Add back a deleted factory preset.">
            <option value="">Add preset…</option>
            {missingPresets.map((p) => <option key={p.name}>{p.name}</option>)}
          </select>
        )}
        <span className="spacer" />
        <a className="button" href={exportUrl} download="memento-rulesets.yml" title="Download the saved rules as YAML. Unsaved changes are not included.">
          Export
        </a>
        <button onClick={() => fileInput.current?.click()} disabled={busy} title="Replace all rules with a YAML or JSON file. Saved immediately.">
          Import…
        </button>
        <input
          ref={fileInput}
          type="file"
          accept=".yml,.yaml,.json"
          hidden
          onChange={async (e) => {
            const file = e.target.files?.[0]
            e.target.value = ''
            if (file) onImport(await file.text())
          }}
        />
        <button className="danger" onClick={onReset} disabled={busy} title="Replace all rules, including your own, with the factory presets.">
          Reset all
        </button>
      </div>

      {error && <p className="error" role="alert">{error}</p>}

      {draft.map((set, i) => {
        const preset = defaults.find((p) => p.name === set.name)
        return (
          <section key={i} className={`ruleset ${set.enabled ? '' : 'off'}`}>
            <div className="ruleset-head">
              <input
                type="checkbox"
                checked={set.enabled}
                onChange={(e) => setDraft(updateSet(draft, i, { enabled: e.target.checked }))}
                aria-label={`Ruleset ${set.name} on`}
                title="Ruleset on/off"
              />
              <input className="ruleset-name" value={set.name} onChange={(e) => setDraft(updateSet(draft, i, { name: e.target.value }))} aria-label="Ruleset name" />
              <Count n={hits.get(hitKey(set.name))} />
              <span className="spacer" />
              {preset && preset.preset !== set.preset && (
                <span className="note" title="The factory version changed. Restore preset to get it; it replaces your edits in this ruleset.">
                  Update available
                </span>
              )}
              {preset && (
                <button onClick={() => setDraft(updateSet(draft, i, preset))} title="Replace this ruleset with its factory version.">
                  Restore preset
                </button>
              )}
              <button className="danger" onClick={() => setDraft(draft.filter((_, j) => j !== i))}>
                Delete
              </button>
            </div>
            <AutoTextarea
              className="description"
              rows={1}
              value={set.description ?? ''}
              placeholder="Description"
              onChange={(e) => setDraft(updateSet(draft, i, { description: oneLine(e.target.value) }))}
              aria-label="Ruleset description"
            />

            <div className="rule rule-labels" aria-hidden="true">
              <span />
              <span>Name</span>
              <span title="words: literal text, case-insensitive, whole words. regex: Go RE2.">Type</span>
              <span>Patterns, one per line</span>
              <span title="# becomes a number per distinct value.">Placeholder</span>
              <span title="Matches of this regex are not redacted.">Keep</span>
              <span />
            </div>
            {set.rules.map((rule, k) => {
              const change = (patch: Partial<Rule>) => setDraft(updateRule(draft, i, k, patch))
              return (
                <div key={k} className={`rule ${rule.enabled ? '' : 'off'}`}>
                  <input type="checkbox" checked={rule.enabled} onChange={(e) => change({ enabled: e.target.checked })} aria-label={`Rule ${rule.name} on`} title="Rule on/off" />
                  <div className="stack">
                    <AutoTextarea rows={1} value={rule.name} onChange={(e) => change({ name: oneLine(e.target.value) })} aria-label="Rule name" />
                    <AutoTextarea className="description" rows={1} value={rule.description ?? ''} placeholder="Description" onChange={(e) => change({ description: oneLine(e.target.value) })} aria-label="Rule description" />
                    <Count n={hits.get(hitKey(set.name, rule.name))} />
                  </div>
                  <select value={rule.type} onChange={(e) => change({ type: e.target.value as Rule['type'] })} aria-label="Type">
                    <option value="words">words</option>
                    <option value="regex">regex</option>
                  </select>
                  <AutoTextarea
                    className="mono"
                    rows={1}
                    value={rule.patterns.join('\n')}
                    onChange={(e) => change({ patterns: e.target.value.split('\n') })}
                    placeholder={rule.type === 'words' ? 'web01\nProject Phoenix' : '\\bSRV-\\d{3}\\b'}
                    spellCheck={false}
                    aria-label="Patterns"
                  />
                  <div className="stack">
                    <input className="mono" value={rule.placeholder} onChange={(e) => change({ placeholder: e.target.value })} aria-label="Placeholder" />
                    {rule.placeholder.includes('.') && (
                      <label className="option" title="Number each dot-separated part by its own value: acme.sk → [DOMAIN_1].[TLD_1], acme.com → [DOMAIN_1].[TLD_2]. Off: every value gets the same placeholder.">
                        <input type="checkbox" checked={rule.per_part ?? false} onChange={(e) => change({ per_part: e.target.checked })} />
                        Number each part
                      </label>
                    )}
                  </div>
                  <AutoTextarea className="mono" rows={1} value={rule.keep ?? ''} placeholder="regex" onChange={(e) => change({ keep: oneLine(e.target.value) })} aria-label="Keep regex" />
                  <button className="danger" onClick={() => setDraft(updateSet(draft, i, { rules: set.rules.filter((_, j) => j !== k) }))} aria-label={`Delete rule ${rule.name}`} title="Delete rule">
                    ✕
                  </button>
                </div>
              )
            })}
            <button onClick={() => setDraft(updateSet(draft, i, { rules: [...set.rules, newRule()] }))}>Add rule</button>
          </section>
        )
      })}

      <details className="help">
        <summary>How rules work</summary>
        <ul>
          <li>Edits apply to the preview at once. <b>Save</b> stores them for the CLI and proxy.</li>
          <li><b>words</b>: literal text, one per line, case-insensitive, whole words only.</li>
          <li><b>regex</b>: Go RE2, no lookarounds. With a group <code>( )</code> only the group is replaced: <code>password=(\S+)</code> → <code>password=[SECRET_1]</code>.</li>
          <li><b>Placeholder</b>: <code>#</code> becomes a number per distinct value; repeats keep their number. Without <code>#</code> every value gets the same text.</li>
          <li><b>Number each part</b> (placeholders with dots): each part is numbered by its own value, so <code>acme.sk</code> → <code>[DOMAIN_1].[TLD_1]</code> and <code>acme.com</code> → <code>[DOMAIN_1].[TLD_2]</code>.</li>
          <li><b>Keep</b>: matched values that also match this regex stay as they are.</li>
          <li>When matches overlap, the longest wins.</li>
          <li>Unedited presets update automatically after an upgrade. Edited ones show <i>Update available</i>.</li>
        </ul>
      </details>
    </main>
  )
}
