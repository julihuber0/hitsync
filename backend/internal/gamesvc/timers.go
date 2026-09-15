package gamesvc

import "time"

// schedulePhaseTimeout arms a timer that invokes action back on the game's
// own goroutine after d, unless the phase has moved on in the meantime
// (guarded by phaseGeneration so a stale timer never fires late).
func (mg *ManagedGame) schedulePhaseTimeout(d time.Duration, action func()) {
	mg.phaseGeneration++
	gen := mg.phaseGeneration
	mg.phaseDeadline = time.Now().Add(d)
	if mg.phaseTimer != nil {
		mg.phaseTimer.Stop()
	}
	mg.phaseTimer = time.AfterFunc(d, func() {
		mg.enqueue(func() {
			if gen != mg.phaseGeneration {
				return
			}
			action()
		})
	})
}

// clearPhaseTimeout cancels any pending phase timer and invalidates it via
// the generation counter, and clears the displayed deadline.
func (mg *ManagedGame) clearPhaseTimeout() {
	mg.phaseGeneration++
	if mg.phaseTimer != nil {
		mg.phaseTimer.Stop()
		mg.phaseTimer = nil
	}
	mg.phaseDeadline = time.Time{}
}
