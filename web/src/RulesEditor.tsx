import { useEffect, useRef, useState } from 'react'
import AutoTextarea from './AutoTextarea'
import { highlight, NumberedText, NumberedTextarea, type Part } from './Numbered'
import { api, errorText, exportUrl, type PreviewResult, type Rule, type Ruleset } from './api'
import { mergeRulesets } from './rulesets'

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

// [syntax, meaning] rows of the regex cheat sheet on the Rules page; keep in sync with docs/rules.md.
const cheatSheet: [string, string][] = [
  ['(?i)', 'at the start: ignore upper and lower case'],
  ['.', 'any character except a line break'],
  ['\\.  \\(  \\[  \\\\', 'a literal dot, bracket, backslash: put \\ before any of . * + ? ( ) [ ] { } | ^ $ \\'],
  ['\\d  \\w  \\s', 'a digit · a letter, digit or _ · whitespace, including line breaks. Uppercase means "not": \\S is not whitespace'],
  ['\\p{L}', 'any letter, including á, č, ž (\\w is ASCII only)'],
  ['[abc]  [a-z0-9-]', 'one character from the set'],
  ['[^"\\n]', 'one character not in the set (here: not a quote, not a line break)'],
  ['*  +  ?', 'what comes before, 0 or more · 1 or more · 0 or 1 times'],
  ['{3}  {2,4}  {8,}', 'exactly 3 · 2 to 4 · 8 or more times'],
  ['*?  +?', 'as few times as possible'],
  ['a|b', 'a or b'],
  ['\\b', 'word boundary: \\bweb01\\b does not match inside myweb01x (ASCII letters only)'],
  ['^  $', 'start and end of the value; use them in Keep'],
  ['( )', 'group: only this part is replaced'],
  ['(?: )', 'group for | or repeats that does not change what is replaced'],
]

// Sample lines of one rule, redacted by that rule alone after every edit, so you see at once what a change does.
function Samples({ rule, setName, onChange }: { rule: Rule; setName: string; onChange: (samples?: string[]) => void }) {
  const text = rule.samples?.join('\n') ?? ''
  const [checked, setChecked] = useState<{ input: string; result: PreviewResult } | null>(null)
  useEffect(() => {
    if (!text.trim()) {
      setChecked(null)
      return
    }
    const controller = new AbortController()
    const timer = setTimeout(() => {
      api
        .preview(text, [{ name: setName, enabled: true, rules: [{ ...rule, enabled: true }] }], [], controller.signal)
        .then((result) => setChecked({ input: text, result }))
        .catch(() => {
          // an invalid rule: the error is shown above the tabs
          if (!controller.signal.aborted) setChecked(null)
        })
    }, 250)
    return () => {
      clearTimeout(timer)
      controller.abort()
    }
  }, [rule, setName])

  const parts: Part[] = []
  if (checked) {
    // shortcut: a match across lines (private keys) shifts the lines after it, so "not redacted" can be off there
    const inputs = checked.input.split('\n')
    checked.result.redacted.split('\n').forEach((line, i) => {
      if (i > 0) parts.push({ text: '\n' })
      parts.push(...highlight(line, checked.result.map))
      if (line.trim() && line === inputs[i]) parts.push({ text: '  not redacted', wrap: (t, key) => <span key={key} className="hint">{t}</span> })
    })
  }
  return (
    <>
      <NumberedTextarea
        className="mono"
        rows={1}
        value={text}
        onChange={(e) => onChange(e.target.value ? e.target.value.split('\n') : undefined)}
        placeholder="Sample lines to try the rule on"
        spellCheck={false}
        aria-label="Sample lines"
      />
      {checked && (
        <div className={`output samples mono ${checked.input === text ? '' : 'stale'}`} aria-label="Sample lines redacted by this rule" aria-live="polite">
          <NumberedText parts={parts} />
        </div>
      )}
    </>
  )
}

const newRule = (): Rule => ({ name: 'New rule', enabled: true, type: 'words', patterns: [], placeholder: '[NAME_#]' })

type Props = {
  draft: Ruleset[]
  saved: Ruleset[]
  setDraft: (sets: Ruleset[]) => void
  defaults: Ruleset[]
  hits: Map<string, number>
  error: string
  busy: boolean
  onReset: () => void
}

