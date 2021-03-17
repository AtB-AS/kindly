package statistics_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/atb-as/kindly/statistics"
)

type doerFunc func(r *http.Request) (*http.Response, error)

func (d doerFunc) Do(r *http.Request) (*http.Response, error) {
	return d(r)
}

func TestClient_Doer(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		botID := "123"
		f := statistics.Filter{
			From: time.Date(2021, 2, 1, 0, 0, 0, 0, time.UTC),
			To:   time.Date(2021, 2, 2, 0, 0, 0, 0, time.UTC),
		}
		c := statistics.NewClient(statistics.WithDoer(doerFunc(func(r *http.Request) (*http.Response, error) {
			wantURL := fmt.Sprintf("%s/%s/chatlabels/added", statistics.BaseURL, botID)
			if !strings.HasPrefix(r.URL.String(), wantURL) {
				t.Errorf("got URL %q, want %q", r.URL.String(), wantURL)
			}

			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte("{\"data\":[]}")))}, nil
		})))
		c.BotID = botID

		if _, err := c.ChatLabels(context.Background(), &f); err != nil {
			t.Errorf("c.ChatLabels() err=%v", err)
		}
	})
	t.Run("Internal server error", func(t *testing.T) {
		c := statistics.NewClient(statistics.WithDoer(doerFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusInternalServerError, Body: ioutil.NopCloser(strings.NewReader(""))}, nil
		})))

		if _, err := c.ChatLabels(context.Background(), nil); err == nil {
			t.Errorf("expected err, got err=%v", err)
		} else if _, ok := err.(interface {
			Body() []byte
			StatusCode() int
		}); !ok {
			t.Errorf("expected err to implement Bodyer and StatusCoder")
		}
	})
}

type retryDoer struct {
	n int
}

func (d *retryDoer) Do(r *http.Request) (*http.Response, error) {
	d.n++
	status := http.StatusTooManyRequests
	if d.n > 2 {
		status = http.StatusOK
	}
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Length": []string{"3"}, "Retry-After": []string{"0"}}, Body: io.NopCloser(bytes.NewReader([]byte("")))}, nil
}

func TestClientDoer_Retries(t *testing.T) {
	doer := &retryDoer{}
	client := statistics.NewClient(statistics.WithDoer(doer))

	if _, err := client.UserMessages(context.Background(), nil); err != nil {
		t.Errorf("UserMessages() err=%v", err)
	}
	fmt.Println(doer.n)
	if doer.n != 3 {
		t.Errorf("expected doer to be called 3 times")
	}
}
