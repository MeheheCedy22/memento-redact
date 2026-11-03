import { useEffect, useMemo, useState, type ReactNode, type RefObject } from 'react'
import Editor, { DiffEditor, type OnMount } from '@monaco-editor/react'
import { Responsive, useContainerWidth, type LayoutItem, type ResponsiveLayouts } from 'react-grid-layout'
import type { PreviewResult, Ruleset } from './api'
import { hitKey, Hits, updateRule, updateSet } from './RulesEditor'

const panels = {
  original: { title: 'Original (not redacted)', hint: 'Paste or type the text you want to give an LLM. It is only sent to your memento-redact server and never stored.' },
  redacted: { title: 'Redacted (safe to send)', hint: 'Live result of the enabled rules. Hover a highlighted placeholder to see what it replaced, then copy the text into your LLM chat.' },
  rulesets: { title: 'Applied rulesets', hint: 'Switch bundles or single rules on/off. The number shows how many replacements each one made in the current text.' },
  editor: { title: 'Rule editor', hint: 'Add words/phrases and regexes. Changes apply to the preview at once and are stored when you click Save rules.' },
  map: { title: 'Redaction map', hint: 'Every placeholder and the original value behind it. Use it to translate the LLM answer back.' },
  diff: { title: 'Diff', hint: 'Side-by-side diff of the original and redacted text with every change highlighted.' },
} as const
type PanelId = keyof typeof panels
const panelIds = Object.keys(panels) as PanelId[]

const cols = { lg: 12, md: 6, sm: 1 }
const defaultLayout: LayoutItem[] = [
  { i: 'original', x: 0, y: 0, w: 6, h: 12, minW: 2, minH: 4 },
  { i: 'redacted', x: 6, y: 0, w: 6, h: 12, minW: 2, minH: 4 },
  { i: 'rulesets', x: 0, y: 12, w: 3, h: 14, minW: 2, minH: 4 },
  { i: 'editor', x: 3, y: 12, w: 9, h: 14, minW: 3, minH: 4 },
  { i: 'map', x: 0, y: 26, w: 5, h: 10, minW: 2, minH: 3 },
  { i: 'diff', x: 5, y: 26, w: 7, h: 10, minW: 3, minH: 4 },
]
const allVisible = Object.fromEntries(panelIds.map((id) => [id, true])) as Record<PanelId, boolean>

const sample = `PS C:\\> Get-VM | Select Name, PowerState, @{N='IP';E={$_.Guest.IPAddress -join ', '}}

Name                    PowerState IP
----                    ---------- --
web01.prod.domain.tld   PoweredOn  10.20.30.41, fe80::250:56ff:fe9a:1b2c
db01.prod.domain.tld    PoweredOn  10.20.30.42
edge-gw.domain.tld      PoweredOn  192.168.1.1, 203.0.113.17
vpn.domain.tld          PoweredOn  198.51.100.7, 2001:db8:85a3::8a2e:370:7334

PS C:\\> Connect-VIServer vcenter.domain.tld -User admin@domain.tld -Password 'S3cr3t!'
Owner: jane.doe@domain.tld, +420 777 123 456
`

const editorOptions = {
  minimap: { enabled: false },
  wordWrap: 'on',
  scrollBeyondLastLine: false,
  automaticLayout: true,
  fontSize: 13,
} as const

// Layout and panel visibility are per-browser conveniences; storage may be unavailable (private mode).
function stored<T>(key: string, fallback: T): T {
  try {
    const value = localStorage.getItem(key)
    return value ? (JSON.parse(value) as T) : fallback
  } catch {
    return fallback
  }
}

function store(key: string, value: unknown) {
  try {
    localStorage.setItem(key, JSON.stringify(value))
  } catch {
    // ignore: the layout simply is not remembered
  }
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
  editor: ReactNode
}

