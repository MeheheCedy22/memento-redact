// Copies value and reports the outcome through setNote; on failure the clipboard keeps its old content.
// It returns whether the copy worked.
export async function copyText(value: string, setNote: (note: string) => void): Promise<boolean> {
  if (!navigator.clipboard) {
    setNote('Copying needs HTTPS or localhost. Select the text and press Ctrl+C.')
    return false
  }
  try {
    await navigator.clipboard.writeText(value)
  } catch {
    // the clipboard still holds what was copied before, often the unredacted original
    setNote('Copy failed. Select the text and press Ctrl+C.')
    return false
  }
  setNote('Copied')
  setTimeout(() => setNote(''), 2000)
  return true
}
