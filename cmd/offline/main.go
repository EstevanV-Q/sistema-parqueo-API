// Comando offline simula un agente de borde que opera sin red: encola eventos
// localmente y luego los sincroniza de forma idempotente con la API.
//
// Ejemplos:
//
//	go run ./cmd/offline -action enqueue -type entry -plate ABC123
//	go run ./cmd/offline -action sync
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"parking-backend/internal/offline"
	"parking-backend/internal/platform/ids"
	"parking-backend/internal/shared/plate"
)

type httpRemote struct {
	url    string
	apiKey string
	client *http.Client
}

func (r *httpRemote) Send(ctx context.Context, e offline.Event) error {
	body, err := json.Marshal(e.Payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Device-Key", r.apiKey)

	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("API respondió %d: %s", resp.StatusCode, string(raw))
	}
	return nil
}

func main() {
	var (
		action    = flag.String("action", "enqueue", "enqueue | sync")
		queuePath = flag.String("queue", "data/offline-queue.jsonl", "ruta de la cola local")
		eventType = flag.String("type", "entry", "entry | exit | heartbeat")
		plateRaw  = flag.String("plate", "", "placa del vehículo")
		apiURL    = flag.String("api", "http://localhost:8080/api/v1/devices/events", "URL de eventos de la API")
		deviceKey = flag.String("device-key", "device-api-key-001", "API key del dispositivo")
	)
	flag.Parse()

	q, err := offline.OpenQueue(*queuePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error al abrir cola: %v\n", err)
		os.Exit(1)
	}

	switch *action {
	case "enqueue":
		eventID := ids.UUIDv7String()
		normalized := plate.Normalize(*plateRaw)
		if normalized == "" {
			fmt.Fprintln(os.Stderr, "la placa es requerida para encolar un evento")
			os.Exit(1)
		}

		mappedType, err := mapEventType(*eventType)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		err = q.Enqueue(offline.Event{
			ID:   eventID,
			Type: mappedType,
			Payload: map[string]any{
				"event_id":         eventID,
				"event_type":       mappedType,
				"plate_normalized": normalized,
			},
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error al encolar: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("evento encolado: id=%s tipo=%s placa=%s pendientes=%d\n", eventID, mappedType, normalized, q.Len())

	case "sync":
		remote := &httpRemote{
			url:    *apiURL,
			apiKey: *deviceKey,
			client: &http.Client{Timeout: 10 * time.Second},
		}
		syncer := offline.NewSyncer(q, remote, 5, 2*time.Second)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		res, err := syncer.RunWithRetry(ctx)
		fmt.Printf("sincronización: enviados=%d fallidos=%d pendientes=%d\n", res.Sent, res.Failed, q.Len())
		if err != nil {
			fmt.Fprintf(os.Stderr, "error de sincronización: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "acción desconocida: %s (use enqueue o sync)\n", *action)
		os.Exit(1)
	}
}

func mapEventType(t string) (string, error) {
	switch t {
	case "entry", "entry_detected":
		return "entry_detected", nil
	case "exit", "exit_detected":
		return "exit_detected", nil
	case "heartbeat":
		return "heartbeat", nil
	default:
		return "", fmt.Errorf("tipo de evento desconocido: %s", t)
	}
}
