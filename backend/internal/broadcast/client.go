// Package broadcast is the authenticated control plane for the private media
// worker. It never forwards media bytes through the game backend.
package broadcast

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Controller interface {
	Prepare(context.Context, string, string, string) error
	Start(context.Context, string, string, string) error
	Stop(context.Context, string, string, string) error
}

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{Timeout: 20 * time.Second}}
}

func (c *Client) Prepare(ctx context.Context, gameID, trackID, token string) error {
	return c.call(ctx, trackID, "prepare", token)
}
func (c *Client) Start(ctx context.Context, gameID, trackID, token string) error {
	return c.call(ctx, trackID, "start", token)
}
func (c *Client) Stop(ctx context.Context, gameID, trackID, token string) error {
	return c.call(ctx, trackID, "stop", token)
}

func (c *Client) call(ctx context.Context, trackID, action, token string) error {
	endpoint := c.baseURL + "/broadcast/" + url.PathEscape(trackID) + "/" + action + "?token=" + url.QueryEscape(token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("media broadcaster returned %s", resp.Status)
	}
	return nil
}
