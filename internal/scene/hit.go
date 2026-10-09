package scene

import (
	"errors"
	"fmt"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/life"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/status"
	"hsmultiplayer/sdk"
)

func (w *world) dispatchHit(e model.Event, players map[string]model.Player, target *model.Enemy) ([]sdk.Command, error) {
	if w.runtime == nil {
		return nil, nil
	} // isolated personal-hall host has no hit targets
	e.Scene = w.id
	var values []model.Player
	for _, id := range w.playerIDs() {
		values = append(values, players[id])
	}
	enemies := w.enemies
	if target != nil {
		enemies = append([]model.Enemy(nil), enemies...)
		for i := range enemies {
			if enemies[i].ID == target.ID {
				enemies[i] = *target
			}
		}
	}
	return w.runtime.Dispatch(sdk.EventContext{Scope: "scene", Scene: w.id, Floor: w.floor, Time: w.clock, Event: e, Players: values, Enemies: enemies})
}

// One hit validates callbacks, typed contributions, defense, shields and
// after-damage effects before committing player/enemy replacements together.
func (w *world) resolveHit(q damage.Request, e model.Event, enemy *model.Enemy) (damage.Result, error) {
	e.Critical = q.Critical
	targetEnemy := enemy
	if enemy != nil {
		v := model.CloneEnemy(*enemy)
		enemy = &v
	}
	players := map[string]model.Player{}
	for id, p := range w.players {
		players[id] = p.player
	}
	preTarget := players[e.Target]
	var commands []sdk.Command
	var err error
	if e.Kind == "hit" {
		commands, err = w.dispatchHit(e, players, enemy)
		for _, id := range q.Weapon.OnHitStatuses {
			source := q.Source
			if source.Kind == "" {
				source = model.SourceRef{Kind: "enemy", Actor: e.Source, Definition: e.SourceDefinition, Instance: e.AttackID}
			}
			commands = append(commands, sdk.Command{Kind: "apply_status", Target: e.Target, Status: id, Source: &source})
		}
	}
	if err != nil {
		return damage.Result{}, err
	}
	var effects []sdk.Command
	allowed := map[string]bool{}
	for id := range players {
		allowed[id] = true
	}
	allowed[e.Target] = true
	for i, c := range commands {
		if c.Kind == "add_hit_damage" || c.Kind == "append_hit_damage" || c.Kind == "hit_multiplier" {
			if err = sdk.ValidateCommands([]sdk.Command{c}, allowed, map[string]bool{c.Kind: true}); err != nil {
				return damage.Result{}, err
			}
			if c.Target != e.Target || !allowed[c.Source.Actor] && c.Source.Actor != e.Source || !w.catalog.KnownSource(c.Source.Definition) {
				return damage.Result{}, fmt.Errorf("hit source outside context")
			}
			if c.Kind == "hit_multiplier" {
				q, err = damage.WithMultiplier(q, c.Group, c.DamageType, c.Amount, *c.Source)
			} else {
				q, err = damage.Contribute(q, fmt.Sprintf("hit:%d", i), c.Kind, c.DamageType, c.Amount, *c.Source)
			}
			if err != nil {
				return damage.Result{}, err
			}
		} else {
			effects = append(effects, c)
		}
	}
	if enemy != nil && e.Kind == "hit" {
		q, err = content.PrepareHit(q, *enemy, e.Attack)
		if err != nil {
			return damage.Result{}, err
		}
	}
	if len(effects) > 0 {
		players, enemy, err = w.hitEffects(players, enemy, effects)
		if err != nil {
			return damage.Result{}, err
		}
	}
	hp, shields := 0.0, model.StatusSet{Time: w.adventureTime}
	if enemy != nil {
		hp, shields = enemy.HP, enemy.Statuses
		q.TypedDefense = w.catalog.EnemyDefense(*enemy, w.floor)
	} else {
		p, ok := players[e.Target]
		if !ok {
			return damage.Result{}, fmt.Errorf("missing hit target")
		}
		hp, shields = p.HP, p.Statuses
		q.Defense.Armor = p.Armor
		q.TypedDefense = w.catalog.PlayerDefenseAtHealth(p, preTarget.HP, preTarget.MaxHP)
	}
	r, err := damage.Evaluate(q)
	if err != nil {
		return damage.Result{}, err
	}
	r, hp, shields, err = damage.Settle(r, hp, shields)
	if err != nil {
		return damage.Result{}, err
	}
	if enemy == nil {
		p := players[e.Target]
		p.HP, p.Statuses = hp, shields
		if hp <= 0 {
			p, err = life.Incapacitate(p)
			if err != nil {
				return damage.Result{}, err
			}
		}
		players[e.Target] = p
	}
	effects = nil
	if enemy != nil {
		enemy.HP, enemy.Statuses = hp, shields
	}
	for _, segment := range r.Segments {
		fact := e
		fact.Kind = "after_damage"
		fact.SegmentID = e.HitID + ":" + segment.ID
		fact.DamageType = segment.Damage.Type
		fact.Amount = segment.HealthLoss
		fact.Absorbed = segment.Absorbed
		fact.Overkill = segment.Overkill
		commands, err = w.dispatchHit(fact, players, enemy)
		if err != nil {
			return damage.Result{}, err
		}
		for _, c := range commands {
			if c.Kind == "lifesteal" {
				if err = sdk.ValidateCommands([]sdk.Command{c}, map[string]bool{fact.Source: true}, map[string]bool{"lifesteal": true}); err != nil {
					return damage.Result{}, err
				}
				if c.Source != nil && (c.Source.Actor != fact.Source || !w.catalog.KnownSource(c.Source.Definition)) {
					return damage.Result{}, fmt.Errorf("invalid lifesteal source")
				}
				if !fact.Attack || c.Target != fact.Source || c.Amount < 0 || c.Amount > 1 {
					return damage.Result{}, fmt.Errorf("invalid lifesteal")
				}
				c.Kind = "restore_health"
				c.Amount = damage.Round(c.Amount * fact.Amount)
				c.Source = nil
			}
			effects = append(effects, c)
		}
	}
	if len(effects) > 0 {
		if enemy != nil {
			enemy.HP, enemy.Statuses = hp, shields
		}
		players, enemy, err = w.hitEffects(players, enemy, effects)
		if enemy != nil {
			hp, shields = enemy.HP, enemy.Statuses
		}
		if err != nil {
			return damage.Result{}, err
		}
	}
	for id, p := range players {
		w.players[id].player = p
		if p.Life != "alive" {
			w.players[id].input = model.Input{}
			w.players[id].attack, w.players[id].dodgeTime, w.players[id].invulnerable = nil, 0, 0
		}
	}
	if enemy != nil {
		enemy.HP, enemy.Statuses = hp, shields
		*targetEnemy = model.CloneEnemy(*enemy)
	}
	return r, nil
}

