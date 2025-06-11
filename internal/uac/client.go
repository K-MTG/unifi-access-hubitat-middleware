package uac

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Response Generic API response wrapper
type Response[T any] struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data"`
}

// Door represents a single door from UniFi Access
type Door struct {
	DoorLockRelayStatus string `json:"door_lock_relay_status"`
	DoorPositionStatus  string `json:"door_position_status"`
	FloorID             string `json:"floor_id"`
	FullName            string `json:"full_name"`
	ID                  string `json:"id"`
	IsBindHub           bool   `json:"is_bind_hub"`
	Name                string `json:"name"`
	Type                string `json:"type"`
}

// DoorLockRule represents the lock rule of a door
type DoorLockRule struct {
	Type      string  `json:"type"`
	EndedTime float64 `json:"ended_time"`
}

// Webhook represents a single webhook endpoint from UniFi Access
type Webhook struct {
	ID       *string           `json:"id,omitempty"`
	Endpoint string            `json:"endpoint"`
	Name     string            `json:"name"`
	Secret   *string           `json:"secret,omitempty"`
	Events   []string          `json:"events"`
	Headers  map[string]string `json:"headers,omitempty"`
}

type Client struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewClient(baseUrl string, apiKey string) *Client {
	return &Client{
		baseURL: baseUrl,
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

// doRequest handles HTTP requests with authentication and status validation.
func (c *Client) doRequest(method, path string, body io.Reader) (*http.Response, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, path)
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating %s request to %s failed: %w", method, url, err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s request to %s failed: %w", method, url, err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("%s request to %s had an unexpected status %d: %s", method, url, resp.StatusCode, respBody)
	}

	return resp, nil
}

func (c *Client) getRequest(path string) (*http.Response, error) {
	return c.doRequest(http.MethodGet, path, nil)
}

func (c *Client) putRequest(path string, body io.Reader) (*http.Response, error) {
	return c.doRequest(http.MethodPut, path, body)
}

func (c *Client) postRequest(path string, body io.Reader) (*http.Response, error) {
	return c.doRequest(http.MethodPost, path, body)
}

func (c *Client) deleteRequest(path string) (*http.Response, error) {
	return c.doRequest(http.MethodDelete, path, nil)
}

// FetchAllDoors retrieves all doors
func (c *Client) FetchAllDoors() ([]Door, error) {
	// permission key - view:space
	resp, err := c.getRequest("/api/v1/developer/doors")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp Response[[]Door]
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decoding response failed: %w", err)
	}

	if apiResp.Code != "SUCCESS" {
		return nil, fmt.Errorf("API error: %s", apiResp.Msg)
	}

	return apiResp.Data, nil
}

