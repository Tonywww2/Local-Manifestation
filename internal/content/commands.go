package content

import (
	"errors"
	"fmt"
	"hsmultiplayer/pkg/status"
	"math"

	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
	"hsmultiplayer/sdk"
)

var NodeCommands = map[string]bool{"grant_item": true, "apply_status": true, "spend_coins": true, "upgrade_weapon": true, "weapon_bonus": true, "remove_penalty": true, "heal_full": true, "grant_xp": true, "grant_coins": true, "heal": true, "restore_health": true}
var EventCommands = map[string]bool{"apply_status": true, "grant_xp": true, "grant_coins": true, "heal": true, "restore_health": true}

// ApplyCommands validates all targets and applies to copies before publishing any result.
func ApplyCommands(c *Catalog, players map[string]model.Player, commands []sdk.Command, kinds map[string]bool) (map[string]model.Player, error) {
	allowed := map[string]bool{}
	next := map[string]model.Player{}
	for id, p := range players {
		allowed[id] = true
		next[id] = model.ClonePlayer(p)
	}
	if err := sdk.ValidateCommands(commands, allowed, kinds); err != nil {
		return nil, err
	}
	for _, cmd := range commands {
		p := next[cmd.Target]
		switch cmd.Kind {
		case "grant_item":
			for i := 0; i < int(cmd.Amount); i++ {
				var err error
				p, err = c.GrantItem(p, cmd.Item)
				if err != nil {
					return nil, err
				}
			}
		case "apply_status":
			var err error
			attributes := model.Attributes{}
			if actor, ok := next[cmd.Source.Actor]; ok {
				attributes = actor.Attributes
			}
			p, err = ApplyStatusSnapshot(c, p, cmd.Status, *cmd.Source, attributes)
			if err != nil {
				if errors.Is(err, status.ErrUnique) {
					continue
				}
				return nil, err
			}
		case "spend_coins":
			if p.Coins < int(cmd.Amount) {
				return nil, protocol.Reject("error.coins", "cost", fmt.Sprint(int(cmd.Amount)))
			}
			p.Coins -= int(cmd.Amount)
		case "upgrade_weapon":
			p.Upgrade += int(cmd.Amount)
		case "weapon_bonus":
			p.WeaponBonus += cmd.Amount
		case "remove_penalty":
			var err error
			p, err = ReducePenalty(p, int(cmd.Amount))
			if err != nil {
				return nil, err
			}
		case "heal_full":
			if p.Life == "alive" {
				p.HP = p.MaxHP
			}
		case "heal", "restore_health":
			if p.Life == "alive" {
				p.HP = damage.Round(math.Min(p.MaxHP, p.HP+c.HealingAmount(p, cmd.Amount, cmd.Kind == "restore_health")))
			}
		case "grant_xp":
			if p.Life != "dead" {
				p.XP += int(cmd.Amount)
			}
		case "grant_coins":
			if p.Life != "dead" {
				p.Coins += int(cmd.Amount)
			}
		}
		if p.Coins > 1e9 || p.XP > 1e9 || p.Upgrade > 100000 || p.WeaponBonus > 1e9 {
			return nil, fmt.Errorf("player command result exceeds limits")
		}
		next[cmd.Target] = p
	}
	for id, p := range next {
		if p.Upgrade != players[id].Upgrade || p.WeaponBonus != players[id].WeaponBonus {
			var err error
			p, err = c.StoreWeaponUpgrade(p)
			if err != nil {
				return nil, err
			}
			next[id] = p
		}
	}
	return next, nil
}
