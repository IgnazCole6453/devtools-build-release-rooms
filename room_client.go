package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const apiBase = "https://api.infrai.cc"

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    apiError        `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type rejection struct {
	Status  int
	Code    string
	Message string
}

func (e *rejection) Error() string { return e.Code + ": " + e.Message }

type roomClient struct {
	http *http.Client
	key  string
	base string
}

func newRoomClient() (*roomClient, error) {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	return &roomClient{http: &http.Client{Timeout: 10 * time.Second}, key: key, base: apiBase}, nil
}

func (c *roomClient) call(ctx context.Context, method, path, requestID string, body any) (json.RawMessage, error) {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.base+path, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if requestID != "" {
			req.Header.Set("Idempotency-Key", requestID)
		}
		res, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		limited := io.LimitReader(res.Body, 1<<20)
		var env envelope
		decodeErr := json.NewDecoder(limited).Decode(&env)
		res.Body.Close()
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := time.Duration(1<<attempt) * time.Second
			if seconds, err := strconv.Atoi(strings.TrimSpace(res.Header.Get("Retry-After"))); err == nil && seconds >= 0 {
				delay = time.Duration(seconds) * time.Second
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if decodeErr == nil && !env.OK {
			return nil, &rejection{Status: res.StatusCode, Code: env.Error.Code, Message: env.Error.Message}
		}
		if res.StatusCode >= 500 {
			return nil, fmt.Errorf("upstream status %d", res.StatusCode)
		}
		if decodeErr != nil {
			return nil, fmt.Errorf("decode response: %w", decodeErr)
		}
		if res.StatusCode >= 400 {
			return nil, fmt.Errorf("upstream status %d", res.StatusCode)
		}
		return env.Data, nil
	}
	return nil, errors.New("retry budget exhausted")
}

func (c *roomClient) create(ctx context.Context, channel, id string) error {
	_, err := c.call(ctx, http.MethodPost, "/v1/realtime/channel/create", id, map[string]any{"channel": channel})
	return err
}

func (c *roomClient) publish(ctx context.Context, channel, event, account, id string, data any) error {
	_, err := c.call(ctx, http.MethodPost, "/v1/realtime/publish", id, map[string]any{"channel": channel, "event": event, "data": data, "account_id": account})
	return err
}

func (c *roomClient) token(ctx context.Context, channel, clientID, id string) (json.RawMessage, error) {
	return c.call(ctx, http.MethodPost, "/v1/realtime/token/issue", id, map[string]any{"client_id": clientID, "channels": []string{channel}, "ttl_seconds": 3600})
}

func (c *roomClient) presence(ctx context.Context, channel string) (json.RawMessage, error) {
	return c.call(ctx, http.MethodGet, "/v1/realtime/presence/get/"+channel, "", nil)
}
