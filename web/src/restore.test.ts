// Run with: npm test (Node strips the types; no test framework needed).
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { remember, restore } from './restore.ts'

const entry = (token: string, original: string, placeholder = '[X_#]') => ({ token, original, ruleset: 's', rule: 'r', count: 1, placeholder })
const map = [
  entry('[HOST_1]', 'web01'),
  entry('[DOMAIN_1].[TLD_1]', 'acme.sk'),
  entry('[DOMAIN_1].[TLD_2]', 'acme.com'),
  entry('[DOMAIN_2].[TLD_1]', 'corp.contoso.sk'),
  entry('[DOMAIN]', 'a.tld'), // a fixed placeholder: one token, several values
  entry('[DOMAIN]', 'b.tld'),
]
const text = (answer: string) => restore(answer, map).map((p) => p.text).join('')

test('restores whole tokens, tokens next to each other, other case and escaped brackets', () => {
  assert.equal(text('Restart [HOST_1].[DOMAIN_1].[TLD_1] and \\[host_1\\], then [domain_1].[tld_2].'), 'Restart web01.acme.sk and web01, then acme.com.')
})

test('restores the parts of a per-part token used alone, with extra labels in the first part', () => {
  assert.equal(text('[DOMAIN_1] is on [TLD_2]; [DOMAIN_2] too'), 'acme is on com; corp.contoso too')
})

test('leaves placeholders it cannot restore and says why', () => {
  const pieces = restore('[DOMAIN] and [HOST_9] and [TLD_1]', map)
  assert.deepEqual(
    pieces.filter((p) => p.kind !== 'plain'),
    [
      { kind: 'ambiguous', text: '[DOMAIN]', values: ['a.tld', 'b.tld'] },
      { kind: 'unknown', text: '[HOST_9]' },
      { kind: 'restored', text: 'sk', token: '[TLD_1]' },
    ],
  )
})

test('bracketed words that are not numbered placeholders are ordinary text', () => {
  const answer = '$i = [array]::IndexOf($names, "web") + 1\n[string]$name = [PSCustomObject]@{}\n[INFO] done, [ ] todo, [x] done\nonly [host_9] is unknown'
  assert.deepEqual(
    restore(answer, map).filter((p) => p.kind !== 'plain'),
    [{ kind: 'unknown', text: '[host_9]' }],
  )
  assert.equal(text(answer), answer)
})

test('text without placeholders and an empty map pass through', () => {
  assert.equal(text('nothing here'), 'nothing here')
  assert.deepEqual(restore('', []), [])
  assert.deepEqual(restore('[X_1] stays', []), [{ kind: 'unknown', text: '[X_1]' }, { kind: 'plain', text: ' stays' }])
})

test('a copied text adds only values the conversation does not have yet', () => {
  const conversation = [entry('[HOST_1]', 'web01', '[HOST_#]')]
  const copied = [entry('[HOST_1]', 'WEB01', '[HOST_#]'), entry('[HOST_2]', 'db01', '[HOST_#]')]
  assert.deepEqual(remember(conversation, copied).map((e) => e.original), ['web01', 'db01'])
  assert.equal(remember(conversation, [conversation[0]]), conversation, 'nothing new keeps the same list')
})
