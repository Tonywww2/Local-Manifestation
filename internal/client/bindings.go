package client

import (
	"fmt"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type bindings map[string]map[string]string

var bindingGroups = [][]string{{"up", "down", "left", "right", "move", "attack", "dodge"}, {"weapon_skill"}, {"item1", "item2", "item3", "item4"}, {"map", "view_left", "view_right", "detail", "settings", "interact"}}

func defaultBindings() bindings {
	keyboard := map[string]string{"up": "W", "down": "S", "left": "A", "right": "D", "attack": "Mouse0", "dodge": "Space", "weapon_skill": "Mouse2", "item1": "Digit1", "item2": "Digit2", "item3": "Digit3", "item4": "Digit4", "map": "M", "view_left": "Q", "view_right": "E", "detail": "Shift", "settings": "Escape", "interact": "F"}
	mouse := map[string]string{}
	for k, v := range keyboard {
		mouse[k] = v
	}
	for _, k := range []string{"up", "down", "left", "right"} {
		delete(mouse, k)
	}
	mouse["move"] = "Mouse2"
	mouse["weapon_skill"] = "Mouse1"
	return bindings{"keyboard": keyboard, "mouse": mouse}
}

func migrateBindings(saved bindings) bindings {
	out := cloneBindings(saved)
	defaults := defaultBindings()
	for _, mode := range []string{"keyboard", "mouse"} {
		items := out[mode]
		if items == nil {
			out[mode] = cloneBindings(defaults)[mode]
			continue
		}
		for _, key := range []string{"skill1", "skill2", "skill3", "skill4"} {
			delete(items, key)
		}
		if oldView, legacy := items["view"]; legacy {
			delete(items, "view")
			if items["interact"] == "G" {
				delete(items, "interact")
			}
			if oldView != "" && oldView != "Tab" && items["view_right"] == "" {
				items["view_right"] = oldView
			}
		}
		for _, action := range []string{"weapon_skill", "interact", "view_left", "view_right"} {
			assignAvailableBinding(items, action, defaults[mode][action])
		}
	}
	return out
}

// Existing custom bindings take precedence over newly introduced defaults.
func assignAvailableBinding(items map[string]string, action string, preferred ...string) {
	if items[action] != "" {
		return
	}
	candidates := append(preferred, "T", "G", "R", "H", "B", "N", "J", "K", "L", "U", "I", "O", "P", "Y", "Z", "X", "C", "V", "Q", "E", "F", "Tab")
	for _, key := range candidates {
		used := false
		for _, code := range items {
			used = used || code == key
		}
		if !used {
			items[action] = key
			return
		}
	}
}

func cloneBindings(b bindings) bindings {
	out := bindings{}
	for mode, items := range b {
		out[mode] = map[string]string{}
		for k, v := range items {
			out[mode][k] = v
		}
	}
	return out
}

var physicalKeys = func() map[string]ebiten.Key {
	out := map[string]ebiten.Key{}
	for k := ebiten.Key(0); k <= ebiten.KeyMax; k++ {
		if s := k.String(); !strings.HasPrefix(s, "Key(") {
			out[s] = k
		}
	}
	return out
}()

func validBinding(code string) bool {
	if code == "Mouse0" || code == "Mouse1" || code == "Mouse2" {
		return true
	}
	_, ok := physicalKeys[code]
	return ok
}

// Capturing a UI mouse event must also suppress actions rebound to that button.
func bindingAvailable(code string, mouseCaptured bool) bool {
	return !mouseCaptured || !strings.HasPrefix(code, "Mouse")
}
func reservedBinding(code string) bool {
	return code == "F2" || code == "F3" || code == "F4" || code == "F5" || code == "F6" || code == "F7"
}
func validateBindings(b bindings) error {
	defaults := defaultBindings()
	if len(b) != 2 {
		return fmt.Errorf("invalid input modes")
	}
	for mode, items := range defaults {
		if len(b[mode]) != len(items) {
			return fmt.Errorf("invalid bindings for %s", mode)
		}
		seen := map[string]bool{}
		for id := range items {
			code := b[mode][id]
			if !validBinding(code) || reservedBinding(code) || seen[code] {
				return fmt.Errorf("invalid/conflicting binding %s", id)
			}
			seen[code] = true
		}
	}
	return nil
}

// Rebind validates a complete detached candidate, leaving the draft unchanged on error.
func rebind(b bindings, mode, id, code string) (bindings, string, error) {
	if _, ok := b[mode][id]; !ok || !validBinding(code) || reservedBinding(code) {
		return nil, "", fmt.Errorf("unsupported/reserved binding")
	}
	for other, used := range b[mode] {
		if other != id && used == code {
			return nil, other, fmt.Errorf("binding conflict")
		}
	}
	out := cloneBindings(b)
	out[mode][id] = code
	return out, "", validateBindings(out)
}
func bindingDown(code string) bool {
	switch code {
	case "Mouse0":
		return ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	case "Mouse1":
		return ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle)
	case "Mouse2":
		return ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight)
	}
	k, ok := physicalKeys[code]
	return ok && ebiten.IsKeyPressed(k)
}
func bindingPressed(code string) bool {
	switch code {
	case "Mouse0":
		return inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	case "Mouse1":
		return inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle)
	case "Mouse2":
		return inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
	}
	k, ok := physicalKeys[code]
	return ok && inpututil.IsKeyJustPressed(k)
}
func (g *Game) inputMode() string {
	if g.mouseMode {
		return "mouse"
	}
	return "keyboard"
}
func (g *Game) bound(id string) string { return g.u.bindings[g.inputMode()][id] }
func (g *Game) bindingLabel(code string) string {
	switch code {
	case "Mouse0":
		return g.tr("ui.key.left_mouse")
	case "Mouse1":
		return g.tr("ui.key.middle_mouse")
	case "Mouse2":
		return g.tr("ui.key.right_mouse")
	case "Space":
		return g.tr("ui.key.space")
	case "Escape":
		return "Esc"
	}
	return strings.TrimPrefix(code, "Digit")
}
