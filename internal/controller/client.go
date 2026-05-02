package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const (
	defaultRetryMax  = 2
	defaultRetryWait = 200 * time.Millisecond
	maxErrorBodySize = 4096
)

type Client struct {
	baseURL    *url.URL
	token      string
	httpClient *http.Client
	logger     *slog.Logger
	retryMax   int
	retryWait  time.Duration
}

type ClientOption func(*Client)

func NewClient(baseURL string, token string, timeout time.Duration, options ...ClientOption) (*Client, error) {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("parse controller base URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("controller base URL must be absolute")
	}
	if token == "" {
		return nil, fmt.Errorf("controller token is required")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("controller timeout must be greater than 0")
	}

	client := &Client{
		baseURL: parsed,
		token:   token,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger:    slog.Default(),
		retryMax:  defaultRetryMax,
		retryWait: defaultRetryWait,
	}

	for _, option := range options {
		option(client)
	}

	return client, nil
}

func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(client *Client) {
		if httpClient != nil {
			client.httpClient = httpClient
		}
	}
}

func WithLogger(logger *slog.Logger) ClientOption {
	return func(client *Client) {
		if logger != nil {
			client.logger = logger
		}
	}
}

func WithRetry(retryMax int, retryWait time.Duration) ClientOption {
	return func(client *Client) {
		if retryMax >= 0 {
			client.retryMax = retryMax
		}
		if retryWait >= 0 {
			client.retryWait = retryWait
		}
	}
}

func (client *Client) Register(ctx context.Context, request RegisterRequest) (RegisterResponse, error) {
	var response RegisterResponse
	err := client.doJSON(ctx, http.MethodPost, "/api/relays/register", request, &response)
	return response, err
}

func (client *Client) Heartbeat(ctx context.Context, nodeID string, request HeartbeatRequest) (HeartbeatResponse, error) {
	var response HeartbeatResponse
	endpoint := fmt.Sprintf("/api/relays/%s/heartbeat", url.PathEscape(nodeID))
	err := client.doJSON(ctx, http.MethodPost, endpoint, request, &response)
	return response, err
}

func (client *Client) GetConfig(ctx context.Context, nodeID string) (RelayConfig, error) {
	var response RelayConfig
	endpoint := fmt.Sprintf("/api/relays/%s/config", url.PathEscape(nodeID))
	err := client.doJSON(ctx, http.MethodGet, endpoint, nil, &response)
	return response, err
}

func (client *Client) ReportApplyResult(ctx context.Context, nodeID string, request ApplyResultRequest) (ApplyResultResponse, error) {
	var response ApplyResultResponse
	endpoint := fmt.Sprintf("/api/relays/%s/config-apply-result", url.PathEscape(nodeID))
	err := client.doJSON(ctx, http.MethodPost, endpoint, request, &response)
	return response, err
}

func (client *Client) doJSON(ctx context.Context, method string, endpoint string, requestBody any, responseBody any) error {
	var encoded []byte
	if requestBody != nil {
		var err error
		encoded, err = json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("encode controller request: %w", err)
		}
	}

	var lastErr error
	for attempt := 0; attempt <= client.retryMax; attempt++ {
		if attempt > 0 && client.retryWait > 0 {
			timer := time.NewTimer(client.retryWait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}

		err := client.doJSONOnce(ctx, method, endpoint, encoded, responseBody)
		if err == nil {
			return nil
		}
		lastErr = err

		if !shouldRetry(err) || attempt == client.retryMax {
			return err
		}

		client.logger.Warn(
			"controller request failed, retrying",
			"method", method,
			"endpoint", endpoint,
			"attempt", attempt+1,
			"maxAttempts", client.retryMax+1,
			"error", err,
		)
	}

	return lastErr
}

func (client *Client) doJSONOnce(ctx context.Context, method string, endpoint string, encoded []byte, responseBody any) error {
	requestURL := client.resolve(endpoint)

	var body io.Reader
	if encoded != nil {
		body = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return fmt.Errorf("create controller request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+client.token)
	if encoded != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send controller request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySize))
		return &APIError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(bodyBytes)),
		}
	}

	if responseBody == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}

	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(responseBody); err != nil {
		return fmt.Errorf("decode controller response: %w", err)
	}

	return nil
}

func (client *Client) resolve(endpoint string) string {
	next := *client.baseURL
	next.Path = path.Join(client.baseURL.Path, endpoint)
	return next.String()
}

func shouldRetry(err error) bool {
	apiErr, ok := err.(*APIError)
	if !ok {
		return true
	}
	return apiErr.StatusCode == http.StatusTooManyRequests || apiErr.StatusCode >= http.StatusInternalServerError
}
