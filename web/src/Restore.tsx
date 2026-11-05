import { useState, type RefObject } from 'react'
import type { MapEntry } from './api'
import { copyText } from './clipboard'
import { NumberedText, NumberedTextarea, type Part } from './Numbered'
import { restore } from './restore'

type Props = {
  answer: string
  setAnswer: (answer: string) => void
  map: MapEntry[]
  outputRef: RefObject<HTMLDivElement>
}

// The Restore page's text pair: the LLM's answer, and the same answer with the real values put back from the
// map of the current redaction. It runs in the browser only; neither text is sent to the server.
export default function Restore({ answer, setAnswer, map, outputRef }: Props) {
  const [copyNote, setCopyNote] = useState('')
  const pieces = restore(answer, map)
  const restored = pieces.map((p) => p.text).join('')
  const count = pieces.filter((p) => p.kind === 'restored').length
  const parts: Part[] = pieces.map((p) => {
    switch (p.kind) {
      case 'plain':
        return { text: p.text }
      case 'restored':
        return { text: p.text, wrap: (t, key) => <mark key={key} data-token={p.token} title={`was ${p.token}`}>{t}</mark> }
      case 'ambiguous':
        return { text: p.text, wrap: (t, key) => <mark key={key} className="unresolved" title={`Stands for several values: ${p.values.join(', ')}`}>{t}</mark> }
      case 'unknown':
        return { text: p.text, wrap: (t, key) => <mark key={key} className="unresolved" title="Not in the map of the current redaction">{t}</mark> }
    }
  })
  const unresolved = [...new Set(pieces.filter((p) => p.kind === 'ambiguous' || p.kind === 'unknown').map((p) => p.text))]

  return (
    <>
      <section>
        <div className="section-head">
          <h2>Answer from the LLM</h2>
          <button onClick={() => setAnswer('')} title="Clear the answer.">Clear</button>
        </div>
        <NumberedTextarea
          className="mono"
          value={answer}
          onChange={(e) => setAnswer(e.target.value)}
          placeholder="Paste the answer here. Its placeholders are replaced with the real values of the current redaction."
          spellCheck={false}
          aria-label="Answer from the LLM"
        />
      </section>

      <section>
        <div className="section-head">
          <h2>Restored</h2>
          <span className="muted" title="Placeholders replaced with their real values.">{answer && `${count} restored`}</span>
          {unresolved.length > 0 && (
            <span className="error-text" title={`Outlined in the text; hover one for why: ${unresolved.join(', ')}`}>
              {unresolved.length} not restored
            </span>
          )}
          {copyNote && <span className="note">{copyNote}</span>}
          <button className="primary" onClick={() => copyText(restored, setCopyNote)} disabled={!answer} title="Copy the restored text. It contains the real values.">
            Copy
          </button>
        </div>
        <div ref={outputRef} className="output mono" aria-label="Restored text">
          {answer ? (
            <NumberedText parts={parts} />
          ) : (
            <span className="muted">{map.length ? 'The restored answer appears here.' : 'Redact a text first: the answer is restored with the map of that redaction.'}</span>
          )}
        </div>
      </section>
    </>
  )
}
