// Run with: npm test (Node strips the types; no test framework needed).
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { nextSort, sortMap } from './mapsort.ts'

const entry = (token: string, original: string, rule: string, count: number) => ({ token, original, ruleset: 'Net', rule, count, placeholder: '[X_#]' })
const map = [
  entry('[HOST_10]', 'web10', 'Hosts', 1),
  entry('[IP_1]', '8.8.8.8', 'Public IPv4', 3),
  entry('[HOST_2]', 'Web02', 'Hosts', 3),
]
const tokens = (sort: Parameters<typeof sortMap>[1]) => sortMap(map, sort).map((e) => e.token)

test('sorts by each column, numbers inside text by value, case ignored', () => {
  assert.deepEqual(tokens({ key: 'token', desc: false }), ['[HOST_2]', '[HOST_10]', '[IP_1]'])
  assert.deepEqual(tokens({ key: 'token', desc: true }), ['[IP_1]', '[HOST_10]', '[HOST_2]'])
  assert.deepEqual(tokens({ key: 'original', desc: false }), ['[IP_1]', '[HOST_2]', '[HOST_10]'])
  assert.deepEqual(tokens({ key: 'rule', desc: false }), ['[HOST_10]', '[HOST_2]', '[IP_1]'])
})

test('equal rows keep their order of appearance', () => {
  assert.deepEqual(tokens({ key: 'count', desc: true }), ['[IP_1]', '[HOST_2]', '[HOST_10]'])
  assert.deepEqual(tokens({ key: 'count', desc: false }), ['[HOST_10]', '[IP_1]', '[HOST_2]'])
})

test('no sort keeps the map as it is; a header click cycles ascending, descending, off', () => {
  assert.equal(sortMap(map, null), map)
  const asc = nextSort(null, 'count')
  assert.deepEqual(asc, { key: 'count', desc: false })
  assert.deepEqual(nextSort(asc, 'count'), { key: 'count', desc: true })
  assert.equal(nextSort({ key: 'count', desc: true }, 'count'), null)
  assert.deepEqual(nextSort({ key: 'count', desc: true }, 'token'), { key: 'token', desc: false })
})
