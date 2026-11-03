package rules

import (
	"io"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Rule defines a redaction rule loaded from YAML.
type Rule struct {
	Category    string `yaml:"category" json:"category"`
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Matcher     string `yaml:"matcher" json:"matcher"`
	Pattern     string `yaml:"pattern" json:"pattern"`
	Replacement string `yaml:"replacement" json:"replacement"`
	Enabled     bool   `yaml:"enabled" json:"enabled"`

	re *regexp.Regexp
}

// Apply redacts input with the compiled rule if enabled.
func (r *Rule) Apply(input string) string {
	if r == nil || !r.Enabled {
		return input
	}
	if strings.EqualFold(r.Matcher, "literal") {
		if r.Pattern == "" {
			return input
		}
		return strings.ReplaceAll(input, r.Pattern, r.Replacement)
	}
	if r.re == nil {
		if r.Pattern == "" {
			return input
		}
		compiled, err := regexp.Compile(r.Pattern)
		if err != nil {
			return input
		}
		r.re = compiled
	}
	return r.re.ReplaceAllString(input, r.Replacement)
}

// LoadRules loads rules from a YAML file. If file missing, returns empty slice.
func LoadRules(path string) ([]*Rule, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	var rr []*Rule
	if err := yaml.Unmarshal(data, &rr); err != nil {
		return nil, err
	}
	// compile regex
	for _, r := range rr {
		if r.Pattern != "" && !strings.EqualFold(r.Matcher, "literal") {
			r.re = regexp.MustCompile(r.Pattern)
		}
	}
	return rr, nil
}

// SaveRules writes the provided rules to a YAML file.
func SaveRules(path string, rr []*Rule) error {
	data, err := yaml.Marshal(rr)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// EnsureSeededRules copies the default rules file to the editable rules file when needed.
func EnsureSeededRules(currentPath, defaultPath string) error {
	if _, err := os.Stat(currentPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	loaded, err := LoadRules(defaultPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepathDir(currentPath), 0o755); err != nil {
		return err
	}
	return SaveRules(currentPath, loaded)
}

// filepathDir returns the directory portion of a file path, or "." when empty.
func filepathDir(path string) string {
	if idx := strings.LastIndexAny(path, "/\\"); idx >= 0 {
		dir := path[:idx]
		if dir != "" {
			return dir
		}
	}
	return "."
}
