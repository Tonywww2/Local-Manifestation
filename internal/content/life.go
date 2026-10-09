package content

import (
	"fmt"
	"hsmultiplayer/pkg/life"
	"hsmultiplayer/pkg/model"
)

func RescuePlayer(p model.Player) (model.Player, error) {
	next, err := life.Raise(p)
	if err != nil {
		return model.Player{}, err
	}
	next, err = RefreshPlayer(next)
	if err != nil {
		return model.Player{}, err
	}
	next.HP = next.MaxHP
	return next, nil
}

func RevivePlayer(p model.Player) (model.Player, error) {
	n, err := life.Revive(p)
	if err != nil {
		return model.Player{}, err
	}
	n, err = RefreshPlayer(n)
	if err != nil {
		return model.Player{}, err
	}
	n.HP = n.MaxHP
	return n, nil
}

func ReducePenalty(p model.Player, layers int) (model.Player, error) {
	if layers < 0 || layers > 3 {
		return model.Player{}, fmt.Errorf("invalid penalty removal")
	}
	next := model.ClonePlayer(p)
	next.Downs = max(0, next.Downs-layers)
	return RefreshPlayer(next)
}

func PrepareLayerPlayer(p model.Player) (model.Player, error) {
	if p.Life == "dead" {
		return model.ClonePlayer(p), nil
	}
	next := model.ClonePlayer(p)
	var err error
	if next.Life == "down" {
		next, err = RescuePlayer(next)
		if err != nil {
			return model.Player{}, err
		}
	}
	next, err = ReducePenalty(next, 1)
	if err != nil {
		return model.Player{}, err
	}
	next.HP = next.MaxHP
	return next, nil
}
