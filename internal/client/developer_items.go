package client

import (
	"cmp"
	"slices"
	"strings"

	"hsmultiplayer/internal/client/ui"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/model"
	"hsmultiplayer/pkg/protocol"
)

const developerItemsWindow = "developer-items"

type developerItemEntry struct {
	Definition   model.ItemDef
	Name, Search string
}

type developerItemBrowser struct {
	Query, Category     string
	Editing             bool
	catalog             *content.Catalog
	language, filterKey string
	entries, filtered   []developerItemEntry
}

func filterDeveloperItems(entries []developerItemEntry, category, query string) []developerItemEntry {
	query = strings.ToLower(strings.TrimSpace(query))
	var result []developerItemEntry
	for _, entry := range entries {
		if (category == "" || entry.Definition.Kind == category) && strings.Contains(entry.Search, query) {
			result = append(result, entry)
		}
	}
	return result
}

// Rebuild the detached index only after content or language changes. Filtering
// and pagination do not clone the complete content catalog every frame.
func (g *Game) developerItems() []developerItemEntry {
	b := &g.u.developerItems
	if b.catalog != g.catalog || b.language != g.language {
		b.catalog, b.language = g.catalog, g.language
		b.entries = nil
		for _, d := range g.catalog.ItemList() {
			name := g.contentName(d.ID, d.Name)
			b.entries = append(b.entries, developerItemEntry{Definition: d, Name: name, Search: strings.ToLower(name + "\n" + d.Name + "\n" + d.ID)})
		}
		slices.SortFunc(b.entries, func(a, b developerItemEntry) int {
			if order := cmp.Compare(a.Name, b.Name); order != 0 {
				return order
			}
			return cmp.Compare(a.Definition.ID, b.Definition.ID)
		})
		b.filterKey = ""
	}
	key := b.Category + "\n" + b.Query
	if key != b.filterKey {
		b.filtered = filterDeveloperItems(b.entries, b.Category, b.Query)
		b.filterKey = key
		g.u.scroll[developerItemsWindow] = 0
	}
	return b.filtered
}

func (g *Game) buildDeveloperItems(f *uiFrame, layout ui.Layout) {
	id := developerItemsWindow
	r := g.windowPanel(f, layout, id, g.tr("ui.developer.items"), ui.Size{W: 750, H: 620})
	entries := g.developerItems()
	b := &g.u.developerItems
	query := b.Query
	if b.Editing && g.u.focus == id {
		query += " |"
	} else if query == "" {
		query = g.tr("ui.developer.items.placeholder")
	}
	button := func(key, action, value, caption string, bounds ui.Rect) {
		f.Controls = append(f.Controls, uiControl{ID: key, Owner: id, Action: action, Value: value, Title: caption, Caption: caption, Bounds: bounds})
	}
	button("dev-items:search", "developer-search", "", g.tr("ui.developer.items.search", "query", query), ui.Rect{X: r.X + 14, Y: r.Y + 42, W: r.W - 72, H: 34})
	addControl(f, id, "dev-items:clear", "cancel", "", "developer-clear", "", g.tr("ui.developer.items.clear"), ui.Rect{X: r.X + r.W - 48, Y: r.Y + 42, W: 34, H: 34})
	categories := []string{"", "weapon", "armor", "accessory", "consumable", "movement", "passive"}
	width := (r.W - 28) / float64(len(categories))
	for i, category := range categories {
		label := g.tr("ui.developer.items." + category)
		if category == "" {
			label = g.tr("ui.developer.items.all")
		}
		if category == b.Category {
			label = "[ " + label + " ]"
		}
		button("dev-items:category:"+category, "developer-category", category, label, ui.Rect{X: r.X + 14 + width*float64(i), Y: r.Y + 86, W: width - 4, H: 32})
	}
	admin := g.state != nil && g.state.Admin == g.id
	note := g.tr("ui.developer.items.help")
	if !admin {
		note = g.tr("error.admin_only")
	}
	g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: r.Y + 128, W: r.W - 28, H: 20}, note)
	rows := max(1, int((r.H-212)/54))
	pages := max(1, (len(entries)+rows-1)/rows)
	page := min(max(0, g.u.scroll[id]), pages-1)
	g.u.scroll[id] = page
	start := page * rows
	for i, entry := range entries[start:min(start+rows, len(entries))] {
		d := entry.Definition
		y := r.Y + 158 + float64(i)*54
		preview := model.ItemInstance{Definition: d.ID, Count: 1, Durability: d.Durability}
		f.Controls = append(f.Controls, g.itemControl(id, "dev-items:preview:"+d.ID, preview, ui.Rect{X: r.X + 14, Y: y, W: 44, H: 44}, ""))
		g.addLabel(f, id, ui.Rect{X: r.X + 70, Y: y + 2, W: r.W - 196, H: 20}, entry.Name)
		f.Labels = append(f.Labels, uiLabel{Owner: id, Bounds: ui.Rect{X: r.X + 70, Y: y + 24, W: r.W - 196, H: 18}, Text: d.ID, Size: 11, Color: muted})
		action := ""
		if admin {
			action = "developer-grant"
		}
		button("dev-items:grant:"+d.ID, action, d.ID, g.tr("ui.developer.items.grant"), ui.Rect{X: r.X + r.W - 108, Y: y + 5, W: 94, H: 34})
	}
	if len(entries) == 0 {
		g.addLabel(f, id, ui.Rect{X: r.X + 14, Y: r.Y + 170, W: r.W - 28, H: 28}, g.tr("ui.developer.items.empty"))
	}
	y := r.Y + r.H - 42
	for i, direction := range []string{"-1", "1"} {
		icon := []string{"scroll-left", "scroll-right"}[i]
		addControl(f, id, "dev-items:page:"+direction, icon, "", "inventory-page", id+":"+direction, g.tr("ui.developer.items.page_help"), ui.Rect{X: r.X + 14 + float64(i)*42, Y: y, W: 34, H: 30})
		f.Controls[len(f.Controls)-1].Count = pages
	}
	g.addLabel(f, id, ui.Rect{X: r.X + 108, Y: y + 5, W: r.W - 122, H: 24}, g.tr("ui.developer.items.page", "page", page+1, "pages", pages, "count", len(entries)))
}

func (g *Game) developerAction(c uiControl) bool {
	b := &g.u.developerItems
	switch c.Action {
	case "developer-search":
		b.Editing = true
	case "developer-clear":
		b.Query = ""
	case "developer-category":
		b.Category = c.Value
	case "developer-grant":
		if g.link != nil && g.state != nil && g.state.Admin == g.id {
			g.link.Send(protocol.Request{Kind: "debug_grant_item", Item: c.Value})
		}
	default:
		return false
	}
	return true
}

func (g *Game) updateDeveloperSearch() {
	if g.u.focus == developerItemsWindow && g.u.developerItems.Editing && slices.Contains(g.u.windows, developerItemsWindow) {
		g.editText(&g.u.developerItems.Query, 80)
	}
}
