package statistics

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/atb-as/kindly"
)

const BaseURL = "https://sage.kindly.ai/api/v1/stats/bot"

type Client struct {
	BotID   string
	BaseURL string
	logger  Logger
	doer    Doer
}

func NewClient(opts ...ClientOption) *Client {
	c := &Client{logger: &nopLogger{}, doer: http.DefaultClient}
	for _, opt := range opts {
		opt(c)
	}

	return c
}

type ClientOption func(c *Client)

func WithDoer(doer Doer) ClientOption {
	return func(c *Client) {
		c.doer = doer
	}
}

func WithLogger(logger Logger) ClientOption {
	return func(c *Client) {
		c.logger = logger
	}
}

type Logger interface {
	Log(keyvals ...interface{}) error
}

type nopLogger struct {
}

func (l *nopLogger) Log(keyvals ...interface{}) error {
	return nil
}

type Doer interface {
	Do(r *http.Request) (*http.Response, error)
}

type Granularity int

const (
	Unspecified Granularity = iota
	Day
	Hour
	Week
)

func (g Granularity) String() string {
	switch g {
	case Day:
		return "day"
	case Hour:
		return "hour"
	case Week:
		return "week"
	default:
		return "day"
	}
}

type Filter struct {
	From          time.Time
	To            time.Time
	Timezone      string
	Limit         int
	Granularity   Granularity
	Sources       []string
	LanguageCodes []string
}

const dateLayout = "2006-01-02"

func (f *Filter) Query() url.Values {
	if f == nil {
		return url.Values{}
	}

	q := url.Values{}

	if !f.From.IsZero() {
		q.Add("from", f.From.Format(dateLayout))
	}

	if !f.To.IsZero() {
		q.Add("to", f.To.Format(dateLayout))
	}

	if f.Granularity != Unspecified {
		q.Add("granularity", f.Granularity.String())
	}

	if f.Limit != 0 {
		q.Add("limit", strconv.Itoa(f.Limit))
	}

	return q
}

type responseWrapper struct {
	Data json.RawMessage `json:"data"`
}

type CountByDate struct {
	Count int
	Date  kindly.Time
}

type RateTotal struct {
	Count int
	Rate  float64
}

type CountByDateWithRate struct {
	CountByDate
	Rate float64
}

type PageStatistic struct {
	Messages int
	Sessions int
	Host     string `json:"web_host"`
	Path     string `json:"web_path"`
}

type HandoversTimeSeries struct {
	Date kindly.Time
	Handovers
}

type Handovers struct {
	Ended               int
	Requests            int
	RequestsWhileClosed int `json:"requests_while_closed"`
	Started             int
}

// Feedback is a container for user feedback ratings.
type Feedback struct {
	Binary []*Rating
	Emojis []*Rating
}

// Rating represents aggregated user ratings.
type Rating struct {
	Count  int
	Rating int
	Ratio  float64
}

// AggregatedFeedback returns the aggregated ratings of the bot given by users
// in the specified period.
func (c *Client) AggregatedFeedback(ctx context.Context, f *Filter) (*Feedback, error) {
	req, err := c.newRequest(ctx, "feedback/summary", f.Query())
	if err != nil {
		return nil, err
	}

	ret := Feedback{}
	if err := c.do(req, &ret); err != nil {
		return nil, err
	}

	return &ret, nil
}

// HandoversTotal returns the total number of handover requests (while open),
// requests while closed, started handovers and ended handovers in the requested
// time period.
func (c *Client) HandoversTotal(ctx context.Context, f *Filter) (*Handovers, error) {
	req, err := c.newRequest(ctx, "takeovers/totals", f.Query())
	if err != nil {
		return nil, err
	}

	ret := Handovers{}
	if err := c.do(req, &ret); err != nil {
		return nil, err
	}

	return &ret, nil
}

// HandoversTimeSeries returns the number of handover requests (while open),
// requests while closed, started handovers and ended handovers in the requested
// time period, as a time series.
func (c *Client) HandoversTimeSeries(ctx context.Context, f *Filter) ([]*HandoversTimeSeries, error) {
	req, err := c.newRequest(ctx, "takeovers/series", f.Query())
	if err != nil {
		return nil, err
	}

	ret := make([]*HandoversTimeSeries, 0)
	if err := c.do(req, &ret); err != nil {
		return nil, err
	}

	return ret, nil
}

