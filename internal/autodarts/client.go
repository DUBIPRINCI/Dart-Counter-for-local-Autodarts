package autodarts

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(host string, port int) *Client {
	return &Client{
		baseURL: fmt.Sprintf("http://%s:%d", host, port),
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
		},
	}
}

func (c *Client) GetState() (*BoardState, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/api/state")
	if err != nil {
		return nil, fmt.Errorf("request board state: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("board manager returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var state BoardState
	if err := json.Unmarshal(body, &state); err != nil {
		return nil, fmt.Errorf("parse board state: %w", err)
	}

	return &state, nil
}

func (c *Client) IsConnected() bool {
	resp, err := c.httpClient.Get(c.baseURL + "/api/version")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (c *Client) BaseURL() string {
	return c.baseURL
}
