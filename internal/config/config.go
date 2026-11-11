package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config contiene toda la configuración de la aplicación.
// Los valores se leen de variables de entorno y, opcionalmente, de un archivo .env.
// Ningún secreto tiene valor por defecto inseguro: deben proveerse explícitamente.
type Config struct {
	// Servidor
	Port     string
	Env      string
	LogLevel string

	// Base de datos
	DBDriver   string // mysql (central) o sqlite (borde)
	SQLitePath string
	DBUser     string
	DBPass     string
	DBHost     string
	DBPort     string
	DBName     string
	DBMaxOpen  int
	DBMaxIdle  int
	DBMaxLife  time.Duration

	// Sincronización del borde con la API central
	EdgeSiteID   string
	SyncURL      string
	SyncToken    string
	SyncInterval time.Duration

	// Seguridad JWT
	JWTSecret   string
	JWTIssuer   string
	JWTAudience string
	JWTAccess   time.Duration
	JWTRefresh  time.Duration

	// CORS: orígenes permitidos separados por coma.
	CORSAllowedOrigins []string

	// Zona horaria del sitio
	SiteTimezone string

	// Tipo de vehículo usado para entradas automáticas por LPR.
	DefaultVehicleTypeCode string

	// Timeouts HTTP
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

	// Seed (solo entornos no productivos)
	SeedAdminPassword    string
	SeedOperatorPassword string
	SeedAuditorPassword  string
	SeedDeviceAPIKey     string
	SeedSiteCode         string
	SeedSiteName         string
	SeedVehiclePlate     string

	// Bruno Collection
	BaseURL string
	WSURL   string
}

