// Comando parqueo: ejecutable de escritorio. Usa SQLite en la carpeta "datos"
// junto al ejecutable, carga datos de prueba y abre la interfaz en el navegador.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // zonas horarias embebidas (Windows no las trae)

	"parking-backend/internal/app"
	"parking-backend/internal/config"
	"parking-backend/internal/platform/db"
	"parking-backend/internal/seed"
)

func main() {
	if err := run(); err != nil {
		log.Printf("ERROR: %v", err)
		fmt.Println("\nPresione Enter para cerrar...")
		_, _ = fmt.Scanln()
		os.Exit(1)
	}
}

func run() error {
	dataDir := filepath.Join(exeDir(), "datos")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("crear carpeta de datos: %w", err)
	}
	secret, err := loadSecret(filepath.Join(dataDir, "clave.txt"))
	if err != nil {
		return err
	}
	port, err := freePort(8080)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("http://localhost:%d", port)

	// Valores por defecto; una variable de entorno o un .env los reemplaza.
	defaults := map[string]string{
		"DB_DRIVER":              "sqlite",
		"SQLITE_PATH":            filepath.Join(dataDir, "parqueo.db"),
		"JWT_SECRET":             secret,
		"JWT_ACCESS":             "12h",
		"PORT":                   fmt.Sprint(port),
		"LOG_LEVEL":              "warn",
		"CORS_ALLOWED_ORIGINS":   url,
		"SITE_TIMEZONE":          "America/Costa_Rica",
		"SEED_SITE_CODE":         "PARQ-01",
		"SEED_SITE_NAME":         "Parqueo Central",
		"SEED_ADMIN_PASSWORD":    "parqueo-admin",
		"SEED_OPERATOR_PASSWORD": "parqueo-operador",
		"SEED_AUDITOR_PASSWORD":  "parqueo-auditor",
	}
	for k, v := range defaults {
		if _, ok := os.LookupEnv(k); !ok {
			_ = os.Setenv(k, v)
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuración: %w", err)
	}
	database, err := db.Open(cfg)
	if err != nil {
		return fmt.Errorf("base de datos: %w", err)
	}
	if _, err := seed.Run(database, cfg); err != nil {
		return fmt.Errorf("datos iniciales: %w", err)
	}
	application := app.NewWithDB(cfg, database)
	defer func() { _ = application.Close() }()

	srv := &http.Server{
		Addr:              "127.0.0.1:" + cfg.Port,
		Handler:           application.Router,
		ReadHeaderTimeout: cfg.ReadTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	fmt.Println("==============================================")
	fmt.Println("  Sistema de Control de Parqueo")
	fmt.Println("==============================================")
	fmt.Printf("  Abra en el navegador: %s\n", url)
	fmt.Println("  Usuarios: operador / parqueo-operador")
	fmt.Println("            admin    / parqueo-admin")
	fmt.Printf("  Datos guardados en: %s\n", dataDir)
	fmt.Println("  Cierre esta ventana para detener el sistema.")
	fmt.Println("==============================================")
	openBrowser(url)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("servidor: %w", err)
		}
	case <-quit:
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

// exeDir devuelve la carpeta del ejecutable (o la actual si no se puede).
func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// loadSecret lee la clave JWT del archivo o crea una aleatoria.
func loadSecret(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(b))) >= 32 {
		return strings.TrimSpace(string(b)), nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	secret := hex.EncodeToString(buf)
	if err := os.WriteFile(path, []byte(secret), 0o600); err != nil {
		return "", fmt.Errorf("guardar clave: %w", err)
	}
	return secret, nil
}

// freePort devuelve el primer puerto libre desde start.
func freePort(start int) (int, error) {
	for p := start; p < start+50; p++ {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			_ = l.Close()
			return p, nil
		}
	}
	return 0, fmt.Errorf("no hay puertos libres entre %d y %d", start, start+49)
}

// openBrowser abre la URL en el navegador predeterminado.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
