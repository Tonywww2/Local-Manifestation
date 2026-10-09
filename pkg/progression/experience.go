package progression

// NextLevelXP is the current development progression curve. Both the owner and
// the HUD use this value; changing presentation must never change earned XP.
func NextLevelXP(level int) int { return max(1, level) * 20 }
