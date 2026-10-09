package client

import (
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/script"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/sdk"
)

func (g *Game) publishExtensions(c *content.Catalog) error {
	next, err := script.New(c, 0)
	if err != nil {
		return err
	}
	if err = g.publishResources(c); err != nil {
		next.Close()
		return err
	}
	if g.extensions != nil {
		g.extensions.Close()
	}
	g.extensions = next
	return nil
}
func (g *Game) dispatchClientSnapshot() {
	if g.extensions == nil || g.state == nil || !g.extensions.HasEvent("client", "snapshot") {
		return
	}
	ctx := sdk.EventContext{Scope: "client", Phase: g.state.Phase, Floor: g.state.Floor, Time: float64(g.state.AdventureTime) / 60000, Event: model.Event{Kind: "snapshot", Tick: g.state.ServerTick, Source: g.id}, Players: g.state.Players}
	if scene := g.currentScene(); scene != nil {
		ctx.Scene = scene.ID
		ctx.Enemies = scene.Enemies
	}
	commands, err := g.extensions.Dispatch(ctx)
	if err == nil {
		err = sdk.ValidateCommands(commands, nil, map[string]bool{"notify": true})
	}
	if err != nil {
		g.notifyError(err)
		g.extensions.Close()
		g.extensions = nil
		return
	}
	for _, command := range commands {
		g.notify("notice.extension", "text", command.Text)
	}
}
