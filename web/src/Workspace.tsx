import { useEffect, useRef, useState } from 'react'
import AutoTextarea from './AutoTextarea'
import type { MapEntry, PreviewResult, Ruleset } from './api'
import { copyText } from './clipboard'
import { highlight, NumberedText, NumberedTextarea } from './Numbered'
import Restore from './Restore'
import { remember } from './restore'
import { debugReport, reasons, type Mark } from './debug'
import { activeProfile, applySwitches, switchesOf } from './rulesets'
import { saveProfiles, saveReasons, savedProfiles, savedReasons } from './storage'
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

const entryKey = (entry: MapEntry) => entry.token + '\u0000' + entry.original
// select values of "New reason…" and "Save switches as…"; a name cannot contain them
const newReason = '\u0000new'
const newProfile = '\u0000save'

type Props = {
  // the Redact page, or the Restore page (same layout, the text pair shows an LLM answer instead)
  mode: 'redact' | 'restore'
  // the map of the texts copied so far, and how to add the current one or start over
  conversation: MapEntry[]
  onCopied: () => void
  onNewConversation: () => void
  answer: string
  setAnswer: (answer: string) => void
  text: string
  setText: (text: string) => void
  result: PreviewResult | null
  upToDate: boolean
  previewError: string
  draft: Ruleset[]
  setDraft: (sets: Ruleset[]) => void
  hits: Map<string, number>
}

