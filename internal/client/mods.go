package client

import (
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/mods"
	"os"
	"path/filepath"
	"strings"
)

type modResult struct {
	catalog   *content.Catalog
	installed []content.InstalledPackage
	lockPath  string
	err       error
	kind      string
}
type modManager struct {
	path, hash, selection, editing, combination string
	installed                                   []content.InstalledPackage
	pending                                     chan modResult
}

func (g *Game) initializeMods() {
	if g.cfg.ModRoot == "" {
		g.cfg.ModRoot = "data/mods"
	}
	g.mods.installed, _ = content.Installed(g.cfg.ModRoot)
	if g.cfg.Combination != "" {
		g.front.Config.Combination = g.cfg.Combination
	}
	if path := g.front.Config.Combination; path != "" && g.cfg.ReloadContent != nil {
		catalog, err := g.cfg.ReloadContent(path, nil)
		if err != nil {
			g.notifyError(err)
			g.front.Config.Combination = ""
		} else {
			if err := g.publishExtensions(catalog); err != nil {
				g.notifyError(err)
				g.front.Config.Combination = ""
			} else {
				g.catalog = catalog
				g.cfg.Combination = path
			}
		}
	}
	if g.extensions == nil {
		if err := g.publishExtensions(g.catalog); err != nil {
			g.notifyError(err)
		}
	}
	g.mods.combination = g.front.Config.Combination
	if g.mods.combination == "" {
		g.mods.combination = filepath.Join(filepath.Dir(g.cfg.Profile), "combinations", "default.json")
	}
}
func (g *Game) buildMods(f *uiFrame, layout ui.Layout) {
	r := g.windowPanel(f, layout, "mods", g.tr("ui.mods"), ui.Size{W: 610, H: 590})
	g.addLabel(f, "mods", ui.Rect{X: r.X + 14, Y: r.Y + 36, W: r.W - 28, H: 44}, g.tr("ui.mods.help"))
	for i, v := range []struct{ id, value string }{{"path", g.mods.path}, {"hash", g.mods.hash}, {"selection", g.mods.selection}, {"combination", g.mods.combination}} {
		y := r.Y + 86 + float64(i)*51
		g.addLabel(f, "mods", ui.Rect{X: r.X + 14, Y: y, W: 110, H: 30}, g.tr("ui.mods."+v.id))
		addControl(f, "mods", "mod-field:"+v.id, "key", "", "mod-field", v.id, g.tr("ui.mods."+v.id), ui.Rect{X: r.X + 128, Y: y, W: r.W - 142, H: 34})
		f.Controls[len(f.Controls)-1].Caption = v.value
	}
	for i, action := range []string{"install", "resolve", "reload", "download", "build"} {
		addControl(f, "mods", "mod:"+action, "settings", "", "mod:"+action, "", g.tr("ui.mods."+action), ui.Rect{X: r.X + 14 + float64(i)*48, Y: r.Y + 301, W: 38, H: 32})
	}
	if g.mods.pending != nil {
		g.addLabel(f, "mods", ui.Rect{X: r.X + 270, Y: r.Y + 301, W: r.W - 280, H: 32}, g.tr("ui.mods.busy"))
	}
	start := min(g.u.scroll["mods"], max(0, len(g.mods.installed)-7))
	for i, p := range g.mods.installed[start:min(start+7, len(g.mods.installed))] {
		mode := g.tr("ui.mods.reloadable")
		if p.ContainsGo {
			mode = g.tr("ui.mods.restart")
		}
		g.addLabel(f, "mods", ui.Rect{X: r.X + 14, Y: r.Y + 349 + float64(i)*29, W: r.W - 28, H: 29}, fmt.Sprintf("%s@%s · %s · %s", p.ID, p.Version, p.Type, mode))
	}
}
func (g *Game) updateModFields() {
	var value *string
	switch g.mods.editing {
	case "path":
		value = &g.mods.path
	case "hash":
		value = &g.mods.hash
	case "selection":
		value = &g.mods.selection
	case "combination":
		value = &g.mods.combination
	}
	if value == nil {
		return
	}
	for _, r := range ebiten.AppendInputChars(nil) {
		if r >= 32 && len([]rune(*value)) < 1024 {
			*value += string(r)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		r := []rune(*value)
		if len(r) > 0 {
			*value = string(r[:len(r)-1])
		}
	}
}
func (g *Game) modAction(c uiControl) bool {
	if c.Action == "mod-field" {
		g.mods.editing = c.Value
		return true
	}
	if !strings.HasPrefix(c.Action, "mod:") {
		return false
	}
	if g.state != nil || g.frontBusy() {
		g.notify("notice.mods_in_run")
		return true
	}
	kind := strings.TrimPrefix(c.Action, "mod:")
	path, hash, selection, store, address := g.mods.path, g.mods.hash, g.mods.selection, g.cfg.ModRoot, g.address
	lockPath := g.mods.combination
	if lockPath == "" {
		lockPath = filepath.Join(filepath.Dir(g.cfg.Profile), "combinations", "default.json")
	}
	current := g.catalog
	reload, build := g.cfg.ReloadContent, g.cfg.BuildContent
	channel := make(chan modResult, 1)
	g.mods.pending = channel
	go func() {
		result := modResult{kind: kind, lockPath: lockPath}
		switch kind {
		case "install":
			_, result.err = content.InstallPackage(store, path, hash)
		case "resolve":
			installed, err := content.Installed(store)
			result.err = err
			if err == nil {
				var available []mods.Package
				for _, p := range installed {
					available = append(available, p.Package)
				}
				previous, _ := content.LoadLock(lockPath)
				lock, err := mods.Resolve(available, strings.FieldsFunc(selection, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' }), previous)
				result.err = err
				if err == nil {
					result.err = content.SaveLock(lockPath, lock)
				}
			}
		case "download":
			result.err = DownloadPackages(address, store, lockPath)
		case "reload":
			if reload == nil {
				result.err = fmt.Errorf("content reload adapter unavailable")
			} else {
				if _, err := os.Stat(lockPath); os.IsNotExist(err) {
					lockPath = ""
					result.lockPath = ""
				}
				result.catalog, result.err = reload(lockPath, current)
			}
		case "build":
			if build == nil {
				result.err = fmt.Errorf("native build adapter unavailable")
			} else {
				result.err = build(lockPath)
			}
		}
		if result.err == nil {
			result.installed, result.err = content.Installed(store)
		}
		channel <- result
	}()
	return true
}
func (g *Game) pollMods() {
	if g.mods.pending == nil {
		return
	}
	select {
	case result := <-g.mods.pending:
		g.mods.pending = nil
		if result.err != nil {
			g.notifyError(result.err)
			return
		}
		if result.catalog != nil {
			if g.catalog.ContainsGo() || result.catalog.ContainsGo() {
				g.notify("notice.mods_restart")
				return
			}
			if err := g.publishExtensions(result.catalog); err != nil {
				g.notifyError(err)
				return
			}
			g.catalog = result.catalog
			g.cfg.Combination = result.lockPath
			g.chassis = min(g.chassis, len(g.catalog.ChassisList())-1)
			for _, surface := range g.roomSurfaces {
				if surface.Image != nil {
					surface.Image.Deallocate()
				}
			}
			g.roomSurfaces = nil
			g.u.wrapped = nil
			g.u.previewState = nil
			g.u.previewValid = false
		}
		g.mods.installed = result.installed
		if result.kind == "resolve" || result.kind == "download" || result.kind == "reload" {
			g.front.Config.Combination = result.lockPath
			s := g.preferences()
			s.Room = &g.front.Config
			if err := g.commitPreferences(s); err != nil {
				g.notifyError(err)
			}
		}
		g.notify("notice.mods_done", "action", g.tr("ui.mods."+result.kind))
		if result.kind == "build" {
			g.notify("notice.mods_restart")
		}
	default:
	}
}
