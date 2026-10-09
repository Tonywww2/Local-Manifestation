package nativedemo

import (
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/sdk"
)

func Shared(reg sdk.Registrar) error {
	_,err:=reg.RegisterEvent("native-demo:hit", "scene", "hit", hitBonus{});return err
}
type hitBonus struct{}
func (hitBonus) Handle(ctx sdk.EventContext)([]sdk.Command,error) {
	if !ctx.Event.Attack{return nil,nil};for _,p:=range ctx.Players{if p.ID==ctx.Event.Source{return []sdk.Command{{Kind:"hit_multiplier",Target:ctx.Event.Target,Group:"native-demo",Amount:1.1,Source:&model.SourceRef{Kind:"weapon",Actor:p.ID,Definition:ctx.Event.SourceDefinition,Instance:p.Inventory.Equipment[0]}}},nil}};return nil,nil
}
// Every Runtime gets its own observer. No game state or VM is globally mutable.
func Client(reg sdk.Registrar) error {
	_,err:=reg.RegisterEvent("native-demo:client", "client", "snapshot", &observer{});return err
}
type observer struct{phase string}
func(o *observer)Handle(ctx sdk.EventContext)([]sdk.Command,error){
	if o.phase==ctx.Phase{return nil,nil};o.phase=ctx.Phase
	return []sdk.Command{{Kind:"notify",Text:"Native demo: "+ctx.Phase}},nil
}
