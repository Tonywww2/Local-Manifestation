package protocol

import (
	"hsmultiplayer/pkg/damage"
	"hsmultiplayer/pkg/model"
	"slices"
)

func CloneScene(s Scene) Scene {
	s.Obstacles = slices.Clone(s.Obstacles)
	s.Players = slices.Clone(s.Players)
	for i := range s.Players {
		s.Players[i] = model.ClonePlayer(s.Players[i])
	}
	s.Enemies = slices.Clone(s.Enemies)
	for i := range s.Enemies {
		s.Enemies[i] = model.CloneEnemy(s.Enemies[i])
	}
	s.Projectiles = slices.Clone(s.Projectiles)
	if s.LastDamage != nil {
		r := damage.CloneResult(*s.LastDamage)
		s.LastDamage = &r
	}
	return s
}

func CloneState(s State) State {
	s.LayerMembers = slices.Clone(s.LayerMembers)
	s.Trades = slices.Clone(s.Trades)
	for i := range s.Trades {
		for j := 0; j < 2; j++ {
			s.Trades[i].Offers[j] = slices.Clone(s.Trades[i].Offers[j])
		}
	}
	s.Players = slices.Clone(s.Players)
	for i := range s.Players {
		s.Players[i] = model.ClonePlayer(s.Players[i])
	}
	s.Nodes = slices.Clone(s.Nodes)
	s.Terrain = slices.Clone(s.Terrain)
	for i := range s.Nodes {
		s.Nodes[i].Edges = slices.Clone(s.Nodes[i].Edges)
	}
	s.Scenes = slices.Clone(s.Scenes)
	for i := range s.Scenes {
		s.Scenes[i] = CloneScene(s.Scenes[i])
	}
	if s.Vote != nil {
		v := *s.Vote
		v.Eligible = slices.Clone(v.Eligible)
		v.Yes = slices.Clone(v.Yes)
		v.No = slices.Clone(v.No)
		s.Vote = &v
	}
	return s
}
