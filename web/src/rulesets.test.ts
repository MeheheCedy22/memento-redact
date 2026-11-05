// Run with: npm test (Node strips the types; no test framework needed).
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { activeProfile, applySwitches, mergeRulesets, switchesOf } from './rulesets.ts'

const rule = (name: string, enabled: boolean, pattern = name) => ({ name, enabled, type: 'words' as const, patterns: [pattern], placeholder: '[X_#]' })
const set = (name: string, enabled: boolean, rules = [rule('r', true)]) => ({ name, enabled, rules })

test('an import adds new rulesets at the end and replaces same-named ones in place', () => {
  const current = [set('Network', true, [rule('IP', true, 'old')]), set('Mine', true)]
  const incoming = [set('Network', true, [rule('IP', true, 'new')]), set('Extra', false)]
  assert.deepEqual(
    mergeRulesets(current, incoming, false).map((s) => [s.name, s.rules[0].patterns[0]]),
    [['Network', 'new'], ['Mine', 'r'], ['Extra', 'r']],
  )
  assert.deepEqual(mergeRulesets(current, incoming, true), incoming)
})

test('a profile restores saved switches and leaves rulesets it does not know alone', () => {
  const saved = [set('Network', true, [rule('IP', true), rule('MAC', false)]), set('Secrets', true)]
  const profile = { name: 'logs', switches: switchesOf(saved) }
  const now = [set('Network', false, [rule('IP', true), rule('MAC', true)]), set('Secrets', true), set('Added later', false)]

  const applied = applySwitches(now, profile.switches)
  assert.deepEqual(switchesOf(applied), [...switchesOf(saved), { name: 'Added later', on: false, rules: [{ name: 'r', on: true }] }])
  assert.equal(activeProfile(applied, [profile]), 'logs')
  assert.equal(activeProfile(now, [profile]), '')
})
