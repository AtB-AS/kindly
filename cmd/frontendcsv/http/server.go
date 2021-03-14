package http

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/atb-as/kindly/statistics"
	"github.com/gorilla/mux"
)

type rowWriter interface {
	Write([]string) error
}

type csvHandler struct {
	h func(ctx context.Context, f *statistics.Filter, w rowWriter) error
}

func (h *csvHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f, err := filterFromRequest(r)
	if err != nil {
		respondErr(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/csv; encoding=utf-8")
	cw := csv.NewWriter(w)
	defer func() {
		cw.Flush()
		if err := cw.Error(); err != nil {
			log.Printf("cw.Error() err=%v\n", err)
		}
	}()

	if err := h.h(r.Context(), f, cw); err != nil {
		log.Printf("handler: err=%v\n", err)
	}
}

func NewServer(client *statistics.Client, port string) *http.Server {
	m := mux.NewRouter()
	m.Handle("/labels", &csvHandler{
		h: func(ctx context.Context, f *statistics.Filter, w rowWriter) error {
			labels, err := client.ChatLabels(ctx, f)
			if err != nil {
				return err
			}

			w.Write([]string{"count", "id", "text"})
			for _, label := range labels {
				w.Write([]string{strconv.Itoa(label.Count), label.ID, label.Text})
			}

			return nil
		},
	})
	m.Handle("/messages", &csvHandler{
		h: func(ctx context.Context, f *statistics.Filter, w rowWriter) error {
			messages, err := client.UserMessages(ctx, f)
			if err != nil {
				return err
			}

			w.Write([]string{"date", "count"})
			for _, msg := range messages {
				w.Write([]string{msg.Date.Format("2006-01-02"), strconv.Itoa(msg.Count)})
			}

			return nil
		},
	})
	m.Handle("/pages", &csvHandler{
		h: func(ctx context.Context, f *statistics.Filter, w rowWriter) error {
			pages, err := client.PageStatistics(ctx, f)
			if err != nil {
				return err
			}

			w.Write([]string{"host", "path", "sessions", "messages"})
			for _, page := range pages {
				w.Write([]string{page.Host, page.Path, strconv.Itoa(page.Sessions), strconv.Itoa(page.Messages)})
			}

			return nil
		},
	})
	m.Handle("/sessions", &csvHandler{
		h: func(ctx context.Context, f *statistics.Filter, w rowWriter) error {
			sessions, err := client.ChatSessions(ctx, f)
			if err != nil {
				return err
			}

			w.Write([]string{"date", "count"})
			for _, session := range sessions {
				w.Write([]string{session.Date.Format("2006-01-02"), strconv.Itoa(session.Count)})
			}

			return nil
		},
	})

	s := &http.Server{
		Addr:         ":" + port,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  30 * time.Second,
		Handler:      m,
	}

	return s
}

func respondErr(w http.ResponseWriter, msg string, code int) {
	http.Error(w, msg, code)
}

func filterFromRequest(r *http.Request) (*statistics.Filter, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}

	f := &statistics.Filter{
		To:    time.Now(),
		From:  time.Now().Add(-1 * 24 * time.Hour),
		Limit: 10,
	}

	from := r.Form.Get("from")
	if from != "" {
		fromDate, err := time.Parse("2006-01-02", from)
		if err != nil {
			return nil, fmt.Errorf("parsing query \"from\": %w", err)
		}
		f.From = fromDate
	}

	to := r.Form.Get("to")
	if to != "" {
		toDate, err := time.Parse("2006-01-02", to)
		if err != nil {
			return nil, fmt.Errorf("parsing query \"to\": %w", err)
		}
		f.To = toDate
	}

	strLim := r.Form.Get("limit")
	if strLim != "" {
		lim, err := strconv.Atoi(strLim)
		if err != nil {
			return nil, fmt.Errorf("parsing query \"limit\": %w", err)
		}
		f.Limit = lim
	}

	return f, nil
}

// ErrServerClosed is aliased to avoid having to import net/http in parent.
var ErrServerClosed = http.ErrServerClosed