export default function Workspace({ text, setText, result, upToDate, previewError, draft, setDraft, hits, editor }: Props) {
  const { width, containerRef, mounted } = useContainerWidth()
  const [layouts, setLayouts] = useState<ResponsiveLayouts>(() => stored('memento.layouts', { lg: defaultLayout }))
  const [visible, setVisible] = useState<Record<PanelId, boolean>>(() => ({ ...allVisible, ...stored('memento.panels', {}) }))
  const [redactedEditor, setRedactedEditor] = useState<Parameters<OnMount>[0] | null>(null)
  const [copyNote, setCopyNote] = useState('')

  const shown = panelIds.filter((id) => visible[id])
  // Panels shown again after hiding have no saved position; put them back at their default size.
  const effectiveLayouts = useMemo(() => {
    const out: ResponsiveLayouts = {}
    for (const [bp, layout = []] of Object.entries({ lg: [] as readonly LayoutItem[], ...layouts })) {
      const missing = defaultLayout
        .filter((d) => shown.includes(d.i as PanelId) && !layout.some((item) => item.i === d.i))
        .map((d) => ({ ...d, x: 0, y: 1000, w: Math.min(d.w, cols[bp as keyof typeof cols] ?? d.w) }))
      out[bp] = [...layout, ...missing]
    }
    return out
  }, [layouts, shown.join()])

  useEffect(() => {
    if (!redactedEditor || !result) return
    const model = redactedEditor.getModel()
    if (!model) return
    const decorations = result.map.flatMap((entry) =>
      model.findMatches(entry.token, false, false, true, null, false).map((match) => ({
        range: match.range,
        options: {
          inlineClassName: 'token-highlight',
          hoverMessage: [{ value: `**${entry.token}** - ${entry.ruleset} › ${entry.rule}` }, { value: '```\n' + entry.original + '\n```' }],
        },
      })),
    )
    const collection = redactedEditor.createDecorationsCollection(decorations)
    return () => collection.clear()
  }, [redactedEditor, result])

  function toggle(id: PanelId) {
    const next = { ...visible, [id]: !visible[id] }
    setVisible(next)
    store('memento.panels', next)
  }

  function resetLayout() {
    setLayouts({ lg: defaultLayout })
    setVisible(allVisible)
    store('memento.layouts', { lg: defaultLayout })
    store('memento.panels', allVisible)
  }

  async function copyRedacted() {
    if (!result) return
    if (!navigator.clipboard) {
      setCopyNote('Clipboard needs HTTPS or localhost - click into the text, Ctrl+A, Ctrl+C.')
      return
    }
    await navigator.clipboard.writeText(result.redacted)
    setCopyNote('Copied ✓')
    setTimeout(() => setCopyNote(''), 2000)
  }

  const replacements = result?.map.reduce((sum, entry) => sum + entry.count, 0) ?? 0
  const content: Record<PanelId, { actions?: ReactNode; body: ReactNode }> = {
    original: {
      actions: (
        <>
          <button className="secondary small" onClick={() => setText(sample)} title="Replace the text with a PowerCLI example that shows what the presets do.">Load example</button>
          <button className="secondary small" onClick={() => setText('')} title="Empty the original text.">Clear</button>
        </>
      ),
      body: <Editor theme="vs-dark" language="plaintext" value={text} onChange={(value) => setText(value ?? '')} options={editorOptions} />,
    },
    redacted: {
      actions: (
        <>
          <span className={`chip ${previewError ? 'error' : upToDate ? 'ok' : 'busy'}`} title="Whether the redacted text matches the current original text and rules.">
            {previewError ? 'rule error' : upToDate ? `${replacements} replacements` : 'updating…'}
          </span>
          {copyNote && <span className="note">{copyNote}</span>}
          <button
            onClick={copyRedacted}
            disabled={!upToDate || !result?.redacted}
            title="Copy the redacted text to the clipboard. Disabled while the result is out of date or a rule has an error."
          >
            Copy redacted
          </button>
        </>
      ),
      body: (
        <div className={`stack ${upToDate ? '' : 'stale'}`}>
          {previewError && <p className="error-banner" role="alert">Not redacted - {previewError}</p>}
          <Editor theme="vs-dark" language="plaintext" value={result?.redacted ?? ''} options={{ ...editorOptions, readOnly: true }} onMount={(ed) => setRedactedEditor(ed)} />
        </div>
      ),
    },
    rulesets: {
      body: (
        <ul className="toggles">
          {draft.map((set, i) => (
            <li key={i} className={set.enabled ? '' : 'off'}>
              <label title={set.description || 'Switch the whole ruleset on or off.'}>
                <input type="checkbox" className="switch" checked={set.enabled} onChange={(e) => setDraft(updateSet(draft, i, { enabled: e.target.checked }))} />
                <strong>{set.name}</strong>
                <Hits n={hits.get(hitKey(set.name))} />
              </label>
              <ul>
                {set.rules.map((rule, k) => (
                  <li key={k}>
                    <label title={rule.description || `${rule.type}: ${rule.patterns.length} pattern(s), placeholder ${rule.placeholder}`}>
                      <input type="checkbox" className="switch small" checked={rule.enabled} onChange={(e) => setDraft(updateRule(draft, i, k, { enabled: e.target.checked }))} />
                      {rule.name}
                      <Hits n={hits.get(hitKey(set.name, rule.name))} />
                    </label>
                  </li>
                ))}
              </ul>
            </li>
          ))}
        </ul>
      ),
    },
    editor: { body: <div className="scroll">{editor}</div> },
    map: {
      body: (
        <div className="scroll">
          {result?.map.length ? (
            <table className="map">
              <thead>
                <tr>
                  <th title="Text that replaced the value in the redacted version.">Placeholder</th>
                  <th title="The value that was removed.">Original</th>
                  <th title="Ruleset › rule that matched.">Rule</th>
                  <th title="How many times the value appeared.">×</th>
                </tr>
              </thead>
              <tbody>
                {result.map.map((entry) => (
                  <tr key={entry.token + '\u0000' + entry.original}>
                    <td><code>{entry.token}</code></td>
                    <td><code>{entry.original}</code></td>
                    <td>{entry.ruleset} › {entry.rule}</td>
                    <td>{entry.count}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <p className="subtle">Nothing redacted yet.</p>
          )}
        </div>
      ),
    },
    diff: {
      body: (
        // The wrapper disposes models before the diff widget on unmount (hiding the panel), which throws;
        // keep them under fixed URIs so a remount reuses them, and refresh their text on mount.
        <DiffEditor
          theme="vs-dark"
          original={text}
          modified={result?.redacted ?? ''}
          originalModelPath="inmemory://memento/diff-original"
          modifiedModelPath="inmemory://memento/diff-redacted"
          keepCurrentOriginalModel
          keepCurrentModifiedModel
          onMount={(diff) => {
            diff.getModel()?.original.setValue(text)
            diff.getModel()?.modified.setValue(result?.redacted ?? '')
          }}
          options={{ ...editorOptions, readOnly: true, originalEditable: false, renderSideBySide: true }}
        />
      ),
    },
  }

  return (
    <div className="workspace">
      <div className="toolbar">
        <span className="subtle" title="Drag a panel by its title bar; resize it from its edges or bottom-right corner. The layout is remembered in this browser.">
          Panels:
        </span>
        {panelIds.map((id) => (
          <label key={id} className="panel-toggle" title={`Show or hide the ${panels[id].title} panel.`}>
            <input type="checkbox" checked={visible[id]} onChange={() => toggle(id)} />
            {panels[id].title.split(' (')[0]}
          </label>
        ))}
        <span className="spacer" />
        <button className="secondary small" onClick={resetLayout} title="Put all panels back to their default position and size, and show hidden ones.">
          Reset layout
        </button>
      </div>
      <div ref={containerRef as RefObject<HTMLDivElement>}>
        {mounted && (
          <Responsive
            width={width}
            layouts={effectiveLayouts}
            breakpoints={{ lg: 1100, md: 700, sm: 0 }}
            cols={cols}
            rowHeight={30}
            dragConfig={{ handle: '.panel-header', cancel: 'button, input, select, label, a, .chip, .note' }}
            resizeConfig={{ handles: ['s', 'e', 'se'] }}
            onLayoutChange={(_, all) => {
              setLayouts(all)
              store('memento.layouts', all)
            }}
          >
            {shown.map((id) => (
              <section key={id} className="panel">
                <header className="panel-header" title={`${panels[id].hint}\n\nDrag this bar to move the panel.`}>
                  <h2>{panels[id].title}</h2>
                  <div className="actions">{content[id].actions}</div>
                </header>
                <div className="panel-body">{content[id].body}</div>
              </section>
            ))}
          </Responsive>
        )}
      </div>
    </div>
  )
}
