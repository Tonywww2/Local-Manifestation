package sdk

import (
	"fmt"
	"math"
	"reflect"

	"hsmultiplayer/pkg/model"
)

// Node and event callbacks return values; the owner validates and commits a batch.
type Command struct {
	Group      string           `json:"group,omitempty"`
	Item       string           `json:"item,omitempty"`
	Text       string           `json:"text,omitempty"`
	DamageType string           `json:"damage_type,omitempty"`
	Status     string           `json:"status,omitempty"`
	Source     *model.SourceRef `json:"source,omitempty"`
	Kind       string           `json:"kind"`
	Target     string           `json:"target"`
	Amount     float64          `json:"amount"`
}
type Parameter struct {
	Type    string  `json:"type"`
	Minimum float64 `json:"minimum"`
	Maximum float64 `json:"maximum"`
}
type NodeContext struct {
	Player     model.Player   `json:"player"`
	Node       model.Node     `json:"node"`
	Floor      int            `json:"floor"`
	Command    string         `json:"command"`
	FirstVisit bool           `json:"first_visit"`
	Parameters map[string]any `json:"parameters"`
}
type EventContext struct {
	Phase   string         `json:"phase,omitempty"`
	Enemies []model.Enemy  `json:"enemies,omitempty"`
	Time    float64        `json:"time"`
	Scope   string         `json:"scope"`
	Scene   string         `json:"scene"`
	Floor   int            `json:"floor"`
	Event   model.Event    `json:"event"`
	Players []model.Player `json:"players"`
}
type NodeHandler interface {
	Execute(NodeContext) ([]Command, error)
}
type EventHandler interface {
	Handle(EventContext) ([]Command, error)
}

func ValidateSchema(schema map[string]Parameter) error {
	for key, p := range schema {
		if key == "" || (p.Type != "number" && p.Type != "integer" && p.Type != "boolean" && p.Type != "string") {
			return fmt.Errorf("invalid parameter %s", key)
		}
		if !finite(p.Minimum) || !finite(p.Maximum) || p.Minimum > p.Maximum {
			return fmt.Errorf("invalid bounds for %s", key)
		}
	}
	return nil
}
func ValidateParameters(schema map[string]Parameter, params map[string]any) error {
	for key := range params {
		if _, ok := schema[key]; !ok {
			return fmt.Errorf("unknown parameter %s", key)
		}
	}
	for key, p := range schema {
		v, ok := params[key]
		if !ok {
			return fmt.Errorf("missing parameter %s", key)
		}
		switch p.Type {
		case "string":
			if _, ok := v.(string); !ok {
				return fmt.Errorf("parameter %s must be string", key)
			}
		case "boolean":
			if _, ok := v.(bool); !ok {
				return fmt.Errorf("parameter %s must be boolean", key)
			}
		default:
			rv := reflect.ValueOf(v)
			var n float64
			switch rv.Kind() {
			case reflect.Float32, reflect.Float64:
				n = rv.Float()
			case reflect.Int, reflect.Int32, reflect.Int64:
				n = float64(rv.Int())
			default:
				return fmt.Errorf("parameter %s must be numeric", key)
			}
			if !finite(n) || n < p.Minimum || n > p.Maximum || p.Type == "integer" && n != math.Trunc(n) {
				return fmt.Errorf("parameter %s outside bounds", key)
			}
		}
	}
	return nil
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func ValidateCommands(commands []Command, allowed map[string]bool, kinds map[string]bool) error {
	if len(commands) > 128 {
		return fmt.Errorf("command budget exceeded")
	}
	for _, c := range commands {
		if c.Kind != "hit_multiplier" && c.Group != "" {
			return fmt.Errorf("unexpected multiplier group")
		}
		if c.Kind == "notify" {
			if !kinds[c.Kind] || c.Target != "" || c.Text == "" || len(c.Text) > 1024 || c.Amount != 0 || c.Source != nil || c.Status != "" || c.DamageType != "" || c.Item != "" {
				return fmt.Errorf("invalid client notification")
			}
			continue
		}
		if c.Text != "" {
			return fmt.Errorf("unexpected command text")
		}
		if c.Kind == "grant_item" {
			if c.Item == "" || len(c.Item) > 128 || c.Amount < 1 || c.Amount > 64 {
				return fmt.Errorf("invalid item grant")
			}
		} else if c.Item != "" {
			return fmt.Errorf("unexpected item data")
		}
		if !allowed[c.Target] {
			return fmt.Errorf("command target %s outside context", c.Target)
		}
		if !kinds[c.Kind] || !finite(c.Amount) || c.Amount < 0 || c.Amount > 1e6 {
			return fmt.Errorf("invalid command %s", c.Kind)
		}
		if c.Kind == "apply_status" {
			if c.Status == "" || len(c.Status) > 128 || c.Source == nil || c.Source.Actor == "" || c.Source.Instance == "" || c.Amount != 0 || c.DamageType != "" {
				return fmt.Errorf("invalid status command")
			}
		} else if c.Kind == "hit_multiplier" {
			if c.Source == nil || c.Source.Actor == "" || c.Source.Definition == "" || c.Group == "" || len(c.Group) > 128 || c.Status != "" {
				return fmt.Errorf("invalid hit multiplier")
			}
		} else if c.Kind == "add_hit_damage" || c.Kind == "append_hit_damage" {
			if c.Source == nil || c.Source.Actor == "" || c.Source.Definition == "" || c.DamageType == "" {
				return fmt.Errorf("invalid hit contribution")
			}
		} else if c.Kind == "lifesteal" {
			if c.Amount > 1 || c.Status != "" || c.DamageType != "" || c.Source != nil && (c.Source.Actor == "" || c.Source.Definition == "") {
				return fmt.Errorf("invalid lifesteal command")
			}
		} else if c.Status != "" || c.Source != nil || c.DamageType != "" {
			return fmt.Errorf("unexpected status command data")
		}
		if c.Kind != "hit_multiplier" && c.Kind != "heal" && c.Kind != "restore_health" && c.Kind != "weapon_bonus" && c.Kind != "lifesteal" && c.Kind != "add_hit_damage" && c.Kind != "append_hit_damage" && c.Amount != math.Trunc(c.Amount) {
			return fmt.Errorf("command %s requires integer amount", c.Kind)
		}
	}
	return nil
}

func CloneCommands(commands []Command) []Command {
	commands = append([]Command(nil), commands...)
	for i := range commands {
		if commands[i].Source != nil {
			copy := *commands[i].Source
			commands[i].Source = &copy
		}
	}
	return commands
}
