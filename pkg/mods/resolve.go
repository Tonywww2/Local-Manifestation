// Package mods resolves immutable package identities before any installation or load.
package mods

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type Package struct {
	ID           string            `json:"id"`
	Version      string            `json:"version"`
	SHA256       string            `json:"sha256"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
}
type Lock struct {
	Version  int       `json:"version"`
	Packages []Package `json:"packages"`
}
type version struct {
	n   [3]int
	pre string
}

func parse(v string) (version, error) {
	var result version
	v = strings.TrimPrefix(v, "v")
	v = strings.SplitN(v, "+", 2)[0]
	p := strings.SplitN(v, "-", 2)
	if len(p) > 1 {
		result.pre = p[1]
	}
	parts := strings.Split(p[0], ".")
	if len(parts) != 3 {
		return result, fmt.Errorf("expected semantic version")
	}
	for i, v := range parts {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || len(v) > 1 && v[0] == '0' {
			return result, fmt.Errorf("invalid semantic version")
		}
		result.n[i] = n
	}
	return result, nil
}
func compare(a, b version) int {
	for i := range a.n {
		if a.n[i] < b.n[i] {
			return -1
		}
		if a.n[i] > b.n[i] {
			return 1
		}
	}
	if a.pre == b.pre {
		return 0
	}
	if a.pre == "" {
		return 1
	}
	if b.pre == "" {
		return -1
	}
	ap, bp := strings.Split(a.pre, "."), strings.Split(b.pre, ".")
	for i := 0; i < min(len(ap), len(bp)); i++ {
		if ap[i] == bp[i] {
			continue
		}
		an, ae := strconv.Atoi(ap[i])
		bn, be := strconv.Atoi(bp[i])
		if ae == nil && be == nil {
			if an < bn {
				return -1
			}
			return 1
		}
		if ae == nil {
			return -1
		}
		if be == nil {
			return 1
		}
		return strings.Compare(ap[i], bp[i])
	}
	if len(ap) < len(bp) {
		return -1
	}
	return 1
}
func Satisfies(v, requirement string) bool {
	value, err := parse(v)
	if err != nil {
		return false
	}
	if requirement == "*" {
		return value.pre == ""
	}
	for _, alternative := range strings.Split(requirement, "||") {
		ok := true
		terms := strings.Fields(strings.ReplaceAll(alternative, ",", " "))
		if len(terms) == 0 {
			continue
		}
		for _, term := range terms {
			op := "="
			for _, prefix := range []string{">=", "<=", ">", "<", "^", "~", "="} {
				if strings.HasPrefix(term, prefix) {
					op = prefix
					term = strings.TrimPrefix(term, prefix)
					break
				}
			}
			target, err := parse(term)
			if err != nil {
				ok = false
				break
			}
			cmp := compare(value, target)
			switch op {
			case "=":
				ok = cmp == 0
			case ">=":
				ok = cmp >= 0
			case "<=":
				ok = cmp <= 0
			case ">":
				ok = cmp > 0
			case "<":
				ok = cmp < 0
			case "~":
				upper := target
				upper.n[1]++
				upper.n[2] = 0
				ok = cmp >= 0 && compare(value, upper) < 0
			case "^":
				upper := target
				if upper.n[0] > 0 {
					upper.n[0]++
					upper.n[1], upper.n[2] = 0, 0
				} else if upper.n[1] > 0 {
					upper.n[1]++
					upper.n[2] = 0
				} else {
					upper.n[2]++
				}
				ok = cmp >= 0 && compare(value, upper) < 0
			}
			if !ok {
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// Resolve prefers a compatible existing lock, then highest installed versions.
// Backtracking also handles conflicting transitive ranges; the final order is
// topological and stable, so client/server produce identical content identities.
func Resolve(available []Package, selected []string, previous Lock) (Lock, error) {
	byID := map[string][]Package{}
	for _, p := range available {
		if _, err := parse(p.Version); err != nil {
			return Lock{}, err
		}
		byID[p.ID] = append(byID[p.ID], p)
	}
	pinned := map[string]Package{}
	for _, p := range previous.Packages {
		pinned[p.ID] = p
	}
	for id := range byID {
		sort.SliceStable(byID[id], func(i, j int) bool {
			a, b := byID[id][i], byID[id][j]
			ap, bp := a.SHA256 == pinned[id].SHA256, b.SHA256 == pinned[id].SHA256
			if ap != bp {
				return ap
			}
			x, _ := parse(a.Version)
			y, _ := parse(b.Version)
			if n := compare(x, y); n != 0 {
				return n > 0
			}
			return a.SHA256 < b.SHA256
		})
	}
	required := map[string][]string{}
	for _, id := range selected {
		required[id] = []string{"*"}
	}
	var search func(map[string]Package, map[string][]string) (map[string]Package, bool)
	search = func(chosen map[string]Package, ranges map[string][]string) (map[string]Package, bool) {
		var ids []string
		for id := range ranges {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			if p, ok := chosen[id]; ok {
				for _, r := range ranges[id] {
					if !Satisfies(p.Version, r) {
						return nil, false
					}
				}
			}
		}
		id := ""
		for _, key := range ids {
			if _, ok := chosen[key]; !ok {
				id = key
				break
			}
		}
		if id == "" {
			return chosen, true
		}
		for _, p := range byID[id] {
			valid := true
			for _, r := range ranges[id] {
				if !Satisfies(p.Version, r) {
					valid = false
				}
			}
			if !valid {
				continue
			}
			next := map[string]Package{}
			for k, v := range chosen {
				next[k] = v
			}
			next[id] = p
			needs := map[string][]string{}
			for k, v := range ranges {
				needs[k] = append([]string(nil), v...)
			}
			for dependency, r := range p.Dependencies {
				needs[dependency] = append(needs[dependency], r)
			}
			if solved, ok := search(next, needs); ok {
				return solved, true
			}
		}
		return nil, false
	}
	chosen, ok := search(map[string]Package{}, required)
	if !ok {
		return Lock{}, fmt.Errorf("no installed combination satisfies dependency ranges")
	}
	var out []Package
	visiting, done := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		if done[id] {
			return nil
		}
		if visiting[id] {
			return fmt.Errorf("dependency cycle")
		}
		visiting[id] = true
		var deps []string
		for dep := range chosen[id].Dependencies {
			deps = append(deps, dep)
		}
		sort.Strings(deps)
		for _, dep := range deps {
			if err := visit(dep); err != nil {
				return err
			}
		}
		visiting[id] = false
		done[id] = true
		out = append(out, chosen[id])
		return nil
	}
	for _, id := range selected {
		if err := visit(id); err != nil {
			return Lock{}, err
		}
	}
	return Lock{Version: 1, Packages: out}, nil
}
