import { useRef } from 'react'
import { exportUrl, type Rule, type Ruleset } from './api'

export const updateSet = (sets: Ruleset[], i: number, patch: Partial<Ruleset>) =>
  sets.map((set, j) => (j === i ? { ...set, ...patch } : set))

export const updateRule = (sets: Ruleset[], i: number, k: number, patch: Partial<Rule>) =>
  updateSet(sets, i, { rules: sets[i].rules.map((rule, j) => (j === k ? { ...rule, ...patch } : rule)) })

export const hitKey = (set: string, rule?: string) => (rule === undefined ? set : `${set}\u0000${rule}`)

export function Hits({ n }: { n?: number }) {
  return n ? <span className="hits" title={`${n} replacement(s) in the current text`}>{n}</span> : null
}

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
    <div className="rules-editor">
      <div className="toolbar">
        <button
          onClick={() => setDraft([...draft, { name: `My ruleset ${draft.length + 1}`, enabled: true, rules: [newRule()] }])}
          title="Add an empty ruleset (a bundle you can switch on/off as a whole) for your own words and regexes."
        >
          + New ruleset
        </button>
        {missingPresets.length > 0 && (
          <select
            value=""
            onChange={(e) => setDraft([...draft, ...missingPresets.filter((p) => p.name === e.target.value)])}
            title="Bring back a factory preset you deleted earlier."
          >
            <option value="">+ Add factory preset…</option>
            {missingPresets.map((p) => <option key={p.name}>{p.name}</option>)}
          </select>
        )}
        <span className="spacer" />
        <a
          className="button secondary"
          href={exportUrl}
          download="memento-rulesets.yml"
          title="Download the SAVED rulesets as YAML - a backup you can import again or edit in your own editor. Unsaved changes are not included."
        >
          Export saved (YAML)
        </a>
        <button
          className="secondary"
          onClick={() => fileInput.current?.click()}
          disabled={busy}
          title="Replace ALL rulesets with a YAML/JSON file exported earlier. It is validated and saved immediately."
        >
          Import file…
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
        <button
          className="danger"
          onClick={onReset}
          disabled={busy}
          title="Throw away ALL rulesets, including your own words, and restore the factory presets. Asks for confirmation; export first to keep a copy."
        >
          Reset all to factory defaults
        </button>
      </div>

      {error && <p className="error-banner" role="alert">Rule error - fix it before saving: {error}</p>}

      {draft.map((set, i) => {
        const preset = defaults.find((p) => p.name === set.name)
        return (
          <section key={i} className={`ruleset ${set.enabled ? '' : 'off'}`}>
            <div className="ruleset-header">
              <input
                type="checkbox"
                className="switch"
                checked={set.enabled}
                onChange={(e) => setDraft(updateSet(draft, i, { enabled: e.target.checked }))}
                title="Switch the whole ruleset on or off. Its rules keep their own on/off state."
                aria-label={`Ruleset ${set.name} enabled`}
              />
              <input
                className="ruleset-name"
                value={set.name}
                onChange={(e) => setDraft(updateSet(draft, i, { name: e.target.value }))}
                title="Ruleset name. Keep a factory preset's name to be able to restore it later."
                aria-label="Ruleset name"
              />
              <Hits n={hits.get(hitKey(set.name))} />
              <span className="spacer" />
              {preset && (
                <button
                  className="secondary small"
                  onClick={() => setDraft(updateSet(draft, i, preset))}
                  title="Replace this ruleset with its factory version. Other rulesets are not touched. Unsaved until you click Save rules."
                >
                  Restore preset
                </button>
              )}
              <button
                className="danger small"
                onClick={() => setDraft(draft.filter((_, j) => j !== i))}
                title="Delete this ruleset. Unsaved until you click Save rules."
              >
                Delete ruleset
              </button>
            </div>
            <input
              className="description"
              value={set.description ?? ''}
              placeholder="Description (shown as a hover hint)"
              onChange={(e) => setDraft(updateSet(draft, i, { description: e.target.value }))}
              aria-label="Ruleset description"
            />
            <div className="table-scroll">
              <table className="rules">
                <thead>
                  <tr>
                    <th title="Rule on/off">On</th>
                    <th title="Rule name and an optional description shown as a hint.">Name</th>
                    <th title="words: literal words/phrases, case-insensitive, whole words only. regex: Go RE2 regular expressions.">Type</th>
                    <th title="One word/phrase or regex per line. Paste many lines at once to bulk-add.">Patterns (one per line)</th>
                    <th title="Replacement text. # becomes a number unique per distinct value: [HOST_#] gives [HOST_1], [HOST_2]… Without # every value gets the same fixed text.">Placeholder</th>
                    <th title="Optional regex. Matched values that also match it are left untouched, e.g. private IP ranges.">Keep if matches</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {set.rules.map((rule, k) => {
                    const change = (patch: Partial<Rule>) => setDraft(updateRule(draft, i, k, patch))
                    return (
                      <tr key={k} className={rule.enabled ? '' : 'off'}>
                        <td>
                          <input
                            type="checkbox"
                            className="switch small"
                            checked={rule.enabled}
                            onChange={(e) => change({ enabled: e.target.checked })}
                            title="Switch this rule on or off."
                            aria-label={`Rule ${rule.name} enabled`}
                          />
                        </td>
                        <td>
                          <input value={rule.name} onChange={(e) => change({ name: e.target.value })} title={rule.description || 'Rule name'} aria-label="Rule name" />
                          <input
                            className="description"
                            value={rule.description ?? ''}
                            placeholder="description"
                            onChange={(e) => change({ description: e.target.value })}
                            aria-label="Rule description"
                          />
                          <Hits n={hits.get(hitKey(set.name, rule.name))} />
                        </td>
                        <td>
                          <select
                            value={rule.type}
                            onChange={(e) => change({ type: e.target.value as Rule['type'] })}
                            title="words = literal text (safe for any characters). regex = regular expression; a capture group ( ) redacts only that part."
                            aria-label="Rule type"
                          >
                            <option value="words">words</option>
                            <option value="regex">regex</option>
                          </select>
                        </td>
                        <td>
                          <textarea
                            className="mono"
                            wrap="off"
                            rows={Math.min(Math.max(rule.patterns.length, 1), 8)}
                            value={rule.patterns.join('\n')}
                            onChange={(e) => change({ patterns: e.target.value.split('\n') })}
                            placeholder={rule.type === 'words' ? 'web01\nProject Phoenix' : '\\bSRV-\\d{3}\\b'}
                            spellCheck={false}
                            title={rule.type === 'words' ? 'One word or phrase per line.' : 'One regex per line (Go RE2: no lookarounds). (?i) = ignore case.'}
                            aria-label="Patterns"
                          />
                        </td>
                        <td>
                          <input
                            className="mono"
                            value={rule.placeholder}
                            onChange={(e) => change({ placeholder: e.target.value })}
                            title="# becomes a number per distinct value. The same value always gets the same placeholder."
                            aria-label="Placeholder"
                          />
                        </td>
                        <td>
                          <input
                            className="mono"
                            value={rule.keep ?? ''}
                            placeholder="optional regex"
                            onChange={(e) => change({ keep: e.target.value })}
                            title="Values matching this regex are NOT redacted, e.g. ^10\. keeps 10.x.x.x addresses."
                            aria-label="Keep regex"
                          />
                        </td>
                        <td>
                          <button
                            className="danger small"
                            onClick={() => setDraft(updateSet(draft, i, { rules: set.rules.filter((_, j) => j !== k) }))}
                            title="Delete this rule. Unsaved until you click Save rules."
                            aria-label={`Delete rule ${rule.name}`}
                          >
                            ✕
                          </button>
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
            <button
              className="secondary small"
              onClick={() => setDraft(updateSet(draft, i, { rules: [...set.rules, newRule()] }))}
              title="Add a rule to this ruleset. Start with words for exact names, switch to regex for patterns."
            >
              + Add rule
            </button>
          </section>
        )
      })}

      <details className="help">
        <summary title="Short guide to writing rules">How rules work</summary>
        <ul>
          <li>A <b>ruleset</b> is a bundle of rules. Toggle a whole bundle or single rules. Edits show up in the preview at once; click <b>Save rules</b> to store them on the server - the CLI and proxy only use saved rules.</li>
          <li><b>words</b>: literal words or phrases, one per line, case-insensitive, whole words only (<code>web01</code> does not match <code>web010</code>).</li>
          <li><b>regex</b>: Go RE2 syntax (no lookarounds). With a capture group only the group is redacted: <code>password=(\S+)</code> gives <code>password=[SECRET_1]</code>. Group without capturing via <code>(?:…)</code>; <code>(?i)</code> ignores case.</li>
          <li><b>placeholder</b>: <code>#</code> becomes a number per distinct value (<code>[HOST_1]</code>, <code>[HOST_2]</code>); a value repeated in the text keeps its number. Without <code>#</code> all values get the same fixed text, e.g. <code>[DOMAIN].[TLD]</code>.</li>
          <li><b>keep</b>: a regex tested against each matched value; matching values stay as they are (the Network presets keep private IPs this way).</li>
          <li>When matches of different rules overlap, the longest one wins.</li>
        </ul>
      </details>
    </div>
  )
}
