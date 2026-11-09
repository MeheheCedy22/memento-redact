package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/MeheheCedy22/memento-redact/internal/redactor"
	"github.com/MeheheCedy22/memento-redact/internal/rules"
)

// main dispatches CLI subcommands against the memento-redact API.
func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "health":
		fs, api := newFlags("health")
		fs.Parse(os.Args[2:])
		os.Stdout.Write(request(*api, http.MethodGet, "/health", "", nil))
		fmt.Println()
	case "preview", "redact":
		runRedact(os.Args[1], os.Args[2:])
	case "restore":
		runRestore(os.Args[2:])
	case "rules":
		runRules(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
}

// runRedact prints the redacted text (redact) or the text and its map as JSON (preview). --rules uses a rules
// file instead of the saved rules; --conversation keeps the placeholders of earlier texts and adds this text's.
func runRedact(cmd string, args []string) {
	fs, api := newFlags(cmd)
	input := fs.String("input", "", "text to redact; stdin when empty")
	rulesFile := fs.String("rules", "", "redact with this YAML/JSON rules file instead of the saved rules")
	profileFile := fs.String("profile", "", "switch rulesets and rules on or off as in this profile file, for this run only")
	convFile := fs.String("conversation", "", "JSON file with the values of earlier texts; created if missing, this text's values are added")
	parse(fs, args)

	text := []byte(*input)
	if *input == "" {
		text = readInput("")
	}
	var sets []*rules.Ruleset // nil: the saved rules
	if *rulesFile != "" {
		sets = readRules(*rulesFile)
	}
	if *profileFile != "" {
		if sets == nil {
			sets = getRulesets(*api)
		}
		applyProfile(sets, readProfile(*profileFile))
	}
	var known []redactor.Entry
	if *convFile != "" {
		known = readConversation(*convFile)
	}
	data := preview(*api, string(text), sets, known)
	var res redactor.Result
	decode(data, &res)
	if *convFile != "" {
		writeConversation(*convFile, remember(known, res.Map))
	}
	for _, l := range res.Leaks {
		fmt.Fprintf(os.Stderr, "warning: %q is still in the text %dx, though %s / %s covers it\n", l.Value, l.Count, l.Ruleset, l.Rule)
	}
	if cmd == "preview" {
		printJSON(data)
		return
	}
	fmt.Print(res.Redacted)
}

// runRestore puts the real values from a conversation file back into an LLM answer, like the Restore page.
// It runs locally: the answer is never sent to the server.
func runRestore(args []string) {
	fs := flag.NewFlagSet("restore", flag.ExitOnError)
	input := fs.String("input", "", "the answer; stdin when empty")
	convFile := fs.String("conversation", "", "the file written by redact --conversation (required)")
	parse(fs, args)
	if *convFile == "" {
		fatal(fmt.Errorf("restore needs --conversation FILE, the file written by redact --conversation"))
	}
	var conv []redactor.Entry
	decode(readInput(*convFile), &conv)
	answer := []byte(*input)
	if *input == "" {
		answer = readInput("")
	}
	text, notes := restore(string(answer), conv)
	fmt.Print(text)
	for _, note := range notes {
		fmt.Fprintln(os.Stderr, note)
	}
}

// preview redacts text with sets (nil: the saved rules), continuing the numbering of known.
func preview(api, text string, sets []*rules.Ruleset, known []redactor.Entry) []byte {
	body, err := json.Marshal(struct {
		Text     string           `json:"text"`
		Rulesets []*rules.Ruleset `json:"rulesets,omitempty"`
		Known    []redactor.Entry `json:"known,omitempty"`
	}{text, sets, known})
	if err != nil {
		fatal(err)
	}
	return request(api, http.MethodPost, "/v1/preview", "application/json", body)
}

// readRules reads and checks a YAML/JSON rules file.
func readRules(path string) []*rules.Ruleset {
	sets, err := rules.Parse(readInput(path))
	if err == nil && len(sets) == 0 {
		err = fmt.Errorf("no rulesets in the file")
	}
	if err != nil {
		fatal(fmt.Errorf("%s: %w", path, err))
	}
	return sets
}

// readConversation reads a conversation file; a missing file is a new conversation.
func readConversation(path string) []redactor.Entry {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		fatal(err)
	}
	var conv []redactor.Entry
	decode(data, &conv)
	return conv
}

// writeConversation saves a conversation file. It holds the real values, so only the owner may read it.
func writeConversation(path string, conv []redactor.Entry) {
	data, err := json.MarshalIndent(conv, "", "  ")
	if err == nil {
		err = os.WriteFile(path, append(data, '\n'), 0o600)
	}
	if err != nil {
		fatal(err)
	}
}

