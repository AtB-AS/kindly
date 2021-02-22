package statistics

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_NewRequest(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		f := Filter{
			From: time.Date(2021, 2, 1, 0, 0, 0, 0, time.UTC),
			To:   time.Date(2021, 2, 2, 0, 0, 0, 0, time.UTC),
		}
		c := Client{
			BotID: "123",
		}

		req, err := c.newRequest(context.Background(), "test", f.Query())
		if err != nil {
			t.Errorf("newRequest() err=%v", err)
		}

		wantURL := fmt.Sprintf("%s/%s/test?from=2021-02-01&to=2021-02-02", BaseURL, c.BotID)
		if req.URL.String() != wantURL {
			t.Errorf("got URL %q, want %q", req.URL.String(), wantURL)
		}
	})
}

func TestClient_Do(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			accept := r.Header.Get("accept")
			if accept != "application/json" {
				t.Errorf("got Accept: %q, want %q", accept, "application/json")
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{\"data\":{}}"))
		}))

		c := Client{
			BotID:   "test",
			BaseURL: srv.URL,
			Doer:    srv.Client(),
		}

		req, err := c.newRequest(context.Background(), "test", nil)
		if err != nil {
			t.Errorf("newRequest err=%v", err)
		}

		err = c.do(req, nil)
		if err != nil {
			t.Errorf("do err=%v", err)
		}
	})
	t.Run("InternalServerError", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("{\"data\":{}}"))
		}))

		c := Client{
			BotID:   "test",
			BaseURL: srv.URL,
			Doer:    srv.Client(),
		}

		req, err := c.newRequest(context.Background(), "test", nil)
		if err != nil {
			t.Errorf("newRequest err=%v", err)
		}

		err = c.do(req, nil)
		if err == nil {
			t.Errorf("got err=%v, expected nil", err)
		}
	})
}
