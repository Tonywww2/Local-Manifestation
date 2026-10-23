package client

import "hsmultiplayer/pkg/model"

func weaponIcon(w model.Weapon) string {
	if w.Type == "magic" {
		return "burst"
	}
	if w.Mode != "melee" && w.Type == "physical" {
		return "firearm"
	}
	return "weapon"
}

func presetIcon(id string) string {
	switch id {
	case "base:bulwark":
		return "weapon"
	case "base:scout":
		return "ranged"
	case "base:arcanist", "base:renewal-frame":
		return "burst"
	default:
		return "armor"
	}
}
