import type { MapEntry } from './api'

// Sorting of the map table by a column. null keeps the order of first appearance.
export type SortKey = 'token' | 'original' | 'rule' | 'count'
export type Sort = { key: SortKey; desc: boolean } | null

const text = (e: MapEntry, key: Exclude<SortKey, 'count'>) => (key === 'rule' ? `${e.ruleset} › ${e.rule}` : e[key])

// sortMap returns the entries sorted by one column. Text compares numbers inside it by value ([HOST_2] before
// [HOST_10]) and ignores case; equal rows keep their order of appearance.
export function sortMap(entries: MapEntry[], sort: Sort): MapEntry[] {
  if (!sort) return entries
  const sign = sort.desc ? -1 : 1
  const { key } = sort
  return [...entries].sort((a, b) =>
    sign * (key === 'count' ? a.count - b.count : text(a, key).localeCompare(text(b, key), undefined, { numeric: true, sensitivity: 'base' })),
  )
}

// nextSort is what a click on a column header does: ascending, then descending, then back to appearance order.
export const nextSort = (sort: Sort, key: SortKey): Sort => (sort?.key !== key ? { key, desc: false } : sort.desc ? null : { key, desc: true })
