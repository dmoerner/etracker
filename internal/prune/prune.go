package prune

import (
	"context"
	"fmt"
	"time"

	"github.com/dmoerner/etracker/internal/config"
	"github.com/jackc/pgx/v5"
)

const (
	PruneIntervalMonths     = 3
	PruneIntervalTimerHours = 24 * 7 // 7 days
)

// PruneAnnounceKeys removes rows from the peers table, and corresponding
// announces from the announce table, for announce keys that have not been
// seen (either from original creation or last announce) for PruneInterval.
func PruneAnnounceKeys(ctx context.Context, conf config.Config) error {
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
		RETURNING
		    peers.announce_key
		`, PruneIntervalMonths, PruneIntervalMonths)
	rows, _ := conf.Dbpool.Query(ctx, query)
	keys, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return fmt.Errorf("error pruning old announce keys from postgres: %w", err)
	}
	if len(keys) > 0 {
		if err = conf.Rdb.Unlink(ctx, keys...).Err(); err != nil {
			// Since the Redis DB is persistent, it is an error if we
			// fail to invalidate these cache entries.
			return fmt.Errorf("error pruning old announce keys from redis: %w", err)
		}
	}

	return nil
}

func PruneTimer(ctx context.Context, conf config.Config, errCh chan error) {
	ticker := time.NewTicker(PruneIntervalTimerHours * time.Hour)

	go func() {
		for range ticker.C {
			err := PruneAnnounceKeys(ctx, conf)
			if err != nil {
				errCh <- err
				return
			}
		}
	}()
}
