package farm

import (
	"reflect"
	"testing"

	"github.com/XOMAK2245/steam-card-xmk/internal/badges"
	"github.com/XOMAK2245/steam-card-xmk/internal/config"
)

func TestPlan(t *testing.T) {
	games := []badges.Game{
		{AppID: 1, Remaining: 2, Hours: 5},
		{AppID: 2, Remaining: 1, Hours: 0.5},
		{AppID: 3, Remaining: 4, Hours: 1.5},
		{AppID: 4, Remaining: 0, Hours: 0},
		{AppID: 5, Remaining: 1, Hours: 9},
	}
	got := Plan(games, nil, nil, config.Sequential, 30, 2)
	if !reflect.DeepEqual(got, []uint32{5}) {
		t.Errorf("sequential: %v", got)
	}
	got = Plan(games, nil, nil, config.Smart, 30, 2)
	if !reflect.DeepEqual(got, []uint32{3, 2}) {
		t.Errorf("smart phase1: %v", got)
	}
	got = Plan(games, nil, nil, config.Smart, 1, 2)
	if !reflect.DeepEqual(got, []uint32{3}) {
		t.Errorf("smart cap: %v", got)
	}
	got = Plan(games, map[uint32]float64{2: 3, 3: 3}, nil, config.Smart, 30, 2)
	if !reflect.DeepEqual(got, []uint32{5}) {
		t.Errorf("smart phase2: %v", got)
	}
	got = Plan(games, nil, map[uint32]bool{5: true}, config.Sequential, 30, 2)
	if !reflect.DeepEqual(got, []uint32{1}) {
		t.Errorf("skip: %v", got)
	}
}
