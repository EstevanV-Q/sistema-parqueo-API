package db

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"

	"parking-backend/internal/config"
)

var (
	tlsMu     sync.Mutex
	tlsLoaded = map[string]bool{}
)

// MySQLDSN construye el DSN de MySQL aplicando TLS, timeouts y zona UTC.
func MySQLDSN(cfg *config.Config) (string, error) {
	mysqlCfg := mysql.NewConfig()
	mysqlCfg.User = cfg.DBUser
	mysqlCfg.Passwd = cfg.DBPass
	mysqlCfg.Net = "tcp"
	mysqlCfg.Addr = net.JoinHostPort(cfg.DBHost, cfg.DBPort)
	mysqlCfg.DBName = cfg.DBName
	mysqlCfg.ParseTime = true
	mysqlCfg.Loc = time.UTC
	mysqlCfg.Timeout = cfg.DBTimeout
	mysqlCfg.ReadTimeout = cfg.DBRead
	mysqlCfg.WriteTimeout = cfg.DBWrite
	mysqlCfg.Params = map[string]string{
		"charset":   "utf8mb4",
		"time_zone": "'+00:00'",
	}

	switch cfg.DBTLS {
	case "true":
		mysqlCfg.TLSConfig = "true"
	case "skip-verify":
		mysqlCfg.TLSConfig = "skip-verify"
	case "preferred":
		mysqlCfg.TLSConfig = "preferred"
	case "custom":
		name, err := registerCustomTLS(cfg)
		if err != nil {
			return "", err
		}
		mysqlCfg.TLSConfig = name
	case "false":
		// Sin TLS. Solo permitido en desarrollo; config.Load lo exige en producción.
	default:
		return "", fmt.Errorf("DB_TLS inválido: %s", cfg.DBTLS)
	}

	return mysqlCfg.FormatDSN(), nil
}

func registerCustomTLS(cfg *config.Config) (string, error) {
	pem, err := os.ReadFile(cfg.DBTLSCA)
	if err != nil {
		return "", fmt.Errorf("leer DB_TLS_CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return "", fmt.Errorf("DB_TLS_CA no contiene certificados PEM válidos")
	}
	sum := sha256.Sum256(pem)
	name := "parking-custom-" + hex.EncodeToString(sum[:8])

	tlsMu.Lock()
	defer tlsMu.Unlock()
	if !tlsLoaded[name] {
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    pool,
			ServerName: cfg.DBHost,
		}
		if err := mysql.RegisterTLSConfig(name, tlsConfig); err != nil {
			return "", fmt.Errorf("registrar TLS de MySQL: %w", err)
		}
		tlsLoaded[name] = true
	}
	return name, nil
}

// New crea y configura el pool de conexiones a MySQL.
func New(cfg *config.Config) (*sql.DB, error) {
	dsn, err := MySQLDSN(cfg)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("error al abrir conexión: %w", err)
	}

	maxOpen := cfg.DBMaxOpen
	if maxOpen <= 0 {
		maxOpen = 25
	}
	maxIdle := cfg.DBMaxIdle
	if maxIdle <= 0 {
		maxIdle = 25
	}
	maxLife := cfg.DBMaxLife
	if maxLife <= 0 {
		maxLife = 5 * time.Minute
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(maxLife)
	db.SetConnMaxIdleTime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("no se pudo conectar a MySQL: %w", err)
	}

	return db, nil
}
