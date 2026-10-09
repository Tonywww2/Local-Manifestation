package ui

// CharacterInventory describes display geometry only. Its height is virtual;
// the host clips and scrolls this content when the window is shorter.
type CharacterInventory struct {
	Character, Accessories, Bag, Consumables, Skills Rect
	Equipment                                        [5]Rect
	AttributeButton, LootButton, Help                Rect
}

func CharacterInventoryAt(r Rect) CharacterInventory {
	left := min(286, max(200, (r.W-42)*.31))
	x, right := r.X+left+28, r.W-left-42
	portrait := Rect{r.X + 65, r.Y + 76, left - 118, 232}
	q := CharacterInventory{
		Character:       portrait,
		Accessories:     Rect{x, r.Y + 76, right, 116},
		Bag:             Rect{x, r.Y + 260, right, 192},
		Consumables:     Rect{r.X + 14, r.Y + 352, left - 14, 54},
		Skills:          Rect{r.X + 14, r.Y + 446, left - 14, 54},
		AttributeButton: Rect{r.X + 14, r.Y + r.H - 48, left - 14, 34},
		LootButton:      Rect{x, r.Y + r.H - 48, 34, 34},
		Help:            Rect{x + 48, r.Y + r.H - 52, right - 48, 46},
	}
	q.Equipment[0] = Rect{r.X + 14, r.Y + 158, 44, 52}
	for i := 1; i < len(q.Equipment); i++ {
		q.Equipment[i] = Rect{r.X + left - 46, r.Y + 80 + float64(i-1)*58, 46, 50}
	}
	return q
}

func GridCell(r Rect, index, columns int, height float64) Rect {
	pitch := r.W / float64(columns)
	return Rect{r.X + float64(index%columns)*pitch, r.Y + float64(index/columns)*(height+8), pitch - 8, height}
}