// runRules manages the saved rulesets: list, export, import, reset, check, profile, enable, disable.
func runRules(args []string) {
	if len(args) == 0 {
		usage()
		os.Exit(1)
	}
	fs, api := newFlags("rules " + args[0])
	add := fs.Bool("add", false, "import: add the file's rulesets to yours (one with the same name replaces yours) instead of replacing all")
	only := fs.String("only", "", "import: only these rulesets from the file, comma-separated, any case")
	arg := parse(fs, args[1:])

	switch args[0] {
	case "list":
		defaults := getDefaults(*api)
		for _, set := range getRulesets(*api) {
			note := ""
			// as on the Rules page: a preset whose factory version differs from the one it was copied from
			if slices.ContainsFunc(defaults, func(p *rules.Ruleset) bool { return p.Name == set.Name && p.Preset != set.Preset }) {
				note = "   (update available)"
			}
			fmt.Printf("%s %s%s\n", mark(set.Enabled), set.Name, note)
			for _, r := range set.Rules {
				fmt.Printf("    %s %-36s %-5s %s (%d patterns)\n", mark(r.Enabled), r.Name, r.Type, r.Placeholder, len(r.Patterns))
			}
		}
	case "export":
		path := "/v1/rulesets?format=yaml"
		if arg != "" {
			path += "&name=" + url.QueryEscape(arg)
		}
		os.Stdout.Write(request(*api, http.MethodGet, path, "", nil))
	case "import":
		incoming := readRules(arg)
		if *only != "" {
			incoming = pick(incoming, strings.Split(*only, ","))
		}
		if *add {
			save(*api, merge(getRulesets(*api), incoming))
		} else {
			save(*api, incoming)
		}
		for _, set := range incoming {
			fmt.Fprintf(os.Stderr, "%s: imported (saved)\n", set.Name)
		}
		printWarnings(os.Stderr, incoming)
	case "reset":
		if arg != "" {
			resetOne(*api, arg)
			return
		}
		request(*api, http.MethodPost, "/v1/rulesets/reset", "", nil)
		fmt.Fprintln(os.Stderr, "rulesets reset to factory defaults")
	case "check":
		var sets []*rules.Ruleset
		if arg != "" {
			sets = readRules(arg)
		} else if sets = getRulesets(*api); rules.Compile(sets) != nil {
			fatal(fmt.Errorf("the saved rules do not compile with this CLI version"))
		}
		printWarnings(os.Stdout, sets)
		checkSamples(*api, sets)
	case "profile":
		sets := getRulesets(*api)
		if arg == "" {
			data, err := json.MarshalIndent(profileOf(sets), "", "  ")
			if err != nil {
				fatal(err)
			}
			fmt.Println(string(data))
			return
		}
		applyProfile(sets, readProfile(arg))
		save(*api, sets)
		fmt.Fprintln(os.Stderr, "profile applied (saved)")
	case "enable", "disable":
		setEnabled(*api, arg, args[0] == "enable")
	default:
		usage()
		os.Exit(1)
	}
}

// setEnabled toggles a ruleset ("Network") or a single rule ("Network/Public IPv4") and saves.
func setEnabled(api, target string, enabled bool) {
	setName, ruleName, hasRule := strings.Cut(target, "/")
	if setName == "" {
		fatal(fmt.Errorf("name a ruleset or ruleset/rule, e.g. \"Network\" or \"Network/Public IPv4\""))
	}
	sets := getRulesets(api)
	changed := false
	for _, set := range sets {
		if !strings.EqualFold(set.Name, setName) {
			continue
		}
		if !hasRule {
			set.Enabled, changed = enabled, true
		}
		for _, r := range set.Rules {
			if hasRule && strings.EqualFold(r.Name, ruleName) {
				r.Enabled, changed = enabled, true
			}
		}
	}
	if !changed {
		fatal(fmt.Errorf("%q not found; see: memento-redact-cli rules list", target))
	}
	save(api, sets)
	fmt.Fprintf(os.Stderr, "%s: enabled=%v (saved)\n", target, enabled)
}

