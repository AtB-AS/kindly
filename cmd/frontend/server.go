package htmlstats

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/torfjor/kindly/statistics"
)

var (
	statsClient *statistics.Client
	tmpl        = template.Must(template.New("stats").Parse(`
<!DOCTYPE html>
<html>
<head>
    <title>kindly.ai Statistics</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width,minimum-scale=1">
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.0.0-beta2/dist/css/bootstrap.min.css"
          rel="stylesheet"
          integrity="sha384-BmbxuPwQa2lc/FVzBcNJ7UAyJxM6wuqIj61tLrc4wSX0szH/Ev+nYRRuWlolflfl"
          crossorigin="anonymous">
</head>
<body>
<div class="container">
    <h2>kindly.ai Statistics</h2>
    <form method="get">
        <div class="row">
            <div class="col-auto mb-3">
                <label class="form-label" for="statistic">Metric:</label>
                <select class="form-select" id="statistic" name="metric">
                    <option value="chats"
                            {{if eq .Filter.Metric "chats"}}selected{{end}}>Chat
                        sessions
                    </option>
                    <option value="messages"
                            {{if eq .Filter.Metric "messages"}}selected{{end}}>
                        User
                        messages
                    </option>
                    <option value="pages"
                            {{if eq .Filter.Metric "pages"}}selected{{end}}>Web
                        pages
                        (aggregated)
                    </option>
                    <option value="feedback"
                            {{if eq .Filter.Metric "feedback"}}selected{{end}}>
                        Feedback
                        (aggregated)
                    </option>
					<option value="labels"
							{{if eq .Filter.Metric "labels"}}selected{{end}}>
						Labels
					</option>
                </select>
            </div>
            <div class="col-auto mb-3">
                <label class="form-label" for="from">From:</label>
                <input class="form-control" id="from" type="date"
                       name="from"
					   placeholder="2021-01-01"
                       value="{{ .Filter.From }}"/>
            </div>
            <div class="col-auto mb-3">
                <label class="form-label" for="to">To:</label>
                <input class="form-control" id="to" type="date" name="to" placeholder="2021-01-02"
                       value="{{ .Filter.To }}"/>
            </div>
            <div class="col-auto align-self-end mb-3">
                <button class="btn btn-primary" type="submit">Submit
                </button>
            </div>
        </div>

    </form>
    <textarea class="form-control" readonly rows="20">{{.CSV}}</textarea>
    <code>Served in {{.RenderTime}}</code>
</div>
</body>
</html>
`))
)

type filterConfig struct {
	Metric string
	From   string
	To     string
}

type pageData struct {
	RenderTime time.Duration
	Filter     filterConfig
	CSV        string
}

func userMessages(ctx context.Context, c *statistics.Client, f *statistics.Filter, w io.Writer) error {
	messages, err := c.UserMessages(ctx, f)
	if err != nil {
		return err
	}

	csvWriter := csv.NewWriter(w)
	csvWriter.Write([]string{"date", "count"})
	for _, chat := range messages {
		csvWriter.Write([]string{chat.Date.Format("2006-01-02"), strconv.Itoa(chat.Count)})
	}
	csvWriter.Flush()

	return csvWriter.Error()
}

func chatSessions(ctx context.Context, c *statistics.Client, f *statistics.Filter, w io.Writer) error {
	chats, err := c.ChatSessions(ctx, f)
	if err != nil {
		return err
	}

	csvWriter := csv.NewWriter(w)
	csvWriter.Write([]string{"date", "count"})
	for _, chat := range chats {
		csvWriter.Write([]string{chat.Date.Format("2006-01-02"), strconv.Itoa(chat.Count)})
	}
	csvWriter.Flush()

	return csvWriter.Error()
}