export default function RulesEditor({ draft, saved, setDraft, defaults, hits, error, busy, onReset }: Props) {
  const fileInput = useRef<HTMLInputElement>(null)
  const importDialog = useRef<HTMLDialogElement>(null)
  const [picked, setPicked] = useState(0)
  const [incoming, setIncoming] = useState<Ruleset[]>([])
  const [chosen, setChosen] = useState<boolean[]>([])
  const [importError, setImportError] = useState('')
  const missingPresets = defaults.filter((preset) => !draft.some((set) => set.name === preset.name))
  // the ruleset shown below the tabs; stays in range when rulesets are deleted or replaced
  const current = Math.min(picked, draft.length - 1)
  const updateAvailable = (set: Ruleset) => defaults.some((p) => p.name === set.name && p.preset !== set.preset)

  // Reads a rules file on the server (it checks every rule) and lets the user pick what to take from it.
  async function readFile(file: File) {
    try {
      const sets = await api.parse(await file.text())
      setIncoming(sets)
      setChosen(sets.map(() => true))
      setImportError('')
      importDialog.current?.showModal()
    } catch (e) {
      setImportError(errorText(e))
    }
  }

  function finishImport(replaceAll: boolean) {
    const taken = incoming.filter((_, k) => chosen[k])
    const merged = mergeRulesets(draft, taken, replaceAll)
    setDraft(merged)
    setPicked(merged.findIndex((set) => set.name === taken[0].name)) // show what arrived
    importDialog.current?.close()
  }

  return (
    <main className="rules-page">
      <details className="help">
        <summary>How rules work</summary>
        <ul>
          <li>Edits apply to the preview at once. <b>Save</b> stores them for the CLI.</li>
          <li><b>words</b>: literal text, one per line, case-insensitive, whole words only.</li>
          <li><b>regex</b>: one regular expression per line, see the cheat sheet below. With a group <code>( )</code> only the group is replaced: <code>password=(\S+)</code> → <code>password=[SECRET_1]</code>.</li>
          <li><b>Placeholder</b>: <code>#</code> becomes a number per distinct value; repeats keep their number. Without <code>#</code> every value gets the same text.</li>
          <li><b>Number each part</b> (placeholders with dots): each part is numbered by its own value, so <code>acme.sk</code> → <code>[DOMAIN_1].[TLD_1]</code> and <code>acme.com</code> → <code>[DOMAIN_1].[TLD_2]</code>.</li>
          <li><b>Keep</b>: matched values that also match this regex stay as they are.</li>
          <li><b>Sample lines</b> (under the patterns): lines to try the rule on. Each is shown redacted by that rule alone as you edit; <i>not redacted</i> marks a line it left as it was. They are saved with the rule and do not count as an edit of a preset.</li>
          <li>When matches overlap, the longest wins.</li>
          <li>Unedited presets update automatically after an upgrade. Edited ones show <i>Update available</i>.</li>
        </ul>
      </details>
      <details className="help">
        <summary>Regex cheat sheet</summary>
        <table>
          <tbody>
            {cheatSheet.map(([syntax, meaning]) => (
              <tr key={syntax}>
                <td><code>{syntax}</code></td>
                <td>{meaning}</td>
              </tr>
            ))}
          </tbody>
        </table>
        <ul>
          <li>Each line in <i>Patterns</i> is its own regex (Go RE2 syntax).</li>
          <li>Only the first group <code>( )</code> that matched is replaced: <code>(?i)\bcustomer_id=(\w+)</code> → <code>customer_id=[CUSTOMER_1]</code>. Without a group the whole match is replaced.</li>
          <li><b>Keep</b> is tested against the replaced part only. Anchor it, <code>^(?:test|demo)$</code>, or <code>test</code> also keeps <code>latest</code>.</li>
          <li>Not supported: lookarounds <code>(?=…)</code> <code>(?!…)</code> <code>(?&lt;=…)</code> and backreferences <code>\1</code>.</li>
          <li>In an exported YAML file every regex is in single quotes; a quote inside is written <code>''</code>.</li>
        </ul>
      </details>

      <div className="toolbar">
        <button
          onClick={() => {
            setDraft([...draft, { name: `Ruleset ${draft.length + 1}`, enabled: true, rules: [newRule()] }])
            setPicked(draft.length)
          }}
        >
          New ruleset
        </button>
        {missingPresets.length > 0 && (
          <select
            value=""
            onChange={(e) => {
              setDraft([...draft, ...missingPresets.filter((p) => p.name === e.target.value)])
              setPicked(draft.length)
            }}
            title="Add back a deleted factory preset."
          >
            <option value="">Add preset…</option>
            {missingPresets.map((p) => <option key={p.name}>{p.name}</option>)}
          </select>
        )}
        <span className="spacer" />
        <a className="button" href={exportUrl} download="memento-rulesets.yml" title="Download the saved rules as YAML. Unsaved changes are not included.">
          Export
        </a>
        <button onClick={() => fileInput.current?.click()} disabled={busy} title="Pick rulesets from a YAML or JSON file to add, or to replace all with. Not saved until you click Save.">
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
            if (file) await readFile(file)
          }}
        />
        <button className="danger" onClick={onReset} disabled={busy} title="Replace all rules, including your own, with the factory presets.">
          Reset all
        </button>
      </div>

      {importError && <p className="error" role="alert">Import: {importError}</p>}
      {error && <p className="error" role="alert">{error}</p>}

      <nav className="tabs" aria-label="Rulesets">
        {draft.map((set, i) => (
          <button key={i} className={set.enabled ? '' : 'off'} aria-current={i === current ? 'true' : undefined} onClick={() => setPicked(i)} title={set.description}>
            {set.name || 'Unnamed'}
            <Count n={hits.get(hitKey(set.name))} />
            {updateAvailable(set) && <span className="note" title="Update available"> •</span>}
          </button>
        ))}
      </nav>

      {draft.map((set, i) => {
        // only the ruleset picked in the tabs is shown
        if (i !== current) return null
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
              {updateAvailable(set) && (
                <span className="note" title="The factory version changed. Restore preset to get it; it replaces your edits in this ruleset.">
                  Update available
                </span>
              )}
              {saved.some((s) => s.name === set.name) && (
                <a className="button" href={`${exportUrl}&name=${encodeURIComponent(set.name)}`} download={`${set.name}.yml`} title="Download this ruleset as saved, as YAML. Unsaved changes are not included.">
                  Export
                </a>
              )}
              {preset && (
                <button onClick={() => setDraft(updateSet(draft, i, preset))} title="Replace this ruleset with its factory version.">
                  Restore preset
                </button>
              )}
              <button
                className="danger"
                onClick={() => {
                  setDraft(draft.filter((_, j) => j !== i))
                  setPicked(Math.max(0, i - 1))
                }}
              >
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
              <span title="# becomes a number per distinct value. With dots, like [DOMAIN].[TLD], each part can be numbered on its own.">Placeholder</span>
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
                  <div className="stack">
                    <NumberedTextarea
                      className="mono"
                      rows={1}
                      value={rule.patterns.join('\n')}
                      onChange={(e) => change({ patterns: e.target.value.split('\n') })}
                      placeholder={rule.type === 'words' ? 'web01\nProject Phoenix' : '\\bSRV-\\d{3}\\b'}
                      spellCheck={false}
                      aria-label="Patterns"
                    />
                    <Samples rule={rule} setName={set.name} onChange={(samples) => change({ samples })} />
                  </div>
                  <div className="stack">
                    <input className="mono" value={rule.placeholder} onChange={(e) => change({ placeholder: e.target.value })} aria-label="Placeholder" />
                    <label
                      className="option"
                      title="Number each dot-separated part by its own value: acme.sk → [DOMAIN_1].[TLD_1], acme.com → [DOMAIN_1].[TLD_2]. Needs a placeholder with dots, like [DOMAIN].[TLD]."
                    >
                      <input type="checkbox" checked={rule.per_part ?? false} disabled={!rule.placeholder.includes('.')} onChange={(e) => change({ per_part: e.target.checked })} />
                      Number each part
                    </label>
                    {!rule.placeholder.includes('.') && (
                      <span className="hint">
                        needs dots, like <code>[DOMAIN].[TLD]</code>
                      </span>
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

      <dialog ref={importDialog} aria-labelledby="import-title">
        <h2 id="import-title">Import rulesets</h2>
        <p className="muted">Pick what to take from the file. Nothing is saved until you click Save.</p>
        {incoming.map((set, k) => (
          <label key={k} className="pick">
            <input type="checkbox" checked={chosen[k] ?? false} onChange={(e) => setChosen(chosen.map((c, j) => (j === k ? e.target.checked : c)))} />
            {set.name}
            <span className="muted">
              {set.rules.length} {set.rules.length === 1 ? 'rule' : 'rules'}
              {draft.some((d) => d.name === set.name) ? ', replaces yours on Add' : ''}
            </span>
          </label>
        ))}
        <div className="toolbar">
          <button className="primary" onClick={() => finishImport(false)} disabled={!chosen.some(Boolean)} title="Add the picked rulesets. One with the same name as yours replaces it.">
            Add
          </button>
          <button className="danger" onClick={() => finishImport(true)} disabled={!chosen.some(Boolean)} title="Your rulesets are replaced by the picked ones.">
            Replace all
          </button>
          <span className="spacer" />
          <button onClick={() => importDialog.current?.close()}>Cancel</button>
        </div>
      </dialog>
    </main>
  )
}
