// Package localization loads standard JSON language packs and formats named parameters.
// Catalogs are immutable after loading and safe to share between readers.
package localization

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

type Pack struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Messages map[string]string `json:"messages"`
}

type Catalog struct{ packs map[string]Pack }

var idPattern = regexp.MustCompile(`^[A-Za-z0-9-]{1,35}$`)
var parameterPattern = regexp.MustCompile(`\{([a-z][a-z0-9_]*)\}`)

// Load merges successive directories. Later packs override messages by language ID.
// Underscore comment fields remain ordinary JSON and are ignored by the decoder.
func Load(sources ...fs.FS) (*Catalog, error) {
	c := &Catalog{packs: map[string]Pack{}}
	for _, source := range sources {
		entries, err := fs.ReadDir(source, ".")
		if err != nil {
			return nil, err
		}
		seen := map[string]bool{}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			b, err := fs.ReadFile(source, e.Name())
			if err != nil {
				return nil, err
			}
			if !utf8.Valid(b) {
				return nil, fmt.Errorf("locale %s is not UTF-8", e.Name())
			}
			var p Pack
			if err := json.Unmarshal(b, &p); err != nil {
				return nil, fmt.Errorf("locale %s: %w", e.Name(), err)
			}
			if !idPattern.MatchString(p.ID) || p.Name == "" || len(p.Messages) == 0 {
				return nil, fmt.Errorf("invalid locale %s", e.Name())
			}
			if seen[p.ID] {
				return nil, fmt.Errorf("duplicate language %s", p.ID)
			}
			seen[p.ID] = true
			old := c.packs[p.ID]
			if old.Messages == nil {
				old.Messages = map[string]string{}
			}
			for k, v := range p.Messages {
				if k == "" || v == "" {
					return nil, fmt.Errorf("empty translation in %s", e.Name())
				}
				if previous, ok := old.Messages[k]; ok && strings.Join(Parameters(v), ",") != strings.Join(Parameters(previous), ",") {
					return nil, fmt.Errorf("locale %s: override parameters differ for %s", p.ID, k)
				}
				old.Messages[k] = v
			}
			old.ID, old.Name = p.ID, p.Name
			c.packs[p.ID] = old
		}
	}
	english, ok := c.packs["en"]
	if !ok {
		return nil, fmt.Errorf("English fallback pack required")
	}
	for _, p := range c.packs {
		for key, value := range p.Messages {
			if fallback, ok := english.Messages[key]; ok && strings.Join(Parameters(value), ",") != strings.Join(Parameters(fallback), ",") {
				return nil, fmt.Errorf("locale %s: parameters differ for %s", p.ID, key)
			}
		}
	}
	return c, nil
}

func Parameters(value string) []string {
	seen := map[string]bool{}
	for _, m := range parameterPattern.FindAllStringSubmatch(value, -1) {
		seen[m[1]] = true
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (c *Catalog) HasLanguage(id string) bool { _, ok := c.packs[id]; return ok }
func (c *Catalog) Name(id string) string      { return c.packs[id].Name }
func (c *Catalog) Languages() []string {
	ids := make([]string, 0, len(c.packs))
	for id := range c.packs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (c *Catalog) Lookup(language, key string) (string, bool) {
	if value, ok := c.packs[language].Messages[key]; ok {
		return value, true
	}
	value, ok := c.packs["en"].Messages[key]
	return value, ok
}

func (c *Catalog) Text(language, key string, params map[string]string) string {
	value, ok := c.Lookup(language, key)
	if !ok {
		return key
	}
	// Replace in one pass; parameter values cannot inject additional placeholders.
	return parameterPattern.ReplaceAllStringFunc(value, func(token string) string {
		if v, ok := params[token[1:len(token)-1]]; ok {
			return v
		}
		return token
	})
}
