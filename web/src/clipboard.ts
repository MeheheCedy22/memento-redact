// Copies value and reports the outcome through setNote; on failure the clipboard keeps its old content.
export async function copyText(value: string, setNote: (note: string) => void) {
  if (!navigator.clipboard) {
    setNote('Copying needs HTTPS or localhost. Select the text and press Ctrl+C.')
    return
  }
  try {
    await navigator.clipboard.writeText(value)
  } catch {
    // the clipboard still holds what was copied before, often the unredacted original
    setNote('Copy failed. Select the text and press Ctrl+C.')
    return
  }
  setNote('Copied')
  setTimeout(() => setNote(''), 2000)
}
