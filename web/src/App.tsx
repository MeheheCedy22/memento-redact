import { useEffect, useMemo, useState } from 'react'
import { api, errorText, type PreviewResult, type Ruleset } from './api'
import RulesEditor, { hitKey } from './RulesEditor'
import Workspace from './Workspace'

type Page = 'workspace' | 'rules'
const pageFromHash = (): Page => (location.hash === '#/rules' ? 'rules' : 'workspace')

function App() {
  const [page, setPage] = useState(pageFromHash)
  const [saved, setSaved] = useState<Ruleset[] | null>(null)
  const [draft, setDraft] = useState<Ruleset[]>([])
  const [defaults, setDefaults] = useState<Ruleset[]>([])
  // Input text lives only in memory: it is exactly the data this tool must not leave lying around.
  const [text, setText] = useState('')
  const [result, setResult] = useState<(PreviewResult & { input: string; rules: Ruleset[] }) | null>(null)
  const [previewError, setPreviewError] = useState('')
  const [status, setStatus] = useState<{ kind: 'ok' | 'error'; text: string } | null>(null)
  const [busy, setBusy] = useState(false)

  const dirty = saved !== null && JSON.stringify(saved) !== JSON.stringify(draft)
  const upToDate = !previewError && result !== null && result.input === text && result.rules === draft

  useEffect(() => {
    const onHash = () => setPage(pageFromHash())
    window.addEventListener('hashchange', onHash)
    return () => window.removeEventListener('hashchange', onHash)
  }, [])

  useEffect(() => {
    void run(async () => {
      const [sets, presets] = await Promise.all([api.rulesets(), api.defaults()])
      setDefaults(presets)
      adopt(sets, '')
    })
  }, [])

  useEffect(() => {
    if (!dirty) return
    const warn = (event: BeforeUnloadEvent) => event.preventDefault()
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [dirty])

  // Live preview with the draft rules, so edits are visible before they are saved.
  useEffect(() => {
    if (saved === null) return
    const controller = new AbortController()
    const timer = setTimeout(() => {
      api
        .preview(text, draft, controller.signal)
        .then((res) => {
          setResult({ ...res, input: text, rules: draft })
          setPreviewError('')
        })
        .catch((error) => {
          if (!controller.signal.aborted) setPreviewError(errorText(error))
        })
    }, 250)
    return () => {
      clearTimeout(timer)
      controller.abort()
    }
  }, [text, draft, saved])

  const hits = useMemo(() => {
    const counts = new Map<string, number>()
    for (const entry of result?.map ?? []) {
      for (const key of [hitKey(entry.ruleset), hitKey(entry.ruleset, entry.rule)]) {
        counts.set(key, (counts.get(key) ?? 0) + entry.count)
      }
    }
    return counts
  }, [result])

  function adopt(sets: Ruleset[], message: string) {
    setSaved(sets)
    setDraft(sets)
    setStatus(message ? { kind: 'ok', text: message } : null)
  }

  async function run(action: () => Promise<void>) {
    setBusy(true)
    try {
      await action()
    } catch (error) {
      setStatus({ kind: 'error', text: errorText(error) })
    } finally {
      setBusy(false)
    }
  }

  const save = () => run(async () => adopt(await api.save(draft), 'Rules saved on the server. The CLI and proxy use them now.'))
  const discard = () => saved && adopt(saved, 'Unsaved rule changes discarded.')
  const reset = () => {
    if (confirm('Replace ALL rulesets with the factory defaults?\n\nYour own words and rules will be lost. Use "Export saved (YAML)" first to keep a copy.')) {
      void run(async () => adopt(await api.reset(), 'All rulesets reset to factory defaults.'))
    }
  }
  const importYaml = (yaml: string) => {
    if (confirm('Replace ALL rulesets with the imported file?\n\nIt is saved immediately; unsaved changes are lost.')) {
      void run(async () => adopt(await api.importYaml(yaml), 'Rulesets imported and saved.'))
    }
  }

  const editor = (
    <RulesEditor
      draft={draft}
      setDraft={setDraft}
      defaults={defaults}
      hits={hits}
      error={previewError}
      busy={busy}
      onReset={reset}
      onImport={importYaml}
    />
  )

  return (
    <div className="app">
      <header className="topbar">
        <h1 title="Self-hosted redaction proxy: remove PII and internal names before text goes to an LLM.">memento-redact</h1>
        <nav>
          <a href="#/" className={page === 'workspace' ? 'active' : ''} title="Redact text: original, redacted result, rulesets and map side by side.">Workspace</a>
          <a href="#/rules" className={page === 'rules' ? 'active' : ''} title="Manage rulesets, presets, words and regexes on a full page.">Rules</a>
        </nav>
        <span className="spacer" />
        {dirty ? (
          <>
            <span className="chip busy" title="Rule edits are already used in the preview, but not stored on the server yet.">● Unsaved rule changes</span>
            <button onClick={save} disabled={busy} title="Store the current rules on the server (persistent volume). The CLI and proxy use them from then on.">Save rules</button>
            <button className="secondary" onClick={discard} disabled={busy} title="Throw away rule edits since the last save and go back to the saved rules.">Discard changes</button>
          </>
        ) : (
          <span className="chip ok" title="The rules in use match the rules stored on the server.">{saved ? 'Rules saved' : 'Loading…'}</span>
        )}
      </header>

      {status && (
        <div className={`status ${status.kind}`} role={status.kind === 'error' ? 'alert' : 'status'}>
          {status.text}
          <button className="secondary small" onClick={() => setStatus(null)} title="Dismiss this message." aria-label="Dismiss">✕</button>
        </div>
      )}

      {page === 'workspace' ? (
        <Workspace
          text={text}
          setText={setText}
          result={result}
          upToDate={upToDate}
          previewError={previewError}
          draft={draft}
          setDraft={setDraft}
          hits={hits}
          editor={editor}
        />
      ) : (
        <main className="rules-page">{editor}</main>
      )}
    </div>
  )
}

export default App
