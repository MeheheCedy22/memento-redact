/// <reference types="vite/client" />

// editor.main = editor + built-in contributions without the heavy language services of the package root.
declare module 'monaco-editor/editor/editor.main' {
  export * from 'monaco-editor'
}
