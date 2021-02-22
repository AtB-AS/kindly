package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

const (
	tokenURLBase = "https://api.kindly.ai/api/v2/bot"
)

type TokenSource struct {
	APIKey   string
	BotID    string
	TokenURL string
}

var (
	ErrRetrieveToken = fmt.Errorf("failed to fetch token")
)

func (t *TokenSource) Token() (*oauth2.Token, error) {
	if t.TokenURL == "" {
		t.TokenURL = fmt.Sprintf("%s/%s/sage/auth", tokenURLBase, t.BotID)
	}

	req, err := http.NewRequest(http.MethodGet, t.TokenURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+t.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("%w: unauthorized", ErrRetrieveToken)
	case http.StatusOK:
		ct := resp.Header.Get("Content-type")
		if !strings.HasPrefix(ct, "application/json") {
			return nil, fmt.Errorf("%w: unexpected content-type: %s", ErrRetrieveToken, ct)
		}
		body, err := ioutil.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return nil, err
		}

		return extractToken(bytes.NewReader(body))
	default:
		return nil, fmt.Errorf("%w", ErrRetrieveToken)
	}
}

type tokenJSON struct {
	AccessToken string
	Expires     time.Time
}

func (t *tokenJSON) UnmarshalJSON(data []byte) error {
	type raw struct {
		JWT string `json:"jwt"`
		TTL int    `json:"ttl"`
	}
	r := raw{}
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}

	t.AccessToken = r.JWT
	t.Expires = time.Now().Add(time.Duration(r.TTL) * time.Second)

	return nil
}

func extractToken(r io.Reader) (*oauth2.Token, error) {
	var t tokenJSON
	if err := json.NewDecoder(r).Decode(&t); err != nil {
		return nil, err
	}

	return &oauth2.Token{
		AccessToken:  t.AccessToken,
		TokenType:    "Bearer",
		RefreshToken: "",
		Expiry:       t.Expires,
	}, nil
}
