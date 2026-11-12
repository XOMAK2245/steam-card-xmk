// Package farm contains the farming engine: queue planning, badge polling
// and phantom process orchestration.
package farm

import (
	"sort"

	"github.com/XOMAK2245/steam-card-xmk/internal/badges"
	"github.com/XOMAK2245/steam-card-xmk/internal/config"
)

// Plan decides which AppIDs should be running right now.
//
//   - sequential: the single best game (most playtime first, since it drops soonest).
//   - smart: while there are games below the playtime threshold, run up to
//     maxParallel of them together; otherwise fall back to one game at a time.
//
// hours maps AppID to the *effective* playtime (Steam's value plus time we have
// been running since). skip contains AppIDs excluded (failed to start).
func Plan(games []badges.Game, hours map[uint32]float64, skip map[uint32]bool, strat config.Strategy, maxParallel int, thr float64) []uint32 {
	var q []badges.Game
	for _, g := range games {
		if g.Remaining > 0 && !skip[g.AppID] {
			q = append(q, g)
		}
	}
	h := func(g badges.Game) float64 {
		if v, ok := hours[g.AppID]; ok {
			return v
		}
		return g.Hours
	}
	sort.SliceStable(q, func(i, j int) bool { return h(q[i]) > h(q[j]) })
	if len(q) == 0 {
		return nil
	}
	if strat == config.Smart {
		var fresh []uint32
		for _, g := range q {
			if h(g) < thr {
				fresh = append(fresh, g.AppID)
			}
		}
		if len(fresh) > maxParallel {
			fresh = fresh[:maxParallel]
		}
		if len(fresh) > 0 {
			return fresh
		}
	}
	return []uint32{q[0].AppID}
}