// FetchDoor retrieves a specific door by ID
func (c *Client) FetchDoor(doorID string) (*Door, error) {
	// permission key - view:space
	resp, err := c.getRequest(fmt.Sprintf("/api/v1/developer/doors/%s", doorID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp Response[Door]
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decoding response failed: %w", err)
	}

	if apiResp.Code != "SUCCESS" {
		return nil, fmt.Errorf("API error: %s", apiResp.Msg)
	}

	return &apiResp.Data, nil
}

// AssertToggleDoorUnlock toggles the lock state of a door
func (c *Client) AssertToggleDoorUnlock(doorID string) error {
	door, err := c.FetchDoor(doorID)
	if err != nil {
		return fmt.Errorf("failed to fetch door: %w", err)
	}
	if door.DoorLockRelayStatus == "unlock" {
		// Already unlocked, skip
		return nil
	}

	// permission key - edit:space
	url := fmt.Sprintf("/api/v1/developer/doors/%s/unlock", doorID)

	resp, err := c.putRequest(url, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var apiResp Response[any]
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return fmt.Errorf("decoding response failed: %w", err)
	}

	if apiResp.Code != "SUCCESS" {
		return fmt.Errorf("API error: %s", apiResp.Msg)
	}

	return nil
}

// setDoorLockRule updates the lock rule of a door
func (c *Client) setDoorLockRule(doorID, ruleType string) error {
	url := fmt.Sprintf("/api/v1/developer/doors/%s/lock_rule", doorID)

	body, err := json.Marshal(map[string]string{"type": ruleType})
	if err != nil {
		return fmt.Errorf("marshaling request body failed: %w", err)
	}

	resp, err := c.putRequest(url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var apiResp Response[any]
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return fmt.Errorf("decoding response failed: %w", err)
	}

	if apiResp.Code != "SUCCESS" {
		return fmt.Errorf("API error: %s", apiResp.Msg)
	}

	return nil
}

// GetDoorLockRule retrieves the lock rule of a door
func (c *Client) GetDoorLockRule(doorID string) (*DoorLockRule, error) {
	url := fmt.Sprintf("/api/v1/developer/doors/%s/lock_rule", doorID)

	resp, err := c.getRequest(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp Response[DoorLockRule]
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decoding response failed: %w", err)
	}

	if apiResp.Code != "SUCCESS" {
		return nil, fmt.Errorf("API error: %s", apiResp.Msg)
	}

	return &apiResp.Data, nil
}

// AssertUnlockDoor sets the lock rule of a door to keep it unlocked, if not already unlocked
func (c *Client) AssertUnlockDoor(doorID string) error {
	rule, err := c.GetDoorLockRule(doorID)
	if err != nil {
		return err
	}
	if rule.Type == "keep_unlock" {
		// Already unlocked, skip
		return nil
	}
	return c.setDoorLockRule(doorID, "keep_unlock")
}

// AssertLockDoor sets the lock rule of a door to default (reset)
func (c *Client) AssertLockDoor(doorID string) error {
	rule, err := c.GetDoorLockRule(doorID)
	if err != nil {
		return err
	}
	if rule.Type == "" {
		// Already locked, skip
		return nil
	}
	return c.setDoorLockRule(doorID, "reset")
}

// FetchWebhookEndpoints retrieves webhook endpoints
func (c *Client) FetchWebhookEndpoints() ([]Webhook, error) {
	// permission key - view:webhook
	resp, err := c.getRequest("/api/v1/developer/webhooks/endpoints")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp Response[[]Webhook]
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decoding response failed: %w", err)
	}
	if apiResp.Code != "SUCCESS" {
		return nil, fmt.Errorf("API error: %s", apiResp.Msg)
	}
	return apiResp.Data, nil
}

// AddWebhookEndpoint creates a new webhook endpoint
func (c *Client) AddWebhookEndpoint(webhook *Webhook) (*Webhook, error) {
	// permission key - edit:webhook
	body, err := json.Marshal(webhook)
	if err != nil {
		return nil, fmt.Errorf("marshaling request body failed: %w", err)
	}

	resp, err := c.postRequest("/api/v1/developer/webhooks/endpoints", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp Response[Webhook]
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decoding response failed: %w", err)
	}
	if apiResp.Code != "SUCCESS" {
		return nil, fmt.Errorf("API error: %s", apiResp.Msg)
	}
	return &apiResp.Data, nil
}

// UpdateWebhookEndpoint updates an existing webhook endpoint by ID
func (c *Client) UpdateWebhookEndpoint(id string, webhook *Webhook) (*Webhook, error) {
	url := fmt.Sprintf("/api/v1/developer/webhooks/endpoints/%s", id)
	body, err := json.Marshal(webhook)
	if err != nil {
		return nil, fmt.Errorf("marshaling request body failed: %w", err)
	}
	resp, err := c.putRequest(url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp Response[Webhook]
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decoding response failed: %w", err)
	}
	if apiResp.Code != "SUCCESS" {
		return nil, fmt.Errorf("API error: %s", apiResp.Msg)
	}
	return &apiResp.Data, nil
}

// DeleteWebhookEndpoint deletes a webhook endpoint by ID
func (c *Client) DeleteWebhookEndpoint(id string) error {
	url := fmt.Sprintf("/api/v1/developer/webhooks/endpoints/%s", id)
	resp, err := c.deleteRequest(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var apiResp Response[any]
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return fmt.Errorf("decoding response failed: %w", err)
	}
	if apiResp.Code != "SUCCESS" {
		return fmt.Errorf("API error: %s", apiResp.Msg)
	}
	return nil
}
