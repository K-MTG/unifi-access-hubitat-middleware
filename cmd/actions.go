package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/K-MTG/unifi-access-hubitat-middleware/internal/hubitat"
	"github.com/K-MTG/unifi-access-hubitat-middleware/internal/uac"
	"github.com/K-MTG/unifi-access-hubitat-middleware/pkg/utils"
)

func assertUacWebhookExists() (*uac.Webhook, error) {
	// Check if the webhook exists
	webhooks, err := uacClient.FetchWebhookEndpoints()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch uac webhook endpoints: %w", err)
	}

	newWebhook := uac.Webhook{
		Name:     "unifi-access-hubitat-middleware",
		Endpoint: fmt.Sprintf("%s/webhook/uac", appConfig.Server.BaseURL),
		Events:   []string{"access.device.dps_status", "access.door.unlock"}, // todo "access.temporary_unlock.start", "access.temporary_unlock.end"},
		Headers: map[string]string{
			"Authorization": appConfig.Server.AuthToken,
		},
	}

	for _, webhook := range webhooks {
		if webhook.Name == newWebhook.Name {
			// Check if fields match
			if webhook.Endpoint != newWebhook.Endpoint || !utils.StringSlicesEqual(webhook.Events, newWebhook.Events) ||
				!utils.StringMapsEqual(webhook.Headers, newWebhook.Headers) {
				logger.Info("UAC webhook exists but fields differ, updating", slog.String("webhook_id", *webhook.ID))
				updated, err := uacClient.UpdateWebhookEndpoint(*webhook.ID, &newWebhook)
				if err != nil {
					return nil, fmt.Errorf("failed to update UAC webhook endpoint: %w", err)
				}
				return updated, nil
			}
			logger.Info("UAC Webhook already exists and matches configuration", slog.String("webhook_id", *webhook.ID))
			return &webhook, nil // Webhook already exists and matches
		}
	}

	// Create the webhook if it doesn't exist
	logger.Info("UAC webhook does not exist, creating new webhook")
	createdWebhook, err := uacClient.AddWebhookEndpoint(&newWebhook)
	if err != nil {
		return nil, fmt.Errorf("failed to create UAC webhook endpoint: %w", err)
	}

	return createdWebhook, nil
}

