import { useEffect, useLayoutEffect, useRef, type TextareaHTMLAttributes } from 'react'

// A textarea that grows with its content, so every line stays visible without an inner scrollbar.
export default function AutoTextarea(props: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  const ref = useRef<HTMLTextAreaElement>(null)
  const fit = () => {
    const el = ref.current
    if (!el) return
    el.style.height = 'auto'
    el.style.height = `${el.scrollHeight + el.offsetHeight - el.clientHeight}px`
  }
  useLayoutEffect(fit, [props.value])
  // refit when the surrounding box changes size: window resize, or the user dragging a resizable parent
  useEffect(() => {
    const parent = ref.current?.parentElement
    if (!parent) return
    const observer = new ResizeObserver(() => fit())
    observer.observe(parent)
    return () => observer.disconnect()
  }, [])
  return <textarea ref={ref} {...props} />
}
