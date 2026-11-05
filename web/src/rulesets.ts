import type { Ruleset } from './api'

// mergeRulesets adds incoming rulesets to current ones: a ruleset with the same name is replaced in place,
// new ones go to the end. With replaceAll, incoming becomes the whole list.
export function mergeRulesets(current: Ruleset[], incoming: Ruleset[], replaceAll: boolean): Ruleset[] {
  if (replaceAll) return incoming
  const merged = current.map((set) => incoming.find((n) => n.name === set.name) ?? set)
  return [...merged, ...incoming.filter((n) => !current.some((set) => set.name === n.name))]
}

// Switches are the on/off states of rulesets and their rules, matched by name. A profile is a saved set of them.
export type Switches = { name: string; on: boolean; rules: { name: string; on: boolean }[] }[]
export type Profile = { name: string; switches: Switches }

export const switchesOf = (sets: Ruleset[]): Switches =>
  sets.map((set) => ({ name: set.name, on: set.enabled, rules: set.rules.map((r) => ({ name: r.name, on: r.enabled })) }))

// applySwitches sets the saved states; rulesets and rules the profile does not know keep their state.
export function applySwitches(sets: Ruleset[], switches: Switches): Ruleset[] {
  return sets.map((set) => {
    const saved = switches.find((s) => s.name === set.name)
    if (!saved) return set
    return { ...set, enabled: saved.on, rules: set.rules.map((r) => ({ ...r, enabled: saved.rules.find((s) => s.name === r.name)?.on ?? r.enabled })) }
  })
}

// activeProfile is the first profile that would change nothing, so the picker can show it as selected.
export function activeProfile(sets: Ruleset[], profiles: Profile[]): string {
  const now = JSON.stringify(switchesOf(sets))
  return profiles.find((p) => JSON.stringify(switchesOf(applySwitches(sets, p.switches))) === now)?.name ?? ''
}
