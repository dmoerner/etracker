package prune

import (
	"context"
	"fmt"
	"time"

	"github.com/dmoerner/etracker/internal/config"
)

const (
	PruneIntervalMonths     = 3
	PruneIntervalTimerHours = 24 * 7 // 7 days
)

// PruneAnnounceKeys removes rows from the peers table, and corresponding
// announces from the announce table, for announce keys that have not been
// seen (either from original creation or last announce) for PruneInterval.
func PruneAnnounceKeys(conf config.Config) error {
	query := fmt.Sprintf(`
		DELETE FROM peers WHERE id IN
		(
		SELECT
		    peers.id
		FROM
		    peers
		    LEFT JOIN announces ON peers.id = announces.peers_id
		GROUP BY
		    peers.id
		HAVING (MAX(announces.last_announce) IS NULL
		    OR MAX(announces.last_announce) < NOW() - INTERVAL '%d months')
		AND (peers.created_time < NOW() - INTERVAL '%d months')
		)
		`, PruneIntervalMonths, PruneIntervalMonths)
	_, err := conf.Dbpool.Exec(context.Background(),
		query)
	if err != nil {
		return fmt.Errorf("error pruning old announce keys: %w", err)
	}
	return nil
}

func PruneTimer(conf config.Config, errCh chan error) {
	ticker := time.NewTicker(PruneIntervalTimerHours * time.Hour)

	go func() {
		for range ticker.C {
			err := PruneAnnounceKeys(conf)
			if err != nil {
				errCh <- err
				return
			}
		}
	}()
}
