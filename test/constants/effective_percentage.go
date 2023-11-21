package constants

import "github.com/0xPolygonHermez/zkevm-sequencer/state"

var (
	EffectivePercentage     = []uint8{state.MaxEffectivePercentage}
	TwoEffectivePercentages = []uint8{state.MaxEffectivePercentage, state.MaxEffectivePercentage}
)