export default function Workspace({ mode, conversation, onCopied, onNewConversation, answer, setAnswer, text, setText, result, upToDate, previewError, draft, setDraft, hits }: Props) {
  const [copyNote, setCopyNote] = useState('')
  const [debug, setDebug] = useState(false)
  const [reported, setReported] = useState(new Map<string, Omit<Mark, 'entry'>>())
  const [missed, setMissed] = useState<string[]>([])
  const [reportNote, setReportNote] = useState('')
  const [ownReasons, setOwnReasons] = useState(savedReasons)
  const [profiles, setProfiles] = useState(savedProfiles)
  const active = activeProfile(draft, profiles)
  const outputRef = useRef<HTMLDivElement>(null)
  const jumps = useRef(new Map<string, number>())

  // Selecting redacted text and pressing Ctrl+C sends it out just like Copy. The browser sends the copy event
  // to the focused element or the body, not to the box with the selection, so the document listens.
  useEffect(() => {
    const copied = () => {
      const at = getSelection()?.anchorNode
      if (mode === 'redact' && upToDate && at && outputRef.current?.contains(at)) onCopied()
    }
    document.addEventListener('copy', copied)
    return () => document.removeEventListener('copy', copied)
  })

  // Scrolls the text pair to the next occurrence of token in the redacted text and outlines it.
  function jumpTo(token: string) {
    const marks = [...(outputRef.current?.querySelectorAll('mark') ?? [])].filter((m) => m.dataset.token === token)
    if (marks.length === 0) return
    const i = ((jumps.current.get(token) ?? -1) + 1) % marks.length
    jumps.current.set(token, i)
    outputRef.current?.querySelector('mark.current')?.classList.remove('current')
    marks[i].classList.add('current')
    marks[i].scrollIntoView({ block: 'center', behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth' })
  }
  const replacements = result?.map.reduce((sum, entry) => sum + entry.count, 0) ?? 0
  // values redacted elsewhere but still in the text, as one line: "hunter2", "acme" 2×
  const leaks = result?.leaks?.map((l) => `"${l.value}"${l.count > 1 ? ` ${l.count}×` : ''}`).join(', ')
  // Restore and its map use every value of the conversation, plus the current text's
  const known = remember(conversation, result?.map ?? [])
  const shown = mode === 'restore' ? known : (result?.map ?? [])

  const marks: Mark[] = (result?.map ?? []).flatMap((entry) => {
    const marked = reported.get(entryKey(entry))
    return marked ? [{ entry, ...marked }] : []
  })
  const report = marks.length || missed.length ? debugReport(result?.redacted ?? '', result?.map ?? [], marks, missed, draft) : ''

  function mark(entry: MapEntry, reason: string, note = reported.get(entryKey(entry))?.note) {
    const next = new Map(reported)
    if (reason) next.set(entryKey(entry), { reason, note })
    else next.delete(entryKey(entry))
    setReported(next)
  }

  // Asks for a name, saves it as the user's own reason and marks entry with it.
  function addReason(entry: MapEntry) {
    const name = prompt('Name of the new reason')?.trim()
    if (!name) return
    if (!reasons.has(name) && !ownReasons.includes(name)) {
      setOwnReasons([...ownReasons, name])
      saveReasons([...ownReasons, name])
    }
    mark(entry, name)
  }

  function forgetReason(name: string) {
    const next = ownReasons.filter((r) => r !== name)
    setOwnReasons(next)
    saveReasons(next)
  }

  // Applies a saved profile to the switches, or saves the current switches under a name.
  function pickProfile(value: string) {
    if (value !== newProfile) {
      const profile = profiles.find((p) => p.name === value)
      if (profile) setDraft(applySwitches(draft, profile.switches))
      return
    }
    const name = prompt('Name of the profile', active)?.trim()
    if (!name) return
    const next = [...profiles.filter((p) => p.name !== name), { name, switches: switchesOf(draft) }]
    setProfiles(next)
    saveProfiles(next)
  }

  function deleteProfile() {
    const next = profiles.filter((p) => p.name !== active)
    setProfiles(next)
    saveProfiles(next)
  }

  // Adds the text selected in the redacted output as a false negative.
  function addSelection() {
    const selection = getSelection()
    const picked = selection?.toString().trim() ?? ''
    if (!picked || !selection?.anchorNode || !outputRef.current?.contains(selection.anchorNode)) {
      setReportNote('Select text in Redacted first.')
      return
    }
    setMissed([...missed, picked])
    setReportNote('')
  }

  return (
    <main className="workspace">
      {/* one scroll area for both columns, so long texts stay side by side; its height changes only by dragging the corner */}
      <div className="pair">
        {mode === 'restore' ? (
          <Restore answer={answer} setAnswer={setAnswer} map={known} outputRef={outputRef} />
        ) : (
          <>
            <section>
              <div className="section-head">
                <h2>Original</h2>
                <button onClick={() => setText(example)} title="Replace the text with an example.">Example</button>
                <button onClick={() => setText('')} title="Clear the text.">Clear</button>
              </div>
              <NumberedTextarea
                className="mono"
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
                  onClick={async () => {
                    if (!result) return
                    if (leaks && !confirm(`Still in the redacted text: ${leaks}.\n\nCopy anyway?`)) return
                    if (await copyText(result.redacted, setCopyNote)) onCopied()
                  }}
                  disabled={!upToDate || !result?.redacted}
                  title="Copy the redacted text. Disabled while it is out of date or a rule has an error. Asks first if a redacted value is still in the text."
                >
                  Copy
                </button>
              </div>
              {previewError && <p className="error" role="alert">Not redacted: {previewError}</p>}
              {!previewError && leaks && <p className="error" role="alert">Not redacted everywhere: {leaks}. Outlined below.</p>}
              <div ref={outputRef} className={`output mono ${upToDate ? '' : 'stale'}`} aria-label="Redacted text" aria-live="polite">
                {result?.redacted ? <NumberedText parts={highlight(result.redacted, result.map, result.leaks)} /> : <span className="muted">The redacted text appears here.</span>}
              </div>
            </section>
          </>
        )}
      </div>

      <div className="below">
        <section>
          <div className="section-head">
            <h2>Rulesets</h2>
            <select value={active} onChange={(e) => pickProfile(e.target.value)} aria-label="Profile" title="A profile is a saved set of on/off switches. Pick one to apply it; Save keeps it for the CLI too.">
              <option value="">{profiles.length ? 'Profile…' : 'No profiles'}</option>
              {profiles.map((p) => (
                <option key={p.name} value={p.name}>
                  {p.name}
                </option>
              ))}
              <option value={newProfile}>Save switches as…</option>
            </select>
            {active && (
              <button className="danger" onClick={deleteProfile} aria-label={`Delete profile ${active}`} title="Delete this profile. The switches stay as they are.">
                ✕
              </button>
            )}
          </div>
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
          <div className="section-head">
            <h2>Map</h2>
            {conversation.length > 0 && (
              <span className="muted" title="Values of the texts you copied keep their placeholders in the next text, and Restore knows them.">
                {conversation.length} in conversation
              </span>
            )}
            <button onClick={onNewConversation} disabled={!conversation.length} title="Forget the copied texts' values: numbering starts from 1 again.">
              New conversation
            </button>
            {mode === 'redact' && (
              <button onClick={() => setDebug(!debug)} aria-pressed={debug} title="Mark wrong redactions and copy a report about them.">
                Debug
              </button>
            )}
          </div>
          <details className="help">
            <summary>What is a conversation?</summary>
            <ul>
              <li>Copying the redacted text (<b>Copy</b> or Ctrl+C) adds its values to the conversation.</li>
              <li>In the next text you paste, the same values get the same placeholders, and new ones continue the numbering: after <code>[HOST_5]</code> comes <code>[HOST_6]</code>.</li>
              <li><b>Restore</b> knows every value of the conversation, so the LLM's answer can mention any text you copied.</li>
              <li><b>New conversation</b> starts from 1 again. The conversation is kept only in this tab; reloading ends it.</li>
            </ul>
          </details>
          {shown.length ? (
            <div className="scroll">
              <table className="map">
                <thead>
                  <tr>
                    <th>Placeholder</th>
                    <th>Original</th>
                    <th>Rule</th>
                    <th title="Occurrences">#</th>
                    {debug && mode === 'redact' && <th>Report</th>}
                  </tr>
                </thead>
                <tbody>
                  {shown.map((entry) => (
                    <tr key={entryKey(entry)}>
                      <td>
                        <button className="link mono" onClick={() => jumpTo(entry.token)} title="Show in the text. Click again for the next occurrence.">
                          {entry.token}
                        </button>
                      </td>
                      <td className="mono">{entry.original}</td>
                      <td>{entry.ruleset} › {entry.rule}</td>
                      <td>{entry.count}</td>
                      {debug && mode === 'redact' && (
                        <td>
                          <select
                            value={reported.get(entryKey(entry))?.reason ?? ''}
                            onChange={(e) => (e.target.value === newReason ? addReason(entry) : mark(entry, e.target.value))}
                            aria-label={`Report ${entry.token}`}
                          >
                            <option value="">—</option>
                            {[...reasons].map(([reason, meaning]) => (
                              <option key={reason} value={reason} title={meaning}>
                                {reason}
                              </option>
                            ))}
                            {ownReasons.length > 0 && (
                              <optgroup label="Your reasons">
                                {ownReasons.map((reason) => (
                                  <option key={reason} value={reason}>
                                    {reason}
                                  </option>
                                ))}
                              </optgroup>
                            )}
                            <option value={newReason}>New reason…</option>
                          </select>
                          {reported.has(entryKey(entry)) && (
                            <AutoTextarea
                              rows={1}
                              value={reported.get(entryKey(entry))?.note ?? ''}
                              onChange={(e) => mark(entry, reported.get(entryKey(entry))?.reason ?? '', e.target.value.replace(/\r?\n/g, ' '))}
                              placeholder="Description (optional)"
                              aria-label={`Description for ${entry.token}`}
                            />
                          )}
                        </td>
                      )}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <p className="muted">Each placeholder and the value it replaced.</p>
          )}
          {debug && mode === 'redact' && (
            <div className="debug">
              <div className="section-head">
                <span className="muted spacer">Pick a reason next to a wrong placeholder, or select missed text in Redacted.</span>
                {reportNote && <span className="note">{reportNote}</span>}
                {/* mousedown would clear the selection before the click reads it */}
                <button onMouseDown={(e) => e.preventDefault()} onClick={addSelection} title="Report the text selected in Redacted as not redacted.">
                  Add selection
                </button>
                <button
                  onClick={() => {
                    setReported(new Map())
                    setMissed([])
                    setReportNote('')
                  }}
                  disabled={!report}
                >
                  Clear report
                </button>
                <button className="primary" onClick={() => copyText(report, setReportNote)} disabled={!report}>
                  Copy report
                </button>
              </div>
              {ownReasons.length > 0 && (
                <p className="muted">
                  Your reasons:{' '}
                  {ownReasons.map((reason) => (
                    <button key={reason} className="chip" onClick={() => forgetReason(reason)} aria-label={`Remove reason ${reason}`} title="Remove this reason">
                      {reason} ✕
                    </button>
                  ))}
                </p>
              )}
              {report && (
                <>
                  <AutoTextarea className="mono" rows={1} value={report} readOnly aria-label="Debug report" />
                  <p className="muted">It contains the values you marked and the rules' regexes. Read it before you send it.</p>
                </>
              )}
            </div>
          )}
        </section>
      </div>
    </main>
  )
}
