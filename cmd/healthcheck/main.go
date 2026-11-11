// Comando healthcheck consulta /healthz desde dentro del contenedor.
// Se usa porque la imagen distroless no incluye shell ni wget.
package main

import (
	"net/http"
	"os"
	"time"
)

func main() {
	url := os.Getenv("HEALTHCHECK_URL")
	if url == "" {
		url = "http://127.0.0.1:8080/healthz"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		os.Exit(1)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		os.Exit(1)
	}
}
