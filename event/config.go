package event

import "github.com/0xPolygonHermez/zkevm-sequencer/db"

// Config for event
type Config struct {
	// DB is the database configuration
	DB db.Config `mapstructure:"DB"`
}
