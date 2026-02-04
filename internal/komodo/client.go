package komodo

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL   string
	apiKey    string
	apiSecret string
	http      *http.Client
}

type ClientOptions struct {
	Timeout            time.Duration
	InsecureSkipVerify bool // set true only if you have self-signed TLS
}

func NewClient(baseURL, apiKey, apiSecret string, opt ClientOptions) *Client {
	timeout := opt.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}

	tr := http.DefaultTransport.(*http.Transport).Clone()
	if opt.InsecureSkipVerify {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} 
	}

	return &Client{
		baseURL:   baseURL,
		apiKey:    apiKey,
		apiSecret: apiSecret,
		http: &http.Client{
			Timeout:   timeout,
			Transport: tr,
		},
	}
}

func (c *Client) doRead(ctx context.Context, req ReadRequest, out any) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/read", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Api-Key", c.apiKey)
	httpReq.Header.Set("X-Api-Secret", c.apiSecret)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("komodo http %d: %s", resp.StatusCode, string(respBytes))
	}

	if err := json.Unmarshal(respBytes, out); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	return nil
}

func (c *Client) ListServers(ctx context.Context) ([]ListServersResponseItem, error) {
	var servers []ListServersResponseItem
	err := c.doRead(ctx, ReadRequest{
		Type:   "ListServers",
		Params: ListServersParams{},
	}, &servers)
	return servers, err
}

func (c *Client) GetSystemStats(ctx context.Context, serverID string) (SystemStats, error) {
	var stats SystemStats
	err := c.doRead(ctx, ReadRequest{
		Type: "GetSystemStats",
		Params: GetSystemStatsParams{
			Server: serverID,
		},
	}, &stats)
	return stats, err
}