// PageStatistics lists the most frequent web pages where interactions with the
// bot has happened. Returns top 3 pages by default, use f.Limit parameter to
// request more results.
func (c *Client) PageStatistics(ctx context.Context, f *Filter) ([]*PageStatistic, error) {
	req, err := c.newRequest(ctx, "chatbubble/pages", f.Query())
	if err != nil {
		return nil, err
	}

	ret := make([]*PageStatistic, 0)
	if err := c.do(req, &ret); err != nil {
		return nil, err
	}

	return ret, nil
}

// FallbackRateTotal returns the number of and fraction of bot replies that are
// fallbacks, as a total aggregate for the selected time interval.
func (c *Client) FallbackRateTotal(ctx context.Context, f *Filter) (*RateTotal, error) {
	req, err := c.newRequest(ctx, "fallbacks/total", f.Query())
	if err != nil {
		return nil, err
	}

	ret := RateTotal{}
	if err := c.do(req, &ret); err != nil {
		return nil, err
	}

	return &ret, nil
}

// FallbackRateTimeSeries returns the number of and fraction of bot replies that
// are fallbacks, as an aggregated time series.
func (c *Client) FallbackRateTimeSeries(ctx context.Context, f *Filter) ([]*CountByDateWithRate, error) {
	req, err := c.newRequest(ctx, "fallbacks/series", f.Query())
	if err != nil {
		return nil, err
	}

	ret := make([]*CountByDateWithRate, 0)
	if err := c.do(req, &ret); err != nil {
		return nil, err
	}

	return ret, nil
}

// UserMessages returns the number of messages from users.
func (c *Client) UserMessages(ctx context.Context, f *Filter) ([]*CountByDate, error) {
	req, err := c.newRequest(ctx, "sessions/messages", f.Query())
	if err != nil {
		return nil, err
	}

	ret := make([]*CountByDate, 0)
	if err := c.do(req, &ret); err != nil {
		return nil, err
	}

	return ret, nil
}

// ChatSessions returns the number of chats where users engaged with the bot.
func (c *Client) ChatSessions(ctx context.Context, f *Filter) ([]*CountByDate, error) {
	req, err := c.newRequest(ctx, "sessions/chats", f.Query())
	if err != nil {
		return nil, err
	}

	ret := make([]*CountByDate, 0)
	if err := c.do(req, &ret); err != nil {
		return nil, err
	}

	return ret, nil
}

type ChatLabel struct {
	Count int    `json:"count"`
	ID    string `json:"label_id"`
	Text  string `json:"label_text"`
}

func (c *Client) ChatLabels(ctx context.Context, f *Filter) ([]*ChatLabel, error) {
	req, err := c.newRequest(ctx, "chatlabels/added", f.Query())
	if err != nil {
		return nil, err
	}

	ret := make([]*ChatLabel, 0)
	if err := c.do(req, &ret); err != nil {
		return nil, err
	}

	return ret, nil
}

func (c *Client) newRequest(ctx context.Context, endpoint string, query url.Values) (*http.Request, error) {
	if c.BaseURL == "" {
		c.BaseURL = BaseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/%s/%s", c.BaseURL, c.BotID, endpoint), nil)
	if err != nil {
		return nil, err
	}
	req.URL.RawQuery = query.Encode()
	req.Header.Set("Accept", "application/json")

	return req, nil
}

type Error struct {
	statusCode int
	body       []byte
}

func (e *Error) StatusCode() int {
	return e.statusCode
}

func (e *Error) Body() []byte {
	return e.body
}

func (e *Error) Error() string {
	return fmt.Sprintf("statistics: errenous status from upstream: %q", http.StatusText(e.StatusCode()))
}

func (c *Client) do(r *http.Request, v interface{}) error {
	if c.doer == nil {
		c.doer = http.DefaultClient
	}
	begin := time.Now()

	for {
		resp, err := c.doer.Do(r)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		c.logger.Log("method", r.Method, "url", r.URL.String(), "code", resp.StatusCode, "took", time.Since(begin))

		if resp.StatusCode == http.StatusTooManyRequests {
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				waitSeconds, err := strconv.Atoi(retryAfter)
				if err != nil {
					return newResponseError(resp)
				}
				select {
				case <-r.Context().Done():
					return r.Context().Err()
				case <-time.After(time.Duration(waitSeconds) * time.Second):
					continue
				}
			}
			continue
		} else if resp.StatusCode > 399 {
			return newResponseError(resp)
		}

		w := responseWrapper{}
		if err := json.NewDecoder(resp.Body).Decode(&w); err != nil {
			return nil
		}

		if v == nil {
			return nil
		}
		return json.Unmarshal(w.Data, &v)
	}
}

func newResponseError(resp *http.Response) error {
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return &Error{statusCode: resp.StatusCode, body: body}
}
