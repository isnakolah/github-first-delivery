// Package github provides small authenticated REST primitives. It never stores tokens.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

const APIURL = "https://api.github.com"

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func NewClient() *Client {
	return &Client{BaseURL: APIURL, Token: authToken(), HTTP: http.DefaultClient}
}

// authToken uses an explicitly supplied token first. For normal developer use,
// it falls back to the active gh CLI credential without persisting or printing
// the token. GitHub Actions supplies GITHUB_TOKEN from GFD_WRITER_TOKEN.
func authToken() string {
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		return token
	}
	output, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func (c *Client) Do(ctx context.Context, method, path string, input, output any) error {
	base := strings.TrimRight(c.BaseURL, "/")
	u, err := url.Parse(base + path)
	if err != nil {
		return err
	}
	var body io.Reader
	if input != nil {
		raw, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GitHub API %s %s: %s", method, path, strings.TrimSpace(string(data)))
	}
	if output != nil && len(data) != 0 {
		return json.Unmarshal(data, output)
	}
	return nil
}

type Comment struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
	User struct {
		Login string `json:"login"`
	} `json:"user"`
	UpdatedAt string `json:"updated_at"`
	CreatedAt string `json:"created_at"`
}

func (c *Client) CreateComment(ctx context.Context, owner, repo string, number int, body string) (Comment, error) {
	var result Comment
	err := c.Do(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, number), map[string]string{"body": body}, &result)
	return result, err
}

func (c *Client) ListComments(ctx context.Context, owner, repo string, number int) ([]Comment, error) {
	var result []Comment
	err := c.Do(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s/issues/%d/comments?per_page=100", owner, repo, number), nil, &result)
	return result, err
}
