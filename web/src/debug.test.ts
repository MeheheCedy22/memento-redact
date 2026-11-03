// Run with: npm test (Node strips the types; no test framework needed).
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { debugReport } from './debug.ts'

const map = [
  { token: '[HOST_1]', original: 'web01', ruleset: 'Org', rule: 'Hosts', count: 2 },
  { token: '[SECRET_1]', original: 'hunter2', ruleset: 'Secrets', rule: 'Pass', count: 1 },
  { token: '[DOMAIN]', original: 'a.sk', ruleset: 'Org', rule: 'Domains', count: 1 },
  { token: '[DOMAIN]', original: 'b.sk', ruleset: 'Org', rule: 'Domains', count: 1 },
]
const sets = [
  {
    name: 'Org',
    enabled: true,
    rules: [
      { name: 'Hosts', enabled: true, type: 'regex' as const, patterns: ['(\\w+)\\.acme'], keep: '^test$', placeholder: '[HOST_#]' },
      { name: 'Domains', enabled: true, type: 'words' as const, patterns: ['a.sk', 'b.sk'], placeholder: '[DOMAIN]' },
    ],
  },
]
const redacted = '[HOST_1].acme login [SECRET_1]\nsee [DOMAIN] and [DOMAIN]\nuser=jdoe\n[HOST_1].acme again'

test('restores only the marked values and keeps words lists out', () => {
  const report = debugReport(
    redacted,
    map,
    [
      { entry: map[0], reason: 'false positive' },
      { entry: map[2], reason: 'wrong number' },
    ],
    ['jdoe'],
    sets,
  )
  assert.equal(
    report,
    `memento-redact debug report

1. false positive (should not be redacted): [HOST_1] = "web01", 2x
   rule: Org › Hosts, regex, placeholder [HOST_#]
   pattern: (\\w+)\\.acme
   keep: ^test$
   redacted:   [HOST_1].acme login [SECRET_1]
   unredacted: web01.acme login [SECRET_1]
   redacted:   [HOST_1].acme again
   unredacted: web01.acme again

2. wrong number (one value got several numbers, or several values share one): [DOMAIN] = "a.sk", 1x
   rule: Org › Domains, words, placeholder [DOMAIN]
   line:       see [DOMAIN] and [DOMAIN]

3. false negative (not redacted): "jdoe"
   line:       user=jdoe
`,
  )
})

test('works without marks', () => {
  assert.equal(debugReport('plain', [], [], ['multi\nline'], []), 'memento-redact debug report\n\n1. false negative (not redacted): "multi\\nline"\n')
})
