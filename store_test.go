package boltstore

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/sessions"
)

// ----------------------------------------------------------------------------
// ResponseRecorder
// ----------------------------------------------------------------------------
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// ResponseRecorder is an implementation of http.ResponseWriter that
// records its mutations for later inspection in tests.
type ResponseRecorder struct {
	Code      int           // the HTTP response code from WriteHeader
	HeaderMap http.Header   // the HTTP response headers
	Body      *bytes.Buffer // if non-nil, the bytes.Buffer to append written data to
	Flushed   bool
}

// NewRecorder returns an initialized ResponseRecorder.
func NewRecorder() *ResponseRecorder {
	return &ResponseRecorder{
		HeaderMap: make(http.Header),
		Body:      new(bytes.Buffer),
	}
}

// DefaultRemoteAddr is the default remote address to return in RemoteAddr if
// an explicit DefaultRemoteAddr isn't set on ResponseRecorder.
const DefaultRemoteAddr = "1.2.3.4"

// Header returns the response headers.
func (rw *ResponseRecorder) Header() http.Header {
	return rw.HeaderMap
}

// Write always succeeds and writes to rw.Body, if not nil.
func (rw *ResponseRecorder) Write(buf []byte) (int, error) {
	if rw.Body != nil {
		rw.Body.Write(buf)
	}
	if rw.Code == 0 {
		rw.Code = http.StatusOK
	}
	return len(buf), nil
}

// WriteHeader sets rw.Code.
func (rw *ResponseRecorder) WriteHeader(code int) {
	rw.Code = code
}

// Flush sets rw.Flushed to true.
func (rw *ResponseRecorder) Flush() {
	rw.Flushed = true
}

// ----------------------------------------------------------------------------

type FlashMessage struct {
	Type    int
	Message string
}

