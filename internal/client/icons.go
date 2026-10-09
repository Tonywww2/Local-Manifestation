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

// These visual defaults do not assign gameplay behavior to content IDs.
// Resource packs can replace the normal icon:<name> assets independently.
func skillIcon(id string) string {
	switch id {
	case "base:barrier-skill":
		return "shield"
	case "base:haste-skill":
		return "haste"
	case "base:burst-skill":
		return "burst"
	case "base:mend-skill", "base:renew-skill":
		return "heart"
	case "base:root-skill":
		return "root"
	default:
		return "status"
	}
}