// Load lee la configuración desde el entorno y un archivo .env si existe.
// Retorna error si faltan variables requeridas o si hay valores inválidos.
func Load() (*Config, error) {
	// Cargar .env si existe (solo para desarrollo local)
	_ = loadEnvFile(".env")

	driver := strings.ToLower(strings.TrimSpace(getEnv("DB_DRIVER", "mysql")))
	if driver != "mysql" && driver != "sqlite" {
		return nil, fmt.Errorf("DB_DRIVER debe ser mysql o sqlite")
	}

	// Validar variables requeridas según el modo de despliegue.
	required := []string{"JWT_SECRET"}
	if driver == "sqlite" {
		required = append(required, "SQLITE_PATH")
	} else {
		required = append(required, "DB_USER", "DB_PASS", "DB_NAME")
	}
	var missing []string
	for _, key := range required {
		if value, exists := os.LookupEnv(key); !exists || strings.TrimSpace(value) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("variables de entorno requeridas no definidas: %s", strings.Join(missing, ", "))
	}

	// Advertencia si JWT_SECRET es débil en producción
	env := getEnv("ENV", "development")
	jwtSecret := getEnv("JWT_SECRET", "")
	if env == "production" && isWeakSecret(jwtSecret) {
		return nil, fmt.Errorf("JWT_SECRET no cumple requisitos de seguridad en entorno de producción")
	}

	cfg := &Config{
		Port:     getEnv("PORT", "8080"),
		Env:      env,
		LogLevel: getEnv("LOG_LEVEL", "info"),

		DBDriver:   driver,
		SQLitePath: getEnv("SQLITE_PATH", ""),
		DBUser:     getEnv("DB_USER", ""),
		DBPass:     getEnv("DB_PASS", ""),
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "3306"),
		DBName:     getEnv("DB_NAME", ""),
		DBMaxOpen:  getEnvInt("DB_MAX_OPEN", 25),
		DBMaxIdle:  getEnvInt("DB_MAX_IDLE", 25),
		DBMaxLife:  getEnvDuration("DB_MAX_LIFE", "5m"),

		EdgeSiteID:   getEnv("EDGE_SITE_ID", ""),
		SyncURL:      getEnv("SYNC_URL", ""),
		SyncToken:    getEnv("SYNC_TOKEN", ""),
		SyncInterval: getEnvDuration("SYNC_INTERVAL", "30s"),

		JWTSecret:          jwtSecret,
		JWTIssuer:          getEnv("JWT_ISSUER", "parking-backend"),
		JWTAudience:        getEnv("JWT_AUDIENCE", "parking-api"),
		JWTAccess:          getEnvDuration("JWT_ACCESS", "15m"),
		JWTRefresh:         getEnvDuration("JWT_REFRESH", "72h"),
		CORSAllowedOrigins: getEnvList("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000", "http://localhost:8080", "https://localhost:3000"}),

		SiteTimezone:           getEnv("SITE_TIMEZONE", "America/Bogota"),
		DefaultVehicleTypeCode: getEnv("DEFAULT_VEHICLE_TYPE_CODE", "car"),

		ReadTimeout:  getEnvDuration("READ_TIMEOUT", "10s"),
		WriteTimeout: getEnvDuration("WRITE_TIMEOUT", "10s"),
		IdleTimeout:  getEnvDuration("IDLE_TIMEOUT", "60s"),

		SeedAdminPassword:    getEnv("SEED_ADMIN_PASSWORD", ""),
		SeedOperatorPassword: getEnv("SEED_OPERATOR_PASSWORD", ""),
		SeedAuditorPassword:  getEnv("SEED_AUDITOR_PASSWORD", ""),
		SeedDeviceAPIKey:     getEnv("SEED_DEVICE_API_KEY", ""),
		SeedSiteCode:         getEnv("SEED_SITE_CODE", "PLAZA-01"),
		SeedSiteName:         getEnv("SEED_SITE_NAME", "Centro Comercial Plaza"),
		SeedVehiclePlate:     getEnv("SEED_VEHICLE_PLATE", "XYZ987"),

		BaseURL: getEnv("BASE_URL", "http://localhost:8080"),
		WSURL:   getEnv("WS_URL", "ws://localhost:8080"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validate ejecuta validaciones de negocio sobre la configuración cargada.
func (c *Config) validate() error {
	if c.IsSQLite() {
		if strings.TrimSpace(c.SQLitePath) == "" {
			return fmt.Errorf("SQLITE_PATH es requerido en modo sqlite")
		}
	} else if c.DBUser == "" || c.DBPass == "" || c.DBName == "" {
		return fmt.Errorf("DB_USER, DB_PASS y DB_NAME son requeridos")
	}
	if c.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET es requerido")
	}
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET debe tener al menos 32 caracteres")
	}
	if strings.TrimSpace(c.JWTIssuer) == "" {
		return fmt.Errorf("JWT_ISSUER es requerido")
	}
	if strings.TrimSpace(c.JWTAudience) == "" {
		return fmt.Errorf("JWT_AUDIENCE es requerido")
	}
	if c.JWTAccess <= 0 {
		return fmt.Errorf("JWT_ACCESS debe ser una duración válida mayor a cero")
	}
	if c.JWTRefresh <= 0 {
		return fmt.Errorf("JWT_REFRESH debe ser una duración válida mayor a cero")
	}
	if strings.TrimSpace(c.DefaultVehicleTypeCode) == "" {
		return fmt.Errorf("DEFAULT_VEHICLE_TYPE_CODE es requerido")
	}
	if !c.IsSQLite() && (c.DBMaxOpen <= 0 || c.DBMaxIdle <= 0) {
		return fmt.Errorf("DB_MAX_OPEN y DB_MAX_IDLE deben ser mayores a cero")
	}
	if c.SyncInterval <= 0 {
		return fmt.Errorf("SYNC_INTERVAL debe ser una duración válida mayor a cero")
	}
	if c.SyncURL != "" && c.SyncToken == "" {
		return fmt.Errorf("SYNC_TOKEN es requerido cuando SYNC_URL está configurado")
	}
	if c.IsSQLite() && c.SyncURL != "" && c.EdgeSiteID == "" {
		return fmt.Errorf("EDGE_SITE_ID es requerido para sincronizar una sede")
	}
	if c.EdgeSiteID != "" && len(c.EdgeSiteID) != 32 {
		return fmt.Errorf("EDGE_SITE_ID debe ser un UUID de 32 caracteres hexadecimales")
	}
	return nil
}

// IsSQLite indica si el proceso opera como API de borde local.
func (c *Config) IsSQLite() bool {
	return c.DBDriver == "sqlite"
}

// isWeakSecret detecta secretos por defecto o débiles.
func isWeakSecret(secret string) bool {
	weak := []string{
		"dev-secret-change-me",
		"change-me",
		"secret",
		"password",
		"123456",
	}
	for _, w := range weak {
		if strings.EqualFold(secret, w) {
			return true
		}
	}
	return false
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		return val
	}
	return fallback
}

func getEnvList(key string, fallback []string) []string {
	val := getEnv(key, "")
	if val == "" {
		return fallback
	}
	parts := strings.Split(val, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if item := strings.TrimSpace(part); item != "" {
			out = append(out, item)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

func getEnvInt(key string, fallback int) int {
	val := getEnv(key, "")
	if val == "" {
		return fallback
	}
	var n int
	if _, err := fmt.Sscanf(val, "%d", &n); err != nil {
		return fallback
	}
	return n
}

func getEnvDuration(key, fallback string) time.Duration {
	val := getEnv(key, "")
	if val == "" {
		d, _ := time.ParseDuration(fallback)
		return d
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		d, _ = time.ParseDuration(fallback)
		return d
	}
	return d
}

// loadEnvFile lee un archivo .env simple y establece variables de entorno
// solo si no están ya definidas. No falla si el archivo no existe.
func loadEnvFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil // archivo no existe, ignorar silenciosamente
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		// Quitar comillas si las tiene
		value = strings.Trim(value, "\"'")
		if key != "" && os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
	return nil
}