func handleUacEvent(evt uac.WebhookEvent) {
	logger.Info("Received UAC Event", slog.Any("event", evt))

	switch evt.Event {
	case "access.door.unlock":
		var payload struct {
			Location struct {
				ID string `json:"id"`
			} `json:"location"`
			Actor struct {
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"actor"`
			Object struct {
				Result string `json:"result"`
			} `json:"object"`
		}
		if err := json.Unmarshal(evt.Data, &payload); err != nil {
			logger.Error("Failed to unmarshal event data", slog.String("err", err.Error()))
			return
		}
		if payload.Actor.Type == "open-api" && payload.Actor.Name == "unifi-access-hubitat-middleware" {
			logger.Info("Door unlock event triggered by API, ignoring", slog.Any("event", evt))
			return
		}
		if payload.Object.Result != "Access Granted" {
			logger.Info("Door unlock event not granted, ignoring", slog.Any("event", evt))
			return
		}

		door, found := getDoorByUacID(payload.Location.ID)
		if !found {
			logger.Warn("Door not found for UAC ID", slog.Any("event", evt))
			return
		}

		// sleep for 200 milliseconds to allow the door lock to actually unlock
		time.Sleep(200 * time.Millisecond)
		err := hubitatClient.AssertDoorSwitchOn(door.HubitatSwitchID)
		if err != nil {
			logger.Error("Failed to assert door switch on in Hubitat",
				slog.Any("event", evt),
				slog.String("err", err.Error()),
				slog.String("hubitat_switch_id", door.HubitatSwitchID))
			return
		}
	case "access.device.dps_status":
		var payload struct {
			Location struct {
				ID string `json:"id"`
			} `json:"location"`
			Object struct {
				EventType string `json:"event_type"`
				Status    string `json:"status"`
			} `json:"object"`
		}
		if err := json.Unmarshal(evt.Data, &payload); err != nil {
			logger.Error("Failed to unmarshal event data", slog.String("err", err.Error()))
			return
		}
		if payload.Object.EventType != "dps_change" {
			logger.Error("Device event type is not dps_change, ignoring", slog.Any("event", evt))
			return
		}

		door, found := getDoorByUacID(payload.Location.ID)
		if !found {
			logger.Warn("Door not found for UAC ID", slog.Any("event", evt))
			return
		}

		var err error
		if payload.Object.Status == "open" {
			err = hubitatClient.AssertDoorContactOpened(door.HubitatContactID)
		} else if payload.Object.Status == "close" {
			err = hubitatClient.AssertDoorContactClosed(door.HubitatContactID)
		} else {
			logger.Error("Unknown door status", slog.Any("event", evt))
			return
		}

		if err != nil {
			logger.Error("Failed to assert door status in hubitat",
				slog.Any("event", evt),
				slog.String("err", err.Error()),
				slog.String("hubitat_contact_id", door.HubitatContactID))
			return
		}
	// todo implement temporary unlock events
	//case "access.temporary_unlock.start":
	//case "access.temporary_unlock.end":

	default:
		logger.Error("Unknown Uac event", slog.Any("event", evt))
	}
}

func handleHubitatEvent(evt hubitat.WebhookEvent) {
	logger.Info("Received Hubitat Event", slog.Any("event", evt))

	door, deviceType, found := getDoorByHubitatID(evt.Content.DeviceID)
	if !found {
		logger.Error("Door not found for Hubitat ID", slog.Any("event", evt))
		return
	}

	var err error

	switch deviceType {
	case "switch":
		if evt.Content.Value == "on" {
			err = uacClient.AssertToggleDoorUnlock(door.UacID)
		}
	case "lock":
		if evt.Content.Value == "unlocked" {
			err = uacClient.AssertUnlockDoor(door.UacID)
		} else if evt.Content.Value == "locked" {
			err = uacClient.AssertLockDoor(door.UacID)
		} else {
			logger.Error("Unknown lock value", slog.Any("event", evt))
			return
		}
	case "contact":
		// no action needed for contact sensor events
	default:
		logger.Warn("Unknown Hubitat event", slog.Any("event", evt))
	}

	if err != nil {
		logger.Error("Failed to execute Hubitat event action",
			slog.Any("event", evt),
			slog.String("err", err.Error()),
			slog.Any("door", door))
	}
}

func pollUacStates(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	// set door contact position at startup
	doors, err := uacClient.FetchAllDoors()
	if err != nil {
		logger.Error("Failed to fetch all doors", slog.String("err", err.Error()))
	} else {
		for _, d := range doors {

			door, found := getDoorByUacID(d.ID)
			if !found {
				logger.Warn("Door not found for UAC ID", slog.String("door_id", d.ID))
				return
			}

			switch d.DoorPositionStatus {
			case "open":
				if err := hubitatClient.AssertDoorContactOpened(door.HubitatContactID); err != nil {
					logger.Error("Failed to assert door contact opened",
						slog.String("door_id", d.ID), slog.String("err", err.Error()))
				}
			case "close":
				if err := hubitatClient.AssertDoorContactClosed(door.HubitatContactID); err != nil {
					logger.Error("Failed to assert door contact closed",
						slog.String("door_id", d.ID), slog.String("err", err.Error()))
				}
			}
		}
	}

	// poll door rule every 5 seconds and update hubitat lock when status changes.
	// This is temporary until below data is included in the webhook
	// todo remove below once door rule status is included in webhook
	doorLockRuleStates := make(map[string]string)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, door := range appConfig.Doors {
				if door.HubitatLockID == nil {
					// no lock associated with this door
					continue
				}
				rule, err := uacClient.GetDoorLockRule(door.UacID)
				if err != nil {
					logger.Error("Failed to get door lock rule", slog.String("door_id", door.UacID),
						slog.String("err", err.Error()))
					continue
				}

				var currDoorLockRuleState string
				if rule.Type == "keep_unlock" {
					currDoorLockRuleState = "unlocked"
				} else if rule.Type == "" {
					currDoorLockRuleState = "locked"
				} else {
					logger.Warn("Unknown door lock rule type", slog.String("door_id", door.UacID),
						slog.String("rule_type", rule.Type))
					continue
				}

				prevState := doorLockRuleStates[door.UacID]
				if currDoorLockRuleState != prevState {
					if currDoorLockRuleState == "locked" {
						if err := hubitatClient.AssertDoorLockLocked(*door.HubitatLockID); err != nil {
							logger.Error("Failed to assert door lock locked", slog.String("door_id", door.UacID), slog.String("err", err.Error()))
						}
					} else if currDoorLockRuleState == "unlocked" {
						if err := hubitatClient.AssertDoorLockUnlocked(*door.HubitatLockID); err != nil {
							logger.Error("Failed to assert door lock unlocked", slog.String("door_id", door.UacID), slog.String("err", err.Error()))
						}
					}
					doorLockRuleStates[door.UacID] = currDoorLockRuleState
				}
			}
		}
	}
}
