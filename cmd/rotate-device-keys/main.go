// Comando rotate-device-keys invalida las API keys actuales de los
// dispositivos y genera claves nuevas. Las claves nuevas se imprimen una sola
// vez; guárdalas en el dispositivo o en el gestor de secretos correspondiente.
//
// Uso:
//
//	go run ./cmd/rotate-device-keys -yes
package main

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"

	"parking-backend/internal/config"
	"parking-backend/internal/platform/db"
	"parking-backend/internal/platform/ids"
	"parking-backend/internal/platform/security"
)

func main() {
	confirm := flag.Bool("yes", false, "confirmar la rotación de todas las API keys")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuración inválida: %v\n", err)
		os.Exit(1)
	}
	security.ConfigureTokenPepper(cfg.TokenPepper)

	database, err := db.Open(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error de base de datos: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	rows, err := database.Query(`SELECT id, site_id, name FROM devices WHERE is_active = TRUE ORDER BY name`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error consultando dispositivos: %v\n", err)
		os.Exit(1)
	}
	type row struct {
		id     string
		siteID string
		name   string
	}
	var devices []row
	for rows.Next() {
		var idBin, siteBin []byte
		var name string
		if err := rows.Scan(&idBin, &siteBin, &name); err != nil {
			_ = rows.Close()
			fmt.Fprintf(os.Stderr, "Error leyendo dispositivos: %v\n", err)
			os.Exit(1)
		}
		devices = append(devices, row{ids.Bin16ToString(idBin), ids.Bin16ToString(siteBin), name})
	}
	if err := rows.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "Error cerrando consulta: %v\n", err)
		os.Exit(1)
	}

	if !*confirm {
		fmt.Printf("Se rotarían %d dispositivos. Ejecuta con -yes para confirmar.\n", len(devices))
		return
	}

	for _, d := range devices {
		key, err := newAPIKey()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generando key para %s: %v\n", d.name, err)
			os.Exit(1)
		}
		if _, err := database.Exec(`UPDATE devices SET api_key_hash = ? WHERE id = ?`,
			security.HashToken(key), ids.StringToBin16(d.id)); err != nil {
			fmt.Fprintf(os.Stderr, "Error actualizando %s: %v\n", d.name, err)
			os.Exit(1)
		}
		fmt.Printf("%s\t%s\t%s\t%s\n", d.id, d.siteID, d.name, key)
	}
	fmt.Fprintln(os.Stderr, "Rotación completada. Guarda las claves; no se vuelven a mostrar.")
}

func newAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
