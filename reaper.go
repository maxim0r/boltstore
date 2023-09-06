package boltstore

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	bolt "go.etcd.io/bbolt"
)

func (s *BoltStore) worker(ctx context.Context) {

	// Create a new ticker
	ticker := time.NewTicker(s.options.ReapCheckInterval)
	defer ticker.Stop()

	for {
		select {

		case <-ctx.Done(): // Check if a quit signal is sent.
			return

		case <-ticker.C: // Check if the ticker fires a signal.
			// This slice is a buffer to save all expired session keys.
			expiredSessionKeys := make([][]byte, 0)

			// Start a bolt read transaction.
			err := s.db.View(func(tx *bolt.Tx) error {

				bucket := tx.Bucket(s.options.BucketName)
				if bucket == nil {
					return nil
				}

				var isExpired bool
				bucket.ForEach(func(k, v []byte) error {

					isExpired = false
					defer func() {
						if isExpired {
							temp := make([]byte, len(k))
							copy(temp, k)
							expiredSessionKeys = append(expiredSessionKeys, temp)
						}
					}()

					sessionBucket := bucket.Bucket(k)
					if sessionBucket == nil {
						return fmt.Errorf("invalid session bucket %s/%s for reap", string(s.options.BucketName), string(k))
					}

					// expiredAt key
					ev := sessionBucket.Get(keyExpiredAt)
					if ev == nil {
						isExpired = true
					}

					expiredAt, err := strconv.ParseInt(string(ev), 10, 64)
					if err != nil {
						isExpired = true
					} else {
						isExpired = time.Unix(expiredAt, 0).Before(time.Now())
					}

					return nil
				})

				return nil
			})

			if err != nil {
				log.Printf("boltstore: obtain expired sessions error: %v", err)
			}

			if len(expiredSessionKeys) > 0 {
				// Remove the expired sessions from the database
				err = s.db.Update(func(txu *bolt.Tx) error {
					// Get the bucket
					b := txu.Bucket(s.options.BucketName)
					if b == nil {
						return nil
					}

					// Remove all expired sessions in the slice
					for _, key := range expiredSessionKeys {
						err = b.Delete(key)
						if err != nil {
							return err
						}
					}

					return nil
				})

				if err != nil {
					log.Printf("boltstore: remove expired sessions error: %v", err)
				}
			}
		}
	}
}
