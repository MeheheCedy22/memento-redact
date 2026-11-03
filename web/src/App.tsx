import { useEffect, useMemo, useState } from 'react'
import { api, errorText, type PreviewResult, type Ruleset } from './api'
import RulesEditor, { hitKey } from './RulesEditor'
import { currentTheme, setTheme, type Theme } from './theme'
import Workspace from './Workspace'

type Page = 'redact' | 'rules'
const pageFromHash = (): Page => (location.hash === '#/rules' ? 'rules' : 'redact')

function App() {
  const [page, setPage] = useState(pageFromHash)
  const [theme, setThemeState] = useState<Theme>(currentTheme)
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

  const save = () => run(async () => adopt(await api.save(draft), 'Saved.'))
  const discard = () => saved && adopt(saved, 'Changes discarded.')
  const reset = () => {
    if (confirm('Replace all rules, including your own, with the factory presets?\n\nExport first if you want a copy.')) {
      void run(async () => adopt(await api.reset(), 'Factory presets restored.'))
    }
  }
  const importYaml = (yaml: string) => {
    if (confirm('Replace all rules with this file? It is saved immediately.')) {
      void run(async () => adopt(await api.importYaml(yaml), 'Imported.'))
    }
  }
  const toggleTheme = () => {
    const next = theme === 'dark' ? 'light' : 'dark'
    setTheme(next)
    setThemeState(next)
  }

  return (
    <>
      <header className="topbar">
        <strong className="brand">memento-redact</strong>
        <nav>
          <a href="#/" aria-current={page === 'redact' ? 'page' : undefined}>Redact</a>
          <a href="#/rules" aria-current={page === 'rules' ? 'page' : undefined}>Rules</a>
        </nav>
        <span className="spacer" />
        {status && (
          <span className={status.kind === 'error' ? 'error-text' : 'note'} role={status.kind === 'error' ? 'alert' : 'status'}>
            {status.text}
          </span>
        )}
        {dirty && (
          <>
            <span className="muted" title="Edits are used in the preview but not stored yet.">Unsaved changes</span>
            <button className="primary" onClick={save} disabled={busy} title="Store the rules on the server. The CLI uses saved rules.">Save</button>
            <button onClick={discard} disabled={busy} title="Go back to the last saved rules.">Discard</button>
          </>
        )}
        <button onClick={toggleTheme} title="Switch between light and dark mode.">{theme === 'dark' ? 'Light' : 'Dark'}</button>
      </header>

      {page === 'redact' ? (
        <Workspace
          text={text}
          setText={setText}
          result={result}
          upToDate={upToDate}
          previewError={previewError}
          draft={draft}
          setDraft={setDraft}
          hits={hits}
        />
      ) : (
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
      )}
    </>
  )
}

export default App
