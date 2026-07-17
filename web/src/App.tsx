import { useEffect, useMemo, useState } from 'react'

type RedactionDiff = {
  original: string
  replacement: string
}

type PreviewResult = {
  original: string
  redacted: string
  diffs: RedactionDiff[]
}

type Rule = {
  category?: string
  name: string
  description?: string
  pattern: string
  replacement: string
  enabled: boolean
}

const defaultPrompt = 'Hello Alice, please email alice@example.com or call +1 (555) 123-4567.'

const defaultRules = JSON.stringify(
  [
    { name: 'email', pattern: '([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,})', replacement: '[REDACTED_EMAIL]', enabled: true },
    { name: 'phone', pattern: '\\b(\\+?\\d[\\d\\-(). ]{6,}\\d)\\b', replacement: '[REDACTED_PHONE]', enabled: true },
  ],
  null,
  2,
)

function App() {
  const apiBaseUrl = useMemo(
    () => import.meta.env.VITE_API_BASE_URL?.replace(/\/$/, '') ?? '/api',
    [],
  )
  const [prompt, setPrompt] = useState(defaultPrompt)
  const [rulesText, setRulesText] = useState(defaultRules)
  const [loadedRules, setLoadedRules] = useState<Rule[]>([])
  const [preview, setPreview] = useState<PreviewResult | null>(null)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    void loadRules()
  }, [])

  async function loadRules() {
    setBusy(true)
    setError('')
    try {
      const response = await fetch(`${apiBaseUrl}/v1/rules`)
      if (!response.ok) {
        throw new Error(`rules load failed: ${response.status}`)
      }
      const data = (await response.json()) as Rule[]
      setRulesText(JSON.stringify(data, null, 2))
      setLoadedRules(data)
      setMessage('Loaded saved rules from the proxy.')
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'failed to load rules')
    } finally {
      setBusy(false)
    }
  }

  async function saveRules() {
    setBusy(true)
    setError('')
    try {
      const parsed = JSON.parse(rulesText) as Rule[]
      const response = await fetch(`${apiBaseUrl}/v1/rules`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(parsed),
      })
      if (!response.ok) {
        throw new Error(`rules save failed: ${response.status}`)
      }
      const data = (await response.json()) as Rule[]
      setRulesText(JSON.stringify(data, null, 2))
      setLoadedRules(data)
      setMessage('Rules saved. The proxy will use the updated rules immediately.')
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : 'failed to save rules')
    } finally {
      setBusy(false)
    }
  }

  async function previewPrompt() {
    setBusy(true)
    setError('')
    try {
      const response = await fetch(`${apiBaseUrl}/v1/preview`, {
        method: 'POST',
        headers: { 'Content-Type': 'text/plain; charset=utf-8' },
        body: prompt,
      })
      if (!response.ok) {
        throw new Error(`preview failed: ${response.status}`)
      }
      const data = (await response.json()) as PreviewResult
      setPreview(data)
      setMessage('Preview updated. Review the redacted output and diff summary below.')
    } catch (previewError) {
      setError(previewError instanceof Error ? previewError.message : 'failed to preview prompt')
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="shell">
      <section className="hero">
        <div>
          <p className="eyebrow">Self-hosted LLM privacy proxy</p>
          <h1>memento-redact</h1>
          <p className="lede">
            Inspect prompts, edit rules, and preview deterministic redaction before traffic ever reaches a model provider.
          </p>
        </div>
        <div className="status-card">
          <span className="status-label">API</span>
          <strong>{apiBaseUrl}</strong>
          <span className={`status-pill ${busy ? 'busy' : 'ready'}`}>{busy ? 'Working' : 'Ready'}</span>
        </div>
      </section>

      <section className="grid">
        <div className="panel">
          <div className="panel-header">
            <h2>Prompt editor</h2>
            <button onClick={previewPrompt} disabled={busy}>Preview redaction</button>
          </div>
          <textarea
            value={prompt}
            onChange={(event) => setPrompt(event.target.value)}
            spellCheck={false}
            aria-label="Prompt editor"
          />
        </div>

        <div className="panel">
          <div className="panel-header">
            <h2>Rules editor</h2>
            <div className="actions">
              <button onClick={loadRules} disabled={busy}>Reload</button>
              <button onClick={saveRules} disabled={busy}>Save rules</button>
            </div>
          </div>
          <textarea
            value={rulesText}
            onChange={(event) => setRulesText(event.target.value)}
            spellCheck={false}
            aria-label="Rules editor"
          />
          <div className="rule-summary">
            <div className="rule-summary-header">
              <h3>Loaded rules</h3>
              <span>{loadedRules.length} rules</span>
            </div>
            {loadedRules.length === 0 ? (
              <p className="subtle">No rules were returned by the backend yet. Check the mounted config path in production.</p>
            ) : (
              <div className="rule-list">
                {loadedRules.map((rule) => (
                  <article key={`${rule.category ?? 'uncategorized'}:${rule.name}`} className="rule-card">
                    <div className="rule-card-heading">
                      <strong>{rule.name}</strong>
                      <span>{rule.category ?? 'uncategorized'}</span>
                    </div>
                    <p>{rule.description ?? 'No description provided.'}</p>
                    <code>{rule.replacement}</code>
                  </article>
                ))}
              </div>
            )}
          </div>
        </div>
      </section>

      <section className="grid lower-grid">
        <div className="panel wide">
          <div className="panel-header">
            <h2>Redaction diff</h2>
            <span className="subtle">Deterministic preview from the same API the CLI uses</span>
          </div>
          {preview ? (
            <>
              <div className="split">
                <article>
                  <h3>Original</h3>
                  <pre>{preview.original}</pre>
                </article>
                <article>
                  <h3>Redacted</h3>
                  <pre className="redacted">{preview.redacted}</pre>
                </article>
              </div>
              <div className="diff-list">
                {preview.diffs.length === 0 ? (
                  <p className="subtle">No redactions were needed.</p>
                ) : (
                  preview.diffs.map((diff, index) => (
                    <div key={`${diff.original}-${index}`} className="diff-item">
                      <span>Match {index + 1}</span>
                      <code>{diff.original}</code>
                      <strong>{diff.replacement}</strong>
                    </div>
                  ))
                )}
              </div>
            </>
          ) : (
            <p className="subtle">Run a preview to inspect the full diff visualization.</p>
          )}
        </div>

        <div className="panel side">
          <div className="panel-header">
            <h2>Notes</h2>
          </div>
          <ul className="notes">
            <li>Rules are saved through <code>/v1/rules</code> and applied immediately.</li>
            <li>The CLI and Web UI call the same preview and proxy endpoints.</li>
            <li>Keep regular expressions anchored and deterministic for predictable redaction.</li>
          </ul>
        </div>
      </section>

      {(message || error) && (
        <section className="footer-bar">
          {message && <p className="success">{message}</p>}
          {error && <p className="error">{error}</p>}
        </section>
      )}
    </main>
  )
}

export default App