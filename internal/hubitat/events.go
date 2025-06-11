package hubitat

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync"
)

// WebhookEvent represents the top-level structure of a Hubitat event
type WebhookEvent struct {
	Content struct {
		Name            string      `json:"name"`
		Value           string      `json:"value"`
		DisplayName     string      `json:"displayName"`
		DeviceID        string      `json:"deviceId"`
		DescriptionText string      `json:"descriptionText"`
		Unit            interface{} `json:"unit"`
		Type            string      `json:"type"`
		Data            interface{} `json:"data"`
	} `json:"content"`
}

// WebhookHandler handles incoming Hubitat Access webhook requests
type WebhookHandler struct {
	authToken string
	onEvent   func(WebhookEvent)
	wg        *sync.WaitGroup
}

// NewWebhookHandler creates a new handler
func NewWebhookHandler(authToken string, onEvent func(WebhookEvent), wg *sync.WaitGroup) *WebhookHandler {
	return &WebhookHandler{authToken: authToken, onEvent: onEvent, wg: wg}
}

// ServeHTTP implements http.Handler for WebhookHandler
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	authToken := r.URL.Query().Get("authorization")
	if authToken != h.authToken {
		http.Error(w, "Forbidden", http.StatusForbidden)
		log.Printf("Invalid auth token")
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var event WebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("Invalid event JSON: %v", err)
		http.Error(w, "Invalid event JSON", http.StatusBadRequest)
		return
	}

	// Send to callback asynchronously
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		h.onEvent(event)
	}()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
