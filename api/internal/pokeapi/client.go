package pokeapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrUpstream marks any failure caused by PokeAPI rather than the caller.
var ErrUpstream = errors.New("pokeapi upstream error")

const maxResponseBytes = 8 << 20 // 8 MiB

type Client struct {
	endpoint string
	http     *http.Client
	maxBytes int64
}

func NewClient(endpoint string) *Client {
	return &Client{
		endpoint: endpoint,
		http:     &http.Client{Timeout: 10 * time.Second},
		maxBytes: maxResponseBytes,
	}
}

type gqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type gqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (c *Client) Do(ctx context.Context, query string, vars map[string]any, out any) error {
	// Marshal gqlRequest and build a POST with NewRequestWithContext
	reqBody, err := json.Marshal(gqlRequest{
		Query:     query,
		Variables: vars,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}

	// Set Content-Type: application/json
	req.Header.Set("Content-Type", "application/json")

	// Send it; wrap transport errors with ErrUpstream
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer resp.Body.Close()

	// Non-2xx status -> ErrUpstream
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: status %d", ErrUpstream, resp.StatusCode)
	}

	// Read the body through io.LimitReader; too big -> ErrUpstream
	body, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBytes+1))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	if int64(len(body)) > c.maxBytes {
		return fmt.Errorf("%w: response too large", ErrUpstream)
	}

	// Unmarshal into gqlResponse; a non-empty Errors slice -> ErrUpstream
	var respData gqlResponse
	if err := json.Unmarshal(body, &respData); err != nil {
		return fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	if len(respData.Errors) > 0 {
		return fmt.Errorf("%w: %v", ErrUpstream, respData.Errors[0].Message)
	}
	if len(respData.Data) == 0 || bytes.Equal(respData.Data, []byte("null")) {
		return fmt.Errorf("%w: no data in response", ErrUpstream)
	}

	// Unmarshal resp.Data into out
	if err := json.Unmarshal(respData.Data, out); err != nil {
		return fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	return nil
}
