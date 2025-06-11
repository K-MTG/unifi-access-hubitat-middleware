package hubitat

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type DeviceInfo struct {
	ID           string           `json:"id"`
	Type         string           `json:"type"`
	Attributes   []map[string]any `json:"attributes"`
	Capabilities []any            `json:"capabilities"`
	Commands     []string         `json:"commands"`
}

type Client struct {
	baseURL     string
	accessToken string
	client      *http.Client
}

func NewClient(baseUrl string, accessToken string) *Client {
	return &Client{
		baseURL:     baseUrl,
		accessToken: accessToken,
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

// hasCapability checks if the device has a given capability.
func hasCapability(deviceInfo *DeviceInfo, capabilityName string) bool {
	for _, capability := range deviceInfo.Capabilities {
		if capability == capabilityName {
			return true
		}
	}
	return false
}

func hasCommand(deviceInfo *DeviceInfo, commandName string) bool {
	for _, command := range deviceInfo.Commands {
		if command == commandName {
			return true
		}
	}
	return false
}

// GetDeviceInfo fetches information about a specific Hubitat device.
func (c *Client) GetDeviceInfo(deviceID string) (*DeviceInfo, error) {
	url := fmt.Sprintf("%s/devices/%s?access_token=%s", c.baseURL, deviceID, c.accessToken)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	var info DeviceInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

// sendDeviceCommand sends a command to a Hubitat device.
// deviceID: the device ID as a string
// command: the command to send (e.g., "on", "off", "lock", "unlock")
// secondaryValue: optional secondary value (can be empty string if not needed)
func (c *Client) sendDeviceCommand(deviceID, command, secondaryValue string) error {
	url := c.baseURL + "/devices/" + deviceID + "/" + command
	if secondaryValue != "" {
		url += "/" + secondaryValue
	}
	url += "?access_token=" + c.accessToken

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}

	if resp, err := c.client.Do(req); err != nil {
		return err
	} else if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	} else {
		return nil
	}
}

// assertDeviceState checks if a device has a capability, command, and attribute value, and sends a command if needed.
func (c *Client) assertDeviceState(deviceID, capability, command, attributeName, desiredValue string) error {
	deviceInfo, err := c.GetDeviceInfo(deviceID)
	if err != nil {
		return fmt.Errorf("failed to get device info for device %s: %w", deviceID, err)
	}

	if !hasCapability(deviceInfo, capability) {
		return fmt.Errorf("device %s does not have %s capability", deviceID, capability)
	}

	if !hasCommand(deviceInfo, command) {
		return fmt.Errorf("device %s does not support %s command", deviceID, command)
	}

	for _, attr := range deviceInfo.Attributes {
		if attr["name"] == attributeName && attr["currentValue"] == desiredValue {
			return nil // Already in desired state
		}
	}

	if err := c.sendDeviceCommand(deviceID, command, ""); err != nil {
		return fmt.Errorf("failed to send %s command to device %s: %w", command, deviceID, err)
	}

	return nil
}

func (c *Client) AssertDoorContactOpened(doorID string) error {
	return c.assertDeviceState(doorID, "ContactSensor", "open", "contact", "open")
}

func (c *Client) AssertDoorContactClosed(doorID string) error {
	return c.assertDeviceState(doorID, "ContactSensor", "close", "contact", "close")
}

func (c *Client) AssertDoorLockUnlocked(doorID string) error {
	return c.assertDeviceState(doorID, "Lock", "unlock", "lock", "unlocked")
}

func (c *Client) AssertDoorLockLocked(doorID string) error {
	return c.assertDeviceState(doorID, "Lock", "lock", "lock", "locked")
}

func (c *Client) AssertDoorSwitchOn(doorID string) error {
	return c.assertDeviceState(doorID, "Switch", "on", "switch", "on")
}

func (c *Client) AssertDoorSwitchOff(doorID string) error {
	return c.assertDeviceState(doorID, "Switch", "off", "switch", "off")
}
