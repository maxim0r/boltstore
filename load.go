package boltstore

import (
	"fmt"

	"github.com/gorilla/sessions"
	bolt "go.etcd.io/bbolt"
)

// load reads the session from db.
// returns true if there is a sessoin data in DB
func (s *BoltStore) load(session *sessions.Session) (bool, error) {
	// exists represents whether a session data exists or not.
	var exists bool
	err := s.db.View(func(tx *bolt.Tx) error {
		id := []byte(session.ID)
		bucket := tx.Bucket(s.options.BucketName).Bucket(id)
		if bucket == nil {
			return fmt.Errorf("invalid session bucket %s/%s", string(s.options.BucketName), session.ID)
		}
		// Get the session data.
		data := bucket.Get(keyValues)
		if data == nil {
			return nil
		}

		if err := s.options.Serializer.Deserialize(data, session); err != nil {
			return err
		}
		exists = true
		return nil
	})
	return exists, err
}
