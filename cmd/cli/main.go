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
)

// cliConfig captures runtime options for the CLI client.
type cliConfig struct {
	apiURL string
	input  string
	mode   string
}

// main dispatches CLI subcommands against the SafePrompt Proxy API.
func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "health":
		runHealth(os.Args[2:])
	case "preview":
		runPreview(os.Args[2:])
	case "proxy":
		runProxy(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
}

// runHealth checks the API health endpoint.
func runHealth(args []string) {
	fs := flag.NewFlagSet("health", flag.ExitOnError)
	apiURL := fs.String("api", getenv("API_URL", "http://localhost:8080"), "proxy API base URL")
	fs.Parse(args)

	resp, err := http.Get(strings.TrimRight(*apiURL, "/") + "/health")
	if err != nil {
		fatal(err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(os.Stdout, resp.Body)
	if resp.StatusCode >= 400 {
		os.Exit(1)
	}
}

// runPreview sends text to the preview endpoint and prints the redaction result.
func runPreview(args []string) {
	config := parseInput(args)
	callEndpoint(config.apiURL, "/v1/preview", config.input)
}

// runProxy sends text to the proxy endpoint and prints the provider response.
func runProxy(args []string) {
	config := parseInput(args)
	callEndpoint(config.apiURL, "/v1/proxy", config.input)
}

// parseInput parses the CLI flags and resolves the payload to send.
func parseInput(args []string) cliConfig {
	fs := flag.NewFlagSet("payload", flag.ExitOnError)
	apiURL := fs.String("api", getenv("API_URL", "http://localhost:8080"), "proxy API base URL")
	input := fs.String("input", "", "payload text; defaults to stdin if empty")
	fs.Parse(args)
	resolved := *input
	if resolved == "" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fatal(err)
		}
		resolved = string(data)
	}
	return cliConfig{apiURL: *apiURL, input: resolved}
}

// callEndpoint posts a payload to the specified endpoint and streams the result.
func callEndpoint(apiURL, path, payload string) {
	body := bytes.NewBufferString(payload)
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(apiURL, "/")+path, body)
	if err != nil {
		fatal(err)
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fatal(err)
	}
	defer resp.Body.Close()

	if strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, mustRead(resp.Body), "", "  "); err == nil {
			_, _ = io.Copy(os.Stdout, &pretty)
			_, _ = io.WriteString(os.Stdout, "\n")
			if resp.StatusCode >= 400 {
				os.Exit(1)
			}
			return
		}
	}

	_, _ = io.Copy(os.Stdout, resp.Body)
	if resp.StatusCode >= 400 {
		os.Exit(1)
	}
}

// mustRead reads the response body or exits.
func mustRead(r io.Reader) []byte {
	data, err := io.ReadAll(r)
	if err != nil {
		fatal(err)
	}
	return data
}

// usage prints CLI usage information.
func usage() {
	fmt.Fprintln(os.Stderr, "memento-redact cli commands: health, preview, proxy")
	fmt.Fprintln(os.Stderr, "examples:")
	fmt.Fprintln(os.Stderr, "  memento-redact-cli health")
	fmt.Fprintln(os.Stderr, "  memento-redact-cli preview --input \"hello alice@example.com\"")
	fmt.Fprintln(os.Stderr, "  cat prompt.txt | memento-redact-cli proxy")
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
