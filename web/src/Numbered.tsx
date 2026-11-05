import type { ReactNode, TextareaHTMLAttributes } from 'react'
import AutoTextarea from './AutoTextarea'

// A run of text, optionally wrapped (e.g. in a <mark>) when shown.
export type Part = { text: string; wrap?: (text: string, key: number) => ReactNode }

// NumberedText shows parts as numbered lines. Each line is its own block, so a long line wraps and keeps
// its number on the first row; a part that spans lines is split at the line breaks.
export function NumberedText({ parts }: { parts: Part[] }) {
  const lines: ReactNode[][] = [[]]
  let key = 0
  for (const part of parts) {
    part.text.split('\n').forEach((piece, i) => {
      if (i > 0) lines.push([])
      if (piece) lines[lines.length - 1].push(part.wrap ? part.wrap(piece, key++) : piece)
    })
  }
  return (
    <div className="lines">
      {lines.map((line, i) => (
        <div key={i}>{line}</div>
      ))}
    </div>
  )
}

// NumberedTextarea draws the same text, invisible, behind a transparent textarea. The copy wraps exactly
// like the textarea, so the line numbers drawn next to it stay at the right rows.
export function NumberedTextarea(props: TextareaHTMLAttributes<HTMLTextAreaElement> & { value: string }) {
  return (
    <div className="numbered">
      <div className={`lines ${props.className ?? ''}`} aria-hidden="true">
        {props.value.split('\n').map((line, i) => (
          <div key={i}>{line}</div>
        ))}
      </div>
      <AutoTextarea {...props} />
    </div>
  )
}