// resetOne replaces one ruleset with its factory version, or adds it back if it was deleted
// (Restore preset and Add preset on the Rules page).
func resetOne(api, name string) {
	defaults := getDefaults(api)
	i := slices.IndexFunc(defaults, func(p *rules.Ruleset) bool { return strings.EqualFold(p.Name, name) })
	if i < 0 {
		fatal(fmt.Errorf("%q is not a factory preset; see: memento-redact-cli rules list", name))
	}
	current := getRulesets(api)
	// sample lines are not part of the preset: keep yours, by rule name
	if j := slices.IndexFunc(current, func(s *rules.Ruleset) bool { return s.Name == defaults[i].Name }); j >= 0 {
		for _, r := range defaults[i].Rules {
			for _, old := range current[j].Rules {
				if r.Name == old.Name {
					r.Samples = old.Samples
				}
			}
		}
	}
	save(api, merge(current, defaults[i:i+1]))
	fmt.Fprintf(os.Stderr, "%s: factory version restored (saved)\n", defaults[i].Name)
}

// pick returns the rulesets of a file named in names (any case), in file order, as the Import… picker does.
func pick(sets []*rules.Ruleset, names []string) []*rules.Ruleset {
	var out []*rules.Ruleset
	for _, name := range names {
		name = strings.TrimSpace(name)
		if !slices.ContainsFunc(sets, func(s *rules.Ruleset) bool { return strings.EqualFold(s.Name, name) }) {
			fatal(fmt.Errorf("%q is not in the file", name))
		}
	}
	for _, set := range sets {
		if slices.ContainsFunc(names, func(n string) bool { return strings.EqualFold(strings.TrimSpace(n), set.Name) }) {
			out = append(out, set)
		}
	}
	return out
}

// A profile is the on/off state of rulesets and their rules, matched by name: the format the Redact page
// keeps under "Save switches as…".
type switchState struct {
	Name string `json:"name"`
	On   bool   `json:"on"`
}

type setSwitches struct {
	switchState
	Rules []switchState `json:"rules"`
}

func profileOf(sets []*rules.Ruleset) []setSwitches {
	out := []setSwitches{}
	for _, set := range sets {
		s := setSwitches{switchState{set.Name, set.Enabled}, []switchState{}}
		for _, r := range set.Rules {
			s.Rules = append(s.Rules, switchState{r.Name, r.Enabled})
		}
		out = append(out, s)
	}
	return out
}

// applyProfile sets the switches of p. Rulesets and rules it does not name keep their state, as in the UI.
func applyProfile(sets []*rules.Ruleset, p []setSwitches) {
	for _, set := range sets {
		for _, s := range p {
			if s.Name != set.Name {
				continue
			}
			set.Enabled = s.On
			for _, r := range set.Rules {
				for _, rs := range s.Rules {
					if rs.Name == r.Name {
						r.Enabled = rs.On
					}
				}
			}
		}
	}
}

func readProfile(path string) []setSwitches {
	var p []setSwitches
	if err := json.Unmarshal(readInput(path), &p); err != nil {
		var syntax *json.SyntaxError
		if errors.As(err, &syntax) {
			fatal(fmt.Errorf("%s: %w", path, err))
		}
		fatal(fmt.Errorf("%s: not a profile file; rules profile prints one", path))
	}
	return p
}

// merge adds incoming rulesets to current ones: one with the same name is replaced in place, new ones go
// to the end, as Import → Add on the Rules page.
func merge(current, incoming []*rules.Ruleset) []*rules.Ruleset {
	out := slices.Clone(current)
	for _, set := range incoming {
		if i := slices.IndexFunc(out, func(c *rules.Ruleset) bool { return c.Name == set.Name }); i >= 0 {
			out[i] = set
		} else {
			out = append(out, set)
		}
	}
	return out
}

// printWarnings prints the rules that probably redact too much or too little, as the Rules page marks them.
func printWarnings(w io.Writer, sets []*rules.Ruleset) {
	for _, warning := range rules.Lint(sets) {
		fmt.Fprintf(w, "warning: %s / %s: %s\n", warning.Ruleset, warning.Rule, warning.Message)
	}
}

// checkSamples prints each rule's sample lines redacted by that rule alone, as the Rules page shows them.
func checkSamples(api string, sets []*rules.Ruleset) {
	found := false
	for _, set := range sets {
		for _, r := range set.Rules {
			samples := strings.Join(r.Samples, "\n")
			if strings.TrimSpace(samples) == "" {
				continue
			}
			found = true
			one := *r
			one.Enabled = true // samples are checked even when the rule is off
			var res redactor.Result
			decode(preview(api, samples, []*rules.Ruleset{{Name: set.Name, Enabled: true, Rules: []*rules.Rule{&one}}}, nil), &res)
			fmt.Printf("%s / %s\n", set.Name, r.Name)
			for i, line := range strings.Split(res.Redacted, "\n") {
				note := ""
				if strings.TrimSpace(line) != "" && i < len(r.Samples) && line == r.Samples[i] {
					note = "   (not redacted)"
				}
				fmt.Printf("  %d  %s%s\n", i+1, line, note)
			}
		}
	}
	if !found {
		fmt.Fprintln(os.Stderr, "no rule has sample lines")
	}
}

