package boltstore

import (
	"encoding/base32"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	bolt "go.etcd.io/bbolt"
)

// Save adds a single session to the response.
func (s *BoltStore) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	// Marked for deletion.
	if session.Options.MaxAge <= 0 {
		if err := s.delete(session); err != nil {
			return fmt.Errorf("delete session from store error: %w", err)
		}
		http.SetCookie(w, sessions.NewCookie(session.Name(), "", session.Options))
	} else {
		// Build an alphanumeric key for the store.
		if session.ID == "" {
			session.ID = strings.TrimRight(base32.StdEncoding.EncodeToString(securecookie.GenerateRandomKey(32)), "=")
		}
		if err := s.save(session); err != nil {
			return fmt.Errorf("save session to store error: %w", err)
		}
		encoded, err := securecookie.EncodeMulti(session.Name(), session.ID, s.Codecs...)
		if err != nil {
			return fmt.Errorf("encode cookie error: %w", err)
		}
		http.SetCookie(w, sessions.NewCookie(session.Name(), encoded, session.Options))
	}
	return nil
}

// save stores the session in db.
func (s *BoltStore) save(session *sessions.Session) error {

	b, err := s.options.Serializer.Serialize(session)
	if err != nil {
		return fmt.Errorf("serialize session error: %w", err)
	}

	if s.options.MaxLength != 0 && len(b) > s.options.MaxLength {
		return errors.New("SessionStore: the value to store is too big")
	}

	expiredAt := []byte(strconv.FormatInt(time.Now().Add(time.Duration(s.options.SessionExpire)).Unix(), 10))

	err = s.db.Update(func(tx *bolt.Tx) error {

		// session root bucket
		root, err := tx.Bucket(s.options.BucketName).CreateBucketIfNotExists([]byte(session.ID))
		if err != nil {
			return fmt.Errorf("create session bucket error: %w", err)
		}

		// store values
		if err := root.Put(keyValues, b); err != nil {
			return fmt.Errorf("put session value to store error: %w", err)
		}

		// store control data
		if err := root.Put(keyExpiredAt, expiredAt); err != nil {
			return fmt.Errorf("put session expireAt to store error: %w", err)
		}

		return nil
	})
	return err
}
