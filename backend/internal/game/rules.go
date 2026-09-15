package game

// PlacementCorrect reports whether inserting a card of the given year at
// slot i in a timeline of length n is correct (§8.7). Valid slot indices are
// 0..n inclusive; equal years are always acceptable.
func PlacementCorrect(timeline []Card, year int, slot int) bool {
	n := len(timeline)
	if slot < 0 || slot > n {
		return false
	}
	if slot > 0 && year < timeline[slot-1].Year {
		return false
	}
	if slot < n && year > timeline[slot].Year {
		return false
	}
	return true
}

// InsertCard returns a new timeline with card inserted at slot, keeping the
// ascending-by-year ordering invariant.
func InsertCard(timeline []Card, slot int, card Card) []Card {
	out := make([]Card, 0, len(timeline)+1)
	out = append(out, timeline[:slot]...)
	out = append(out, card)
	out = append(out, timeline[slot:]...)
	return out
}
