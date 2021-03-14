package auth_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/torfjor/kindly/statistics/auth"
)

func TestApiKeyTokenSource_Token(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		want := struct {
			JWT string `json:"jwt"`
			TTL int    `json:"ttl"`
		}{
			JWT: "token",
			TTL: 300,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth != "Bearer key" {
				t.Errorf("got Authorization %q, want %q", auth, "Bearer key")
			}

			j, _ := json.Marshal(want)

			w.Header().Set("Content-type", "application/json")
			w.Write(j)
		}))

		src := auth.TokenSource{
			APIKey:   "key",
			TokenURL: srv.URL,
		}

		tok, err := src.Token()
		if err != nil {
			t.Errorf("err=%v", err)
		}

		if tok.AccessToken != "token" {
			t.Errorf("got AccessToken %q, want %q", tok.AccessToken, "token")
		}

		if tok.Expiry.Before(time.Now().Add(295 * time.Second)) {
			t.Errorf("unexpected expiry")
		}
	})
	t.Run("InternalServerError", func(t *testing.T) {
		srv := newTestSrv(http.StatusInternalServerError, nil)

		src := auth.TokenSource{
			TokenURL: srv.URL,
		}

		if _, err := src.Token(); err == nil {
			t.Errorf("expected err, got nil")
		} else if !errors.Is(err, auth.ErrRetrieveToken) {
			t.Errorf("expected err to wrap ErrRetrieveToken")
		}
	})
	t.Run("Unauthorized", func(t *testing.T) {
		srv := newTestSrv(http.StatusUnauthorized, nil)

		src := auth.TokenSource{
			TokenURL: srv.URL,
		}

		if _, err := src.Token(); err == nil {
			t.Errorf("expected err, got nil")
		} else if !errors.Is(err, auth.ErrRetrieveToken) {
			t.Errorf("expected err to wrap ErrRetrieveToken")
		}
	})
}

func newTestSrv(status int, resp []byte) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if resp != nil {
			w.Write(resp)
		}
		w.WriteHeader(status)
	}))
	return srv
}
