package http

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/atb-as/kindly/statistics"
	"github.com/gorilla/mux"
)

type rowWriter interface {
	WriteAll(rows [][]string) error
}

type csvHandler struct {
	hdr []string
	h   func(ctx context.Context, f *statistics.Filter, w rowWriter) error
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

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	cw := csv.NewWriter(w)
	cw.Write(h.hdr)

	if err := h.h(r.Context(), f, &csvRowWriter{cw}); err != nil {
		fmt.Fprintf(os.Stderr, "handler: err=%v\n", err)
		return
	}

	cw.Flush()
	if err := cw.Error(); err != nil {
		fmt.Fprintf(os.Stderr, "handler: flush: err=%v\n", err)
		return
	}
}

// NewServer returns a configured *http.Server that listens on 0.0.0.0:port.
func NewServer(client *statistics.Client, port string) *http.Server {
	m := mux.NewRouter()
	m.Handle("/labels", &csvHandler{
		hdr: []string{"date", "count", "id", "text", "source"},
		h: func(ctx context.Context, f *statistics.Filter, w rowWriter) error {
			for t := f.From; t.Before(f.To); t = t.Add(24 * time.Hour) {
				for _, source := range f.Sources {
					temp := *f
					temp.From = t
					temp.To = t.Add(24 * time.Hour)
					temp.Sources = []string{source}
					labels, err := client.ChatLabels(ctx, &temp)
					if err != nil {
						return err
					}

					out := make([][]string, 0, f.Limit)
					for _, label := range labels {
						out = append(out, []string{formatTime(temp.From, f.Granularity), strconv.Itoa(label.Count), label.ID, label.Text, source})
					}
					if err := w.WriteAll(out); err != nil {
						return err
					}
				}
			}
			return nil
		},
	})
	m.Handle("/messages", &csvHandler{
		hdr: []string{"date", "count", "source"},
		h: func(ctx context.Context, f *statistics.Filter, w rowWriter) error {
			out := make([][]string, 0, f.Limit)
			for _, source := range f.Sources {
				temp := *f
				temp.Sources = []string{source}
				messages, err := client.UserMessages(ctx, &temp)

				if err != nil {
					return err
				}

				for _, msg := range messages {
					out = append(out, []string{formatTime(msg.Date.Time, f.Granularity), strconv.Itoa(msg.Count), source})
				}
			}

			return w.WriteAll(out)
		},
	})
	m.Handle("/pages", &csvHandler{
		hdr: []string{"date", "host", "path", "sessions", "messages", "source"},
		h: func(ctx context.Context, f *statistics.Filter, w rowWriter) error {
			for t := f.From; t.Before(f.To); t = t.Add(24 * time.Hour) {
				for _, source := range f.Sources {
					temp := *f
					temp.From = t
					temp.To = t.Add(24 * time.Hour)
					temp.Sources = []string{source}
					pages, err := client.PageStatistics(ctx, &temp)
					if err != nil {
						return err
					}

					out := make([][]string, 0, f.Limit)
					for _, page := range pages {
						out = append(out, []string{formatTime(temp.From, f.Granularity), page.Host, page.Path, strconv.Itoa(page.Sessions), strconv.Itoa(page.Messages), source})
					}
					if err := w.WriteAll(out); err != nil {
						return err
					}
				}
			}
			return nil
		},
	})
	m.Handle("/sessions", &csvHandler{
		hdr: []string{"date", "count", "source"},
		h: func(ctx context.Context, f *statistics.Filter, w rowWriter) error {
			out := make([][]string, 0, f.Limit)
			for _, source := range f.Sources {
				temp := *f
				temp.Sources = []string{source}
				sessions, err := client.ChatSessions(ctx, &temp)
				if err != nil {
					return err
				}

				for _, session := range sessions {
					out = append(out, []string{formatTime(session.Date.Time, f.Granularity), strconv.Itoa(session.Count), source})
				}
			}
			return w.WriteAll(out)
		},
	})

	s := &http.Server{
		Addr:        ":" + port,
		ReadTimeout: 5 * time.Second,
		Handler:     m,
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

	sources, ok := r.Form["sources"]
	if ok {
		for _, source := range sources {
			f.Sources = append(f.Sources, source)
		}
	}
	if len(f.Sources) == 0 {
		f.Sources = append(f.Sources, "web", "facebook")
	}

	return f, nil
}

// ErrServerClosed is aliased to avoid having to import net/http in parent.
var ErrServerClosed = http.ErrServerClosed
