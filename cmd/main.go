package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/K-MTG/unifi-access-hubitat-middleware/cmd/config"
	"github.com/K-MTG/unifi-access-hubitat-middleware/internal/hubitat"
	"github.com/K-MTG/unifi-access-hubitat-middleware/internal/uac"
)

var (
	logger        *slog.Logger
	uacClient     *uac.Client
	hubitatClient *hubitat.Client
	appConfig     *config.Config
)

// getDoorByUacID returns the Door struct for a given UAC door ID.
func getDoorByUacID(uacID string) (door *config.Door, found bool) {
	for i, d := range appConfig.Doors {
		if d.UacID == uacID {
			return &appConfig.Doors[i], true
		}
	}
	return nil, false
}

// getDoorByHubitatID returns the Door struct and device type ("contact", "lock", or "switch") for a given Hubitat device ID.
func getDoorByHubitatID(hubitatID string) (door *config.Door, deviceType string, found bool) {
	for i, d := range appConfig.Doors {
		if d.HubitatContactID == hubitatID {
			return &appConfig.Doors[i], "contact", true
		}
		if d.HubitatLockID != nil && *d.HubitatLockID == hubitatID {
			return &appConfig.Doors[i], "lock", true
		}
		if d.HubitatSwitchID == hubitatID {
			return &appConfig.Doors[i], "switch", true
		}
	}
	return nil, "", false
}

func main() {
	var err error

	// initialize logger
	logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// get config path from argument
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		for i, arg := range os.Args {
			if arg == "--config" && i+1 < len(os.Args) {
				configPath = os.Args[i+1]
				break
			}
		}
	}

	// load config
	appConfig, err = config.LoadConfig(configPath)
	if err != nil {
		logger.Error("Error loading config", slog.String("ConfigPath", configPath),
			slog.String("err", err.Error()))
		os.Exit(1)
	}

	uacClient = uac.NewClient(appConfig.UAC.BaseURL, appConfig.UAC.APIKey)
	hubitatClient = hubitat.NewClient(appConfig.Hubitat.BaseURL, appConfig.Hubitat.AccessToken)

	// asset that uac webhook exists
	uacWebHook, err := assertUacWebhookExists()
	if err != nil {
		logger.Error("Error asserting webhook exists", slog.String("err", err.Error()))
		os.Exit(1)
	}

	wg := sync.WaitGroup{}

	// Register the signal handler for graceful shutdown
	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM)

	HServer := &http.Server{Addr: "0.0.0.0:9423"}

	// runs webserver in a goroutine for graceful shutdown
	go func(HServer *http.Server, wg *sync.WaitGroup) {
		// Create handlers
		uacHandler := uac.NewWebhookHandler(*uacWebHook.Secret, appConfig.Server.AuthToken, handleUacEvent, wg)
		hubitatHandler := hubitat.NewWebhookHandler(appConfig.Server.AuthToken, handleHubitatEvent, wg)

		// Register the routes
		http.Handle("/webhook/uac", uacHandler)
		http.Handle("/webhook/hubitat", hubitatHandler)

		// Start the HTTP server
		logger.Info("Starting Server")
		if err := http.ListenAndServe("0.0.0.0:9423", nil); err != nil {
			logger.Error("Failed to start server", slog.String("err", err.Error()))
			os.Exit(1)
		}
	}(HServer, &wg)

	// Create a cancellable context for the polling goroutine
	ctx, cancelPoll := context.WithCancel(context.Background())

	// Start the polling goroutine to check UAC states
	wg.Add(1)
	go pollUacStates(ctx, &wg)

	// Wait for a signal to shutdown
	sig := <-osSignals
	logger.Warn("Received shutdown signal", slog.String("signal", sig.String()))

	// Cancel the polling goroutine
	cancelPoll()

	ctxShutDown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := HServer.Shutdown(ctxShutDown); err != nil {
			logger.Error("Failed to shutdown server gracefully", slog.String("err", err.Error()))
		} else {
			logger.Info("Server shutdown gracefully")
		}
	}()

	wg.Wait()
	logger.Info("Exiting application")
}
