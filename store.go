package boltstore

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	bolt "go.etcd.io/bbolt"
)

var (
	keyValues    = []byte("values")
	keyExpiredAt = []byte("expired_at")
)

type Options struct {
	KeyPairs          [][]byte
	KeyPrefix         string
	BucketName        []byte
	SessionExpire     time.Duration // Amount of time for cookies/boltdb keys to expire.
	Serializer        SessionSerializer
	MaxLength         int // max length of session data (0 - unlimited with caution)
	ReapCheckInterval time.Duration
}

func setOptions(o Options) Options {
	if o.KeyPrefix == "" {
		o.KeyPrefix = "session_"
	}
	if o.BucketName == nil {
		o.BucketName = []byte("sessions")
	}
	if o.SessionExpire == 0 {
		o.SessionExpire = time.Hour * 24
	}
	if o.Serializer == nil {
		o.Serializer = GobSerializer{}
	}
	if o.ReapCheckInterval == 0 {
		o.ReapCheckInterval = time.Minute
	}
	return o
}

// boltstore stores sessions in a boltdb backend.
type BoltStore struct {
	db      *bolt.DB
	Codecs  []securecookie.Codec
	Options *sessions.Options // default session configuration
	options Options           // store options
}

// NewStoreWithDB returns a new BoltStore.
func NewStoreWithDB(ctx context.Context, db *bolt.DB, opts Options) (*BoltStore, error) {
	opts = setOptions(opts)

	if opts.KeyPairs == nil {
		return nil, errors.New("store secret key is absent")
	}

	// Create buckets
	err := db.Update(func(tx *bolt.Tx) error {
		// main bucket
		if _, err := tx.CreateBucketIfNotExists(opts.BucketName); err != nil {
			return err
		}
		// values bucket
		// if _, err := b.CreateBucketIfNotExists(bucketValues); err != nil {
		// 	return err
		// }
		// // control bucket
		// if _, err := b.CreateBucketIfNotExists(bucketControl); err != nil {
		// 	return err
		// }
		return nil
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create sessions buckets %q error: %w", string(opts.BucketName), err)
	}

	bs := &BoltStore{
		db:     db,
		Codecs: securecookie.CodecsFromPairs(opts.KeyPairs...),
		Options: &sessions.Options{
			Path:   "/",
			MaxAge: int(opts.SessionExpire / time.Second),
		},
		options: opts,
	}

	go bs.worker(ctx)

	return bs, err
}

func NewStore(ctx context.Context, fn string, o Options) (*BoltStore, error) {
	db, err := bolt.Open(fn, 0600, &bolt.Options{Timeout: 3 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open bolt store %q error: %w", fn, err)
	}
	return NewStoreWithDB(ctx, db, o)
}

func (s *BoltStore) Close() error {
	return s.db.Close()
}

func (s *BoltStore) DB() *bolt.DB {
	return s.db
}

// Get returns a session for the given name after adding it to the registry.
//
// See gorilla/sessions FilesystemStore.Get().
func (s *BoltStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(s, name)
}

// New returns a session for the given name without adding it to the registry.
//
// See gorilla/sessions FilesystemStore.New().
func (s *BoltStore) New(r *http.Request, name string) (*sessions.Session, error) {
	var (
		err error
		ok  bool
	)
	session := sessions.NewSession(s, name)
	// make a copy
	options := *s.Options
	session.Options = &options
	session.IsNew = true
	if c, errCookie := r.Cookie(name); errCookie == nil {
		err = securecookie.DecodeMulti(name, c.Value, &session.ID, s.Codecs...)
		if err == nil {
			ok, err = s.load(session)
			session.IsNew = !(err == nil && ok) // not new if no error and data available
		}
	}
	return session, err
}

// delete removes keys
func (s *BoltStore) delete(session *sessions.Session) error {
	err := s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(s.options.BucketName).Bucket([]byte(session.ID))
		if bucket == nil {
			return fmt.Errorf("invalid session bucket %s/%s", string(s.options.BucketName), session.ID)
		}
		return bucket.Delete([]byte(session.ID))
	})
	if err != nil {
		return err
	}
	return nil
}
