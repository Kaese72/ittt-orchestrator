package devicestore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Attribute mirrors the attribute shape returned by the device-store REST API.
type Attribute struct {
	Name    string   `json:"name"`
	Boolean *bool    `json:"boolean-state,omitempty"`
	Numeric *float32 `json:"numeric-state,omitempty"`
	Text    *string  `json:"string-state,omitempty"`
}

// Device is the subset of the device-store Device model that the orchestrator needs.
type Device struct {
	ID         int         `json:"id"`
	Attributes []Attribute `json:"attributes"`
}

// Client is an HTTP client for the device-store service.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

func (c *Client) GetDevice(id int) (Device, error) {
	resp, err := c.httpClient.Get(fmt.Sprintf("%s/device-store/v0/devices/%d", c.baseURL, id))
	if err != nil {
		return Device{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return Device{}, fmt.Errorf("device-store returned %d: %s", resp.StatusCode, body)
	}
	var device Device
	if err := json.NewDecoder(resp.Body).Decode(&device); err != nil {
		return Device{}, err
	}
	return device, nil
}

func (c *Client) TriggerDeviceCapability(id int, capability string, args map[string]any) error {
	return c.triggerCapability(fmt.Sprintf("%s/device-store/v0/devices/%d/capabilities/%s", c.baseURL, id, capability), args)
}

func (c *Client) TriggerGroupCapability(id int, capability string, args map[string]any) error {
	return c.triggerCapability(fmt.Sprintf("%s/device-store/v0/groups/%d/capabilities/%s", c.baseURL, id, capability), args)
}

func (c *Client) triggerCapability(url string, args map[string]any) error {
	if args == nil {
		args = map[string]any{}
	}
	body, err := json.Marshal(args)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("device-store returned %d: %s", resp.StatusCode, respBody)
	}
	return nil
}
