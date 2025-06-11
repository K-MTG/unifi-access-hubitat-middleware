package uac

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// WebhookHandler handles incoming UniFi Access webhook requests
type WebhookHandler struct {
	secret    string
	authToken string
	onEvent   func(WebhookEvent)
	wg        *sync.WaitGroup
}

// NewWebhookHandler creates a new handler
func NewWebhookHandler(secret string, authToken string, onEvent func(WebhookEvent), wg *sync.WaitGroup) *WebhookHandler {
	return &WebhookHandler{secret: secret, authToken: authToken, onEvent: onEvent, wg: wg}
}

// ServeHTTP implements http.Handler for WebhookHandler
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	signature := r.Header.Get("Signature")
	authHeader := r.Header.Get("Authorization")
	if authHeader != h.authToken {
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

	rawEvent, err := constructEvent(body, signature, h.secret)
	if err != nil {
		log.Printf("Signature validation failed: %v", err)
		http.Error(w, fmt.Sprintf("Signature validation failed: %s", err), http.StatusUnauthorized)
		return
	}

	var event WebhookEvent
	if err := json.Unmarshal(rawEvent, &event); err != nil {
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

// WebhookEvent represents the top-level structure of a UniFi Access event
type WebhookEvent struct {
	Event         string          `json:"event"`
	EventObjectID string          `json:"event_object_id"`
	Data          json.RawMessage `json:"data"`
}

// --- Internal signature verification logic ---

var (
	ErrInvalidHeader    = errors.New("webhook has invalid Signature header")
	ErrNoValidSignature = errors.New("webhook had no valid signature")
	ErrNotSigned        = errors.New("webhook has no Signature header")
	signingVersion      = "v1"
)

type signedHeader struct {
	timestamp time.Time
	signature []byte
}

func parseSignatureHeader(header string) (*signedHeader, error) {
	sh := &signedHeader{}
	if header == "" {
		return sh, ErrNotSigned
	}

	pairs := strings.Split(header, ",")
	for _, pair := range pairs {
		parts := strings.Split(pair, "=")
		if len(parts) != 2 {
			return sh, ErrInvalidHeader
		}
		switch parts[0] {
		case "t":
			ts, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				return sh, ErrInvalidHeader
			}
			sh.timestamp = time.Unix(ts, 0)
		case signingVersion:
			sig, err := hex.DecodeString(parts[1])
			if err != nil {
				continue
			}
			sh.signature = sig
		}
	}

	if len(sh.signature) == 0 {
		return sh, ErrNoValidSignature
	}

	return sh, nil
}

func computeSignature(t time.Time, payload []byte, secret string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d", t.Unix())))
	mac.Write([]byte("."))
	mac.Write(payload)
	return mac.Sum(nil)
}

func validatePayload(payload []byte, sigHeader string, secret string) error {
	header, err := parseSignatureHeader(sigHeader)
	if err != nil {
		return err
	}
	expected := computeSignature(header.timestamp, payload, secret)
	if hmac.Equal(expected, header.signature) {
		return nil
	}
	return ErrNoValidSignature
}

func constructEvent(payload []byte, sigHeader string, secret string) (json.RawMessage, error) {
	if err := validatePayload(payload, sigHeader, secret); err != nil {
		return nil, err
	}
	var e json.RawMessage
	if err := json.Unmarshal(payload, &e); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return e, nil
}