func pages(ctx context.Context, c *statistics.Client, f *statistics.Filter, w io.Writer) error {
	pages, err := c.PageStatistics(ctx, f)
	if err != nil {
		return err
	}

	csvWriter := csv.NewWriter(w)
	csvWriter.Write([]string{"host", "path", "sessions", "messages"})
	for _, page := range pages {
		csvWriter.Write([]string{page.Host, page.Path, strconv.Itoa(page.Sessions), strconv.Itoa(page.Messages)})
	}
	csvWriter.Flush()

	return csvWriter.Error()
}

func feedback(ctx context.Context, c *statistics.Client, f *statistics.Filter, w io.Writer) error {
	feedback, err := c.AggregatedFeedback(ctx, f)
	if err != nil {
		return err
	}

	csvWriter := csv.NewWriter(w)
	csvWriter.Write([]string{"type", "rating", "count", "ratio"})
	for _, binaryRating := range feedback.Binary {
		csvWriter.Write([]string{"binary", strconv.Itoa(binaryRating.Rating), strconv.Itoa(binaryRating.Count), fmt.Sprintf("%.2f", binaryRating.Ratio)})
	}
	for _, emojiRating := range feedback.Emojis {
		csvWriter.Write([]string{"emoji", strconv.Itoa(emojiRating.Rating), strconv.Itoa(emojiRating.Count), fmt.Sprintf("%.2f", emojiRating.Ratio)})
	}
	csvWriter.Flush()

	return csvWriter.Error()
}

func labels(ctx context.Context, c *statistics.Client, f *statistics.Filter, w io.Writer) error {
	labels, err := c.ChatLabels(ctx, f)
	if err != nil {
		return err
	}

	csvWriter := csv.NewWriter(w)
	csvWriter.Write([]string{"id", "count", "text"})
	for _, label := range labels {
		csvWriter.Write([]string{label.ID, strconv.Itoa(label.Count), label.Text})
	}
	csvWriter.Flush()

	return csvWriter.Error()
}

func Handle(w http.ResponseWriter, r *http.Request) {
	begin := time.Now()

	if err := r.ParseForm(); err != nil {
		log.Println(err)
	}
	from := r.Form.Get("from")
	to := r.Form.Get("to")
	metric := r.Form.Get("metric")

	if metric == "" || from == "" || to == "" {
		if err := tmpl.Execute(w, pageData{
			Filter: filterConfig{},
			CSV:    "",
		}); err != nil {
			log.Println(err)
		}
		return
	}

	filter := filterConfig{
		Metric: metric,
		From:   from,
		To:     to,
	}

	fromDate, err := time.Parse("2006-01-02", from)
	if err != nil {
		http.Error(w, fmt.Sprintf("parsing from date: %v", err), http.StatusBadRequest)
		return
	}
	toDate, err := time.Parse("2006-01-02", to)
	if err != nil {
		http.Error(w, fmt.Sprintf("parsing to date: %v", err), http.StatusBadRequest)
		return
	}

	var csvBuf bytes.Buffer
	switch metric {
	case "chats":
		err := chatSessions(r.Context(), statsClient, &statistics.Filter{
			From: fromDate,
			To:   toDate,
		}, &csvBuf)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "messages":
		err := userMessages(r.Context(), statsClient, &statistics.Filter{
			From: fromDate,
			To:   toDate,
		}, &csvBuf)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "pages":
		err := pages(r.Context(), statsClient, &statistics.Filter{
			From: fromDate,
			To:   toDate,
		}, &csvBuf)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "feedback":
		err := feedback(r.Context(), statsClient, &statistics.Filter{
			From: fromDate,
			To:   toDate,
		}, &csvBuf)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "labels":
		err := labels(r.Context(), statsClient, &statistics.Filter{
			From: fromDate,
			To:   toDate,
		}, &csvBuf)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if err := tmpl.Execute(w, pageData{
		Filter:     filter,
		CSV:        csvBuf.String(),
		RenderTime: time.Since(begin),
	}); err != nil {
		log.Println(err)
	}
}
