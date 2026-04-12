package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"code-execution/internal/config"
)

type Client struct {
	cfg        config.EurekaConfig
	httpClient *http.Client
}

func NewClient(cfg config.EurekaConfig) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) Register(ctx context.Context) error {
	if !c.cfg.Enabled {
		return nil
	}

	payload := map[string]any{
		"instance": map[string]any{
			"instanceId": c.cfg.InstanceID,
			"hostName":   c.cfg.IPAddr,
			"app":        strings.ToUpper(c.cfg.ServiceName),
			"ipAddr":     c.cfg.IPAddr,
			"status":     "UP",
			"port": map[string]any{
				"$":        getPort(c.cfg.InstanceID),
				"@enabled": "true",
			},
			"vipAddress":     strings.ToLower(c.cfg.ServiceName),
			"secureVipAddress": strings.ToLower(c.cfg.ServiceName),
			"dataCenterInfo": map[string]any{
				"@class": "com.netflix.appinfo.InstanceInfo$DefaultDataCenterInfo",
				"name":   "MyOwn",
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("eureka payload build failed: %w", err)
	}

	registerURL := fmt.Sprintf("%s/apps/%s", c.baseURL(), strings.ToUpper(c.cfg.ServiceName))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, registerURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("eureka register request creation failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.applyAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("eureka register call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusCreated {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("eureka register failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	return nil
}

func (c *Client) SendHeartbeat(ctx context.Context) error {
	if !c.cfg.Enabled {
		return nil
	}

	heartbeatURL := fmt.Sprintf("%s/apps/%s/%s", c.baseURL(), strings.ToUpper(c.cfg.ServiceName), url.PathEscape(c.cfg.InstanceID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, heartbeatURL, nil)
	if err != nil {
		return fmt.Errorf("eureka heartbeat request creation failed: %w", err)
	}
	c.applyAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("eureka heartbeat call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("eureka heartbeat failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	return nil
}

func (c *Client) StartHeartbeat(ctx context.Context, onError func(error)) {
	if !c.cfg.Enabled {
		return
	}

	ticker := time.NewTicker(c.cfg.HeartbeatInterval)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				heartbeatCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				err := c.SendHeartbeat(heartbeatCtx)
				cancel()
				if err != nil && onError != nil {
					onError(err)
				}
			}
		}
	}()
}

func (c *Client) Deregister(ctx context.Context) error {
	if !c.cfg.Enabled {
		return nil
	}

	deregisterURL := fmt.Sprintf("%s/apps/%s/%s", c.baseURL(), strings.ToUpper(c.cfg.ServiceName), url.PathEscape(c.cfg.InstanceID))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, deregisterURL, nil)
	if err != nil {
		return fmt.Errorf("eureka deregister request creation failed: %w", err)
	}
	c.applyAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("eureka deregister call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("eureka deregister failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	return nil
}

func (c *Client) applyAuth(req *http.Request) {
	if c.cfg.Username == "" {
		return
	}
	req.SetBasicAuth(c.cfg.Username, c.cfg.Password)
}

func (c *Client) baseURL() string {
	base := strings.TrimRight(c.cfg.BaseURL, "/")
	if strings.HasSuffix(strings.ToLower(base), "/eureka") {
		return base
	}
	return base + "/eureka"
}

func getPort(instanceID string) int {
	parts := strings.Split(instanceID, ":")
	if len(parts) == 0 {
		return 8080
	}
	last := parts[len(parts)-1]
	port, err := strconv.Atoi(last)
	if err != nil {
		return 8080
	}
	return port
}
