package rules

import (
	"io"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Rule defines a redaction rule loaded from YAML.
type Rule struct {
	Name        string `yaml:"name" json:"name"`
	Pattern     string `yaml:"pattern" json:"pattern"`
	Replacement string `yaml:"replacement" json:"replacement"`
	Enabled     bool   `yaml:"enabled" json:"enabled"`

	re *regexp.Regexp
}

// Apply redacts input with the compiled rule if enabled.
func (r *Rule) Apply(input string) string {
	if r == nil || !r.Enabled || r.re == nil {
		if r == nil || !r.Enabled || r.Pattern == "" {
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
		if r.Pattern != "" {
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