func (w *world) advanceEnemyStatuses(now int64) error {
	for i := range w.enemies {
		e := &w.enemies[i]
		next, pulses, err := status.Pulses(e.Statuses, now)
		if err != nil {
			return err
		}
		e.Statuses = next
		for _, p := range pulses {
			if p.Kind == "heal" {
				e.HP = min(e.MaxHP, e.HP+p.Amount*float64(p.Count))
				continue
			}
			for j := int64(0); j < p.Count && e.HP > 0; j++ {
				_, err = w.resolveHit(damage.Request{Weapon: model.Weapon{ID: p.Definition, Type: p.Type, Base: p.Amount}, FinalMultiplier: 1}, model.Event{Kind: "periodic", Source: p.Source.Actor, Target: fmt.Sprint(e.ID), HitID: fmt.Sprintf("pulse:%d:%d:%d", e.ID, now, j), SourceDefinition: p.Definition, Tick: w.tick}, e)
				if err != nil {
					return err
				}
			}
		}
		next, _, err = status.Advance(e.Statuses, now)
		if err != nil {
			return err
		}
		w.enemies[i].Statuses = next
	}
	return nil
}

func (w *world) hitEffects(players map[string]model.Player, enemy *model.Enemy, commands []sdk.Command) (map[string]model.Player, *model.Enemy, error) {
	var playerCommands []sdk.Command
	var npc *model.Enemy
	if enemy != nil {
		v := model.CloneEnemy(*enemy)
		npc = &v
	}
	for _, cmd := range commands {
		if npc == nil || cmd.Target != fmt.Sprint(npc.ID) {
			playerCommands = append(playerCommands, cmd)
			continue
		}
		if err := sdk.ValidateCommands([]sdk.Command{cmd}, map[string]bool{cmd.Target: true}, map[string]bool{"apply_status": true, "heal": true}); err != nil {
			return nil, nil, err
		}
		if cmd.Kind == "heal" {
			npc.HP = min(npc.MaxHP, npc.HP+cmd.Amount)
			continue
		}
		d, ok := w.catalog.Status(cmd.Status)
		if !ok || !w.catalog.KnownSource(cmd.Source.Definition) {
			return nil, nil, fmt.Errorf("unknown enemy status source")
		}
		if len(d.Controls) > 0 || hasSlow(d) {
			res := w.catalog.Enemy(npc.Definition).ControlResistance
			if res >= 1 {
				continue
			}
			d.DurationMillis = max(1, int64(float64(d.DurationMillis)*(1-res)))
		}
		attributes := model.Attributes{}
		if actor, ok := players[cmd.Source.Actor]; ok {
			attributes = actor.Attributes
		}
		set, err := status.ApplySnapshot(npc.Statuses, d, *cmd.Source, w.adventureTime, attributes)
		if err != nil {
			if errors.Is(err, status.ErrUnique) {
				continue
			}
			return nil, nil, err
		}
		npc.Statuses = set
	}
	if len(playerCommands) > 0 {
		next, err := content.ApplyCommands(w.catalog, players, playerCommands, content.EventCommands)
		if err != nil {
			return nil, nil, err
		}
		players = next
	}
	return players, npc, nil
}
