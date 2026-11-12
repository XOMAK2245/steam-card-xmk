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
//   - batch: run up to maxParallel games with remaining card drops simultaneously.
//   - single: run 1 game at a time (most playtime first, then fewest cards remaining).
//
// hours maps AppID to the *effective* playtime (Steam's value plus time we have
// been running since). skip contains AppIDs excluded (failed to start).
func Plan(games []badges.Game, hours map[uint32]float64, skip map[uint32]bool, strat config.Strategy, maxParallel int) []uint32 {
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
	sort.SliceStable(q, func(i, j int) bool {
		hi, hj := h(q[i]), h(q[j])
		if hi != hj {
			return hi > hj
		}
		return q[i].Remaining < q[j].Remaining
	})
	if len(q) == 0 {
		return nil
	}
	if strat == config.Single {
		return []uint32{q[0].AppID}
	}
	if maxParallel < 1 {
		maxParallel = 32
	}
	if len(q) > maxParallel {
		q = q[:maxParallel]
	}
	out := make([]uint32, len(q))
	for i, g := range q {
		out[i] = g.AppID
	}
	return out
}