func TestBoltStore(t *testing.T) {
	var (
		req     *http.Request
		rsp     *ResponseRecorder
		hdr     http.Header
		ok      bool
		cookies []string
		session *sessions.Session
		flashes []interface{}
	)

	ctx := context.Background()

	// Copyright 2012 The Gorilla Authors. All rights reserved.
	// Use of this source code is governed by a BSD-style
	// license that can be found in the LICENSE file.

	opts := Options{
		KeyPairs:  [][]byte{[]byte("secret-key")},
		MaxLength: 4096,
	}

	// clear db
	os.Remove("test.db")

	// Round 1 ----------------------------------------------------------------
	{
		// BoltStore
		store, err := NewStore(ctx, "test.db", opts)
		if err != nil {
			t.Fatal(err.Error())
		}
		defer store.Close()

		req, _ := http.NewRequest("GET", "http://localhost:8080/", nil)
		rsp = NewRecorder()
		// Get a session.
		if session, err = store.Get(req, "session-key"); err != nil {
			t.Fatalf("Error getting session: %v", err)
		}
		// Get a flash.
		flashes = session.Flashes()
		if len(flashes) != 0 {
			t.Errorf("Expected empty flashes; Got %v", flashes)
		}
		// Add some flashes.
		session.AddFlash("foo")
		session.AddFlash("bar")
		// Custom key.
		session.AddFlash("baz", "custom_key")
		// Save.
		if err = sessions.Save(req, rsp); err != nil {
			t.Fatalf("Error saving session: %v", err)
		}
		hdr = rsp.Header()
		cookies, ok = hdr["Set-Cookie"]
		if !ok || len(cookies) != 1 {
			t.Fatalf("No cookies. Header: %s", hdr)
		}

		store.Close()
	}

	// Round 2 ----------------------------------------------------------------
	{
		store, err := NewStore(ctx, "test.db", opts)
		if err != nil {
			t.Fatal(err.Error())
		}
		defer store.Close()

		req, _ := http.NewRequest("GET", "http://localhost:8080/", nil)
		req.Header.Add("Cookie", cookies[0])
		rsp = NewRecorder()
		// Get a session.
		if session, err = store.Get(req, "session-key"); err != nil {
			t.Fatalf("Error getting session: %v", err)
		}
		// Check all saved values.
		flashes = session.Flashes()
		if len(flashes) != 2 {
			t.Fatalf("Expected flashes; Got %v", flashes)
		}
		if flashes[0] != "foo" || flashes[1] != "bar" {
			t.Errorf("Expected foo,bar; Got %v", flashes)
		}
		flashes = session.Flashes()
		if len(flashes) != 0 {
			t.Errorf("Expected dumped flashes; Got %v", flashes)
		}
		// Custom key.
		flashes = session.Flashes("custom_key")
		if len(flashes) != 1 {
			t.Errorf("Expected flashes; Got %v", flashes)
		} else if flashes[0] != "baz" {
			t.Errorf("Expected baz; Got %v", flashes)
		}
		flashes = session.Flashes("custom_key")
		if len(flashes) != 0 {
			t.Errorf("Expected dumped flashes; Got %v", flashes)
		}

		// Save.
		if err = sessions.Save(req, rsp); err != nil {
			t.Fatalf("Error saving session: %v", err)
		}

		store.Close()
	}

	// Round 3 ----------------------------------------------------------------
	// Custom type

	// BoltStore
	{
		store, err := NewStore(ctx, "test.db", opts)
		if err != nil {
			t.Fatal(err.Error())
		}
		defer store.Close()

		req, _ = http.NewRequest("GET", "http://localhost:8080/", nil)
		rsp = NewRecorder()
		// Get a session.
		if session, err = store.Get(req, "session-key"); err != nil {
			t.Fatalf("Error getting session: %v", err)
		}
		// Get a flash.
		flashes = session.Flashes()
		if len(flashes) != 0 {
			t.Errorf("Expected empty flashes; Got %v", flashes)
		}
		// Add some flashes.
		session.AddFlash(&FlashMessage{42, "foo"})
		// Save.
		if err = sessions.Save(req, rsp); err != nil {
			t.Fatalf("Error saving session: %v", err)
		}
		hdr = rsp.Header()
		cookies, ok = hdr["Set-Cookie"]
		if !ok || len(cookies) != 1 {
			t.Fatalf("No cookies. Header: %s", hdr)
		}

		store.Close()
	}

	// Round 4 ----------------------------------------------------------------
	// Custom type
	{
		store, err := NewStore(ctx, "test.db", opts)
		if err != nil {
			t.Fatal(err.Error())
		}
		defer store.Close()

		req, _ := http.NewRequest("GET", "http://localhost:8080/", nil)
		req.Header.Add("Cookie", cookies[0])
		rsp = NewRecorder()
		// Get a session.
		if session, err = store.Get(req, "session-key"); err != nil {
			t.Fatalf("Error getting session: %v", err)
		}
		// Check all saved values.
		flashes = session.Flashes()
		if len(flashes) != 1 {
			t.Fatalf("Expected flashes; Got %v", flashes)
		}
		custom := flashes[0].(FlashMessage)
		if custom.Type != 42 || custom.Message != "foo" {
			t.Errorf("Expected %#v, got %#v", FlashMessage{42, "foo"}, custom)
		}

		// Save.
		if err = sessions.Save(req, rsp); err != nil {
			t.Fatalf("Error saving session: %v", err)
		}

		store.Close()
	}

	{
		store, err := NewStore(ctx, "test.db", opts)
		if err != nil {
			t.Fatal(err.Error())
		}
		req, err = http.NewRequest("GET", "http://www.example.com", nil)
		if err != nil {
			t.Fatal("failed to create request", err)
		}
		w := httptest.NewRecorder()

		session, _ = store.New(req, "my session")
		session.Values["big"] = make([]byte, base64.StdEncoding.DecodedLen(4096*2))
		err = session.Save(req, w)
		if err == nil {
			t.Fatal("expected an error, got nil")
		}

		store.Close()
	}

	// Round 7 ----------------------------------------------------------------

	{
		store, err := NewStore(ctx, "test.db", opts)
		if err != nil {
			t.Fatal(err.Error())
		}
		defer store.Close()

		req, _ = http.NewRequest("GET", "http://localhost:8080/", nil)
		rsp = NewRecorder()
		// Get a session. Using the same key as previously, but on different DB
		if session, err = store.Get(req, "session-key"); err != nil {
			t.Fatalf("Error getting session: %v", err)
		}
		// Get a flash.
		flashes = session.Flashes()
		if len(flashes) != 0 {
			t.Errorf("Expected empty flashes; Got %v", flashes)
		}
		// Add some flashes.
		session.AddFlash("foo")
		// Save.
		if err = sessions.Save(req, rsp); err != nil {
			t.Fatalf("Error saving session: %v", err)
		}
		hdr = rsp.Header()
		cookies, ok = hdr["Set-Cookie"]
		if !ok || len(cookies) != 1 {
			t.Fatalf("No cookies. Header: %s", hdr)
		}

		// Get a session.
		req.Header.Add("Cookie", cookies[0])
		if session, err = store.Get(req, "session-key"); err != nil {
			t.Fatalf("Error getting session: %v", err)
		}
		// Check all saved values.
		flashes = session.Flashes()
		if len(flashes) != 1 {
			t.Fatalf("Expected flashes; Got %v", flashes)
		}
		if flashes[0] != "foo" {
			t.Errorf("Expected foo,bar; Got %v", flashes)
		}

		store.Close()
	}

	// Round 8 ----------------------------------------------------------------
	// JSONSerializer

	{
		opts.Serializer = JSONSerializer{}
		store, err := NewStore(ctx, "test.db", opts)
		if err != nil {
			t.Fatal(err.Error())
		}
		defer store.Close()

		req, _ = http.NewRequest("GET", "http://localhost:8080/", nil)
		rsp = NewRecorder()
		// Get a session.
		if session, err = store.Get(req, "session-key"); err != nil {
			t.Fatalf("Error getting session: %v", err)
		}
		// Get a flash.
		flashes = session.Flashes()
		if len(flashes) != 0 {
			t.Errorf("Expected empty flashes; Got %v", flashes)
		}
		// Add some flashes.
		session.AddFlash("foo")
		// Save.
		if err = sessions.Save(req, rsp); err != nil {
			t.Fatalf("Error saving session: %v", err)
		}
		hdr = rsp.Header()
		cookies, ok = hdr["Set-Cookie"]
		if !ok || len(cookies) != 1 {
			t.Fatalf("No cookies. Header: %s", hdr)
		}

		// Get a session.
		req.Header.Add("Cookie", cookies[0])
		if session, err = store.Get(req, "session-key"); err != nil {
			t.Fatalf("Error getting session: %v", err)
		}
		// Check all saved values.
		flashes = session.Flashes()
		if len(flashes) != 1 {
			t.Fatalf("Expected flashes; Got %v", flashes)
		}
		if flashes[0] != "foo" {
			t.Errorf("Expected foo,bar; Got %v", flashes)
		}

		store.Close()
	}

	if err := os.Remove("test.db"); err != nil {
		t.Fatal(err)
	}
}

func ExampleBoltStore() {
	store, err := NewStore(context.Background(), "example.db", Options{})
	if err != nil {
		panic(err)
	}
	defer store.Close()
}

func init() {
	gob.Register(FlashMessage{})
}
