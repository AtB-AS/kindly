package server

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/torfjor/kindly/statistics"
)

func NewServer(client *statistics.Client) http.Handler {
	m := mux.NewRouter()

	m.HandleFunc("/labels", newLabelsHandler(client))
	m.HandleFunc("/messages", newMessagesHandler(client))
	m.HandleFunc("/sessions", newSessionsHandler(client))

	return m
}

type sessionsReader interface {
	ChatSessions(ctx context.Context, f *statistics.Filter) ([]*statistics.CountByDate, error)
}

func newSessionsHandler(sr sessionsReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter, err := filterFromRequest(r)
		if err != nil {
			respondErr(w, err.Error(), http.StatusBadRequest)
			return
		}

		sessions, err := sr.ChatSessions(r.Context(), filter)
		if err != nil {
			respondErr(w, err.Error(), http.StatusInternalServerError)
			return
		}

		cw := newCSVWriter(w)
		cw.Write([]string{"date", "count"})
		for _, session := range sessions {
			cw.Write([]string{session.Date.Format("2006-01-02"), strconv.Itoa(session.Count)})
		}
		cw.Flush()

		if err := cw.Error(); err != nil {
			log.Printf("cw.Error() err=%v", err)
		}
	}
}

type messageReader interface {
	UserMessages(ctx context.Context, f *statistics.Filter) ([]*statistics.CountByDate, error)
}

func newMessagesHandler(mr messageReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter, err := filterFromRequest(r)
		if err != nil {
			respondErr(w, err.Error(), http.StatusBadRequest)
			return
		}

		messages, err := mr.UserMessages(r.Context(), filter)
		if err != nil {
			respondErr(w, err.Error(), http.StatusInternalServerError)
			return
		}

		cw := newCSVWriter(w)
		cw.Write([]string{"date", "count"})
		for _, message := range messages {
			cw.Write([]string{message.Date.Format("2006-01-02"), strconv.Itoa(message.Count)})
		}
		cw.Flush()

		if err := cw.Error(); err != nil {
			log.Printf("cw.Error() err=%v", err)
		}
	}
}

type labelReader interface {
	ChatLabels(ctx context.Context, f *statistics.Filter) ([]*statistics.ChatLabel, error)
}

func newLabelsHandler(lr labelReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter, err := filterFromRequest(r)
		if err != nil {
			respondErr(w, err.Error(), http.StatusBadRequest)
			return
		}

		labels, err := lr.ChatLabels(r.Context(), filter)
		if err != nil {
			respondErr(w, err.Error(), http.StatusInternalServerError)
			return
		}

		cw := newCSVWriter(w)
		cw.Write([]string{"count", "id", "text"})
		for _, label := range labels {
			cw.Write([]string{strconv.Itoa(label.Count), label.ID, label.Text})
		}
		cw.Flush()

		if err := cw.Error(); err != nil {
			log.Printf("cw.Error() err=%v", err)
		}
	}
}

func newCSVWriter(w http.ResponseWriter) *csv.Writer {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")

	return csv.NewWriter(w)
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
