package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

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
	case "preview":
		api, input := parseInput(os.Args[2:])
		printJSON(request(api, http.MethodPost, "/v1/preview", "text/plain; charset=utf-8", input))
	case "redact":
		api, input := parseInput(os.Args[2:])
		var res struct {
			Redacted string `json:"redacted"`
		}
		decode(request(api, http.MethodPost, "/v1/preview", "text/plain; charset=utf-8", input), &res)
		fmt.Print(res.Redacted)
	case "proxy":
		api, input := parseInput(os.Args[2:])
		os.Stdout.Write(request(api, http.MethodPost, "/v1/proxy", "text/plain; charset=utf-8", input))
	case "rules":
		runRules(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
}

// runRules manages the saved rulesets: list, export, import, reset, enable, disable.
func runRules(args []string) {
	if len(args) == 0 {
		usage()
		os.Exit(1)
	}
	fs, api := newFlags("rules " + args[0])
	fs.Parse(args[1:])

	switch args[0] {
	case "list":
		for _, set := range getRulesets(*api) {
			fmt.Printf("%s %s\n", mark(set.Enabled), set.Name)
			for _, r := range set.Rules {
				fmt.Printf("    %s %-36s %-5s %s (%d patterns)\n", mark(r.Enabled), r.Name, r.Type, r.Placeholder, len(r.Patterns))
			}
		}
	case "export":
		os.Stdout.Write(request(*api, http.MethodGet, "/v1/rulesets?format=yaml", "", nil))
	case "import":
		data := readInput(fs.Arg(0))
		request(*api, http.MethodPut, "/v1/rulesets", "application/yaml", data)
		fmt.Fprintln(os.Stderr, "rulesets imported and saved")
	case "reset":
		request(*api, http.MethodPost, "/v1/rulesets/reset", "", nil)
		fmt.Fprintln(os.Stderr, "rulesets reset to factory defaults")
	case "enable", "disable":
		setEnabled(*api, fs.Arg(0), args[0] == "enable")
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
	body, err := json.Marshal(sets)
	if err != nil {
		fatal(err)
	}
	request(api, http.MethodPut, "/v1/rulesets", "application/json", body)
	fmt.Fprintf(os.Stderr, "%s: enabled=%v (saved)\n", target, enabled)
}

func getRulesets(api string) []*rules.Ruleset {
	var sets []*rules.Ruleset
	decode(request(api, http.MethodGet, "/v1/rulesets", "", nil), &sets)
	return sets
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

// parseInput parses the CLI flags and resolves the payload to send.
func parseInput(args []string) (string, []byte) {
	fs, api := newFlags("payload")
	input := fs.String("input", "", "payload text; defaults to stdin if empty")
	fs.Parse(args)
	if *input != "" {
		return *api, []byte(*input)
	}
	return *api, readInput("")
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
  preview [--input TEXT]          print redacted text and the token map as JSON
  proxy   [--input TEXT]          redact and forward to PROVIDER_URL, print the response
  rules list                      show rulesets and rules with their on/off state
  rules export                    print saved rulesets as YAML
  rules import [FILE|-]           replace all rulesets with a YAML/JSON file (or stdin)
  rules reset                     restore factory default rulesets
  rules enable|disable NAME       toggle a ruleset ("Network") or rule ("Network/Public IPv4")

examples:
  kubectl get pods -o wide | memento-redact-cli redact | wl-copy
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
