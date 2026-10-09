package client

import (
	"hsmultiplayer/pkg/model"
)

type itemView struct {
	Icon, Title, Badge string
	Summary, Details   []string
}
type itemViewKey struct {
	Item                             model.ItemInstance
	Inventory, Statuses              uint64
	Attributes                       model.Attributes
	Base                             model.StatValues
	Contributions                    string
	Downs, Level                     int
	Weapon, Player, Language, Digest string
}

func (g *Game) itemViewKey(it model.ItemInstance) itemViewKey {
	k := itemViewKey{Item: it, Language: g.language, Digest: g.catalog.Digest}
	if p := g.self(); p != nil {
		k.Inventory = p.Inventory.Revision
		k.Statuses = p.Statuses.Revision
		k.Attributes = p.Attributes
		k.Base = p.BaseStats
		k.Contributions = g.u.previewContributions
		k.Downs = p.Downs
		k.Level = p.Level
		k.Weapon = p.Weapon
		k.Player = p.ID
	}
	return k
}
