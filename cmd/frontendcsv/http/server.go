package http

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/atb-as/kindly/statistics"
	"github.com/gorilla/mux"
)

type rowWriter interface {
	Write(cols ...string) error
}

type csvHandler struct {
	h func(ctx context.Context, f *statistics.Filter, w rowWriter) error
}

type csvRowWriter struct {
	*csv.Writer
}

func (c *csvRowWriter) Write(cols ...string) error {
	return c.Writer.Write(cols)
}

// ServeHTTP implements http.Handler.
func (h *csvHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f, err := filterFromRequest(r)
	if err != nil {
		respondErr(w, err.Error(), http.StatusBadRequest)
		return
	}

	buf := bytes.Buffer{}
	cw := csv.NewWriter(&buf)
	if err := h.h(r.Context(), f, &csvRowWriter{cw}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	cw.Flush()
	if err := cw.Error(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
}

// NewServer returns a configured *http.Server that listens on 0.0.0.0:port.
func NewServer(client *statistics.Client, port string) *http.Server {
	m := mux.NewRouter()
	m.Handle("/labels", &csvHandler{
		h: func(ctx context.Context, f *statistics.Filter, w rowWriter) error {
			w.Write("date", "count", "id", "text")
			for t := f.From; f.To.Sub(t) > 0; t = t.Add(24 * time.Hour) {
				temp := *f
				temp.From = t
				temp.To = t.Add(24 * time.Hour)
				labels, err := client.ChatLabels(ctx, &temp)
				if err != nil {
					return err
				}
				for _, label := range labels {
					w.Write(formatTime(temp.From, f.Granularity), strconv.Itoa(label.Count), label.ID, label.Text)
				}
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

			w.Write("date", "count")
			for _, msg := range messages {
				w.Write(formatTime(msg.Date.Time, f.Granularity), strconv.Itoa(msg.Count))
			}

			return nil
		},
	})
	m.Handle("/pages", &csvHandler{
		h: func(ctx context.Context, f *statistics.Filter, w rowWriter) error {
			w.Write("date", "host", "path", "sessions", "messages")
			for t := f.From; f.To.Sub(t) > 0; t = t.Add(24 * time.Hour) {
				temp := *f
				temp.From = t
				temp.To = t.Add(24 * time.Hour)
				pages, err := client.PageStatistics(ctx, &temp)
				if err != nil {
					return err
				}
				for _, page := range pages {
					w.Write(formatTime(temp.From, f.Granularity), page.Host, page.Path, strconv.Itoa(page.Sessions), strconv.Itoa(page.Messages))
				}
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

			w.Write("date", "count")
			for _, session := range sessions {
				w.Write(formatTime(session.Date.Time, f.Granularity), strconv.Itoa(session.Count))
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

func formatTime(t time.Time, g statistics.Granularity) string {
	if g == statistics.Hour {
		return t.Format("2006-01-02 15:04")
	}

	return t.Format("2006-01-02")
}

func respondErr(w http.ResponseWriter, msg string, code int) {
	http.Error(w, msg, code)
}

func filterFromRequest(r *http.Request) (*statistics.Filter, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}

	f := &statistics.Filter{
		To:          time.Now(),
		From:        time.Now().Add(-1 * 24 * time.Hour),
		Limit:       10,
		Granularity: statistics.Day,
	}

	from := r.Form.Get("from")
	if from != "" {
		fromDate, err := time.Parse("2006-01-02", from)
		if err != nil {
			return nil, fmt.Errorf("parsing query: \"from\": %w", err)
		}
		f.From = fromDate
	}

	to := r.Form.Get("to")
	if to != "" {
		toDate, err := time.Parse("2006-01-02", to)
		if err != nil {
			return nil, fmt.Errorf("parsing query: \"to\": %w", err)
		}
		f.To = toDate
	}

	strLim := r.Form.Get("limit")
	if strLim != "" {
		lim, err := strconv.Atoi(strLim)
		if err != nil {
			return nil, fmt.Errorf("parsing query: \"limit\": %w", err)
		}
		f.Limit = lim
	}

	if f.To.Equal(f.From) {
		return nil, fmt.Errorf("parsing query: \"from\" and \"to\" are equal")
	}

	granularity := r.Form.Get("granularity")
	if granularity != "" {
		switch granularity {
		case "hour":
			f.Granularity = statistics.Hour
		}
	}

	return f, nil
}

// ErrServerClosed is aliased to avoid having to import net/http in parent.
var ErrServerClosed = http.ErrServerClosed