func getRulesets(api string) []*rules.Ruleset {
	var sets []*rules.Ruleset
	decode(request(api, http.MethodGet, "/v1/rulesets", "", nil), &sets)
	return sets
}

func getDefaults(api string) []*rules.Ruleset {
	var sets []*rules.Ruleset
	decode(request(api, http.MethodGet, "/v1/rulesets/defaults", "", nil), &sets)
	return sets
}

// save replaces all saved rulesets with sets.
func save(api string, sets []*rules.Ruleset) {
	body, err := json.Marshal(sets)
	if err != nil {
		fatal(err)
	}
	request(api, http.MethodPut, "/v1/rulesets", "application/json", body)
}

func mark(enabled bool) string {
	if enabled {
		return "[x]"
	}
	return "[ ]"
}

func newFlags(name string) (*flag.FlagSet, *string) {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	return fs, fs.String("api", getenv("API_URL", "http://localhost:8080"), "memento-redact API base URL")
}

// parse reads flags anywhere among args, so "rules import FILE --add" works like "rules import --add FILE",
// and returns the first other argument, or "".
func parse(fs *flag.FlagSet, args []string) string {
	var rest []string
	for {
		fs.Parse(args)
		if fs.NArg() == 0 {
			break
		}
		rest = append(rest, fs.Arg(0))
		args = fs.Args()[1:]
	}
	if len(rest) == 0 {
		return ""
	}
	return rest[0]
}

// readInput reads a file, or stdin when path is empty or "-".
func readInput(path string) []byte {
	var data []byte
	var err error
	if path == "" || path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		fatal(err)
	}
	return data
}

// request calls the API and returns the response body, exiting with the server's message on errors.
func request(api, method, path, contentType string, body []byte) []byte {
	req, err := http.NewRequest(method, strings.TrimRight(api, "/")+path, bytes.NewReader(body))
	if err != nil {
		fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		fatal(err)
	}
	if resp.StatusCode >= 400 {
		fatal(fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, strings.TrimSpace(string(data))))
	}
	return data
}

func decode(data []byte, v any) {
	if err := json.Unmarshal(data, v); err != nil {
		fatal(err)
	}
}

func printJSON(data []byte) {
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, data, "", "  "); err != nil {
		os.Stdout.Write(data)
		return
	}
	pretty.WriteTo(os.Stdout)
	fmt.Println()
}

// usage prints CLI usage information.
func usage() {
	fmt.Fprint(os.Stderr, `memento-redact-cli <command> [--api URL]   (API_URL env sets the default URL)

  health                          check the API
  redact  [--input TEXT]          print the redacted text (reads stdin without --input)
          [--rules FILE]          redact with a rules file instead of the saved rules
          [--profile FILE]        switch rulesets and rules as in a profile file, for this run only
          [--conversation FILE]   keep the placeholders of earlier texts, add this text's values
  preview [same flags]            print redacted text and the token map as JSON
  restore --conversation FILE     put the real values back into an LLM answer (stdin or --input)
  rules list                      show rulesets and rules with their on/off state and updates
  rules export [NAME]             print saved rulesets, or only ruleset NAME, as YAML
  rules import [--add] [--only NAMES] [FILE|-]
                                  replace all rulesets with a YAML/JSON file (or stdin);
                                  --add adds them instead (same name replaces yours),
                                  --only takes just these rulesets ("Network,Secrets")
  rules reset [NAME]              restore all factory presets, or only preset NAME
  rules check [FILE]              show rule warnings, and each rule's sample lines redacted by that rule alone
  rules profile [FILE]            print the on/off switches as a profile file, or apply one and save
  rules enable|disable NAME       toggle a ruleset ("Network") or rule ("Network/Public IPv4")

flags can go before or after the other arguments

examples:
  memento-redact-cli redact < app.log > redacted.txt
  memento-redact-cli redact --conversation chat.json < app.log && memento-redact-cli restore --conversation chat.json < answer.txt
  memento-redact-cli rules export > rulesets.yml && $EDITOR rulesets.yml && memento-redact-cli rules import rulesets.yml
`)
}

// getenv returns an environment variable or fallback when unset.
func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// fatal prints a CLI error and exits.
func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
