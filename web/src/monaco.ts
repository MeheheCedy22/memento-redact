// Bundle Monaco locally instead of @monaco-editor/react's default CDN download:
// the UI must work offline and never send anything to third parties.
import * as monaco from 'monaco-editor/editor/editor.main'
import EditorWorker from 'monaco-editor/editor/editor.worker?worker'
import { loader } from '@monaco-editor/react'

self.MonacoEnvironment = { getWorker: () => new EditorWorker() }
loader.config({ monaco })
