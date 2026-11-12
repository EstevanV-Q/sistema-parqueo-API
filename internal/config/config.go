package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
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
	DBTLS      string
	DBTLSCA    string
	DBTimeout  time.Duration
	DBRead     time.Duration
	DBWrite    time.Duration

	// Sincronización del borde con la API central
	EdgeSiteID            string
	SyncURL               string
	SyncToken             string
	SyncSiteTokens        map[string]string
	SyncInterval          time.Duration
	SyncProcessorInterval time.Duration

	// Seguridad
	TokenPepper       string
	TrustProxyHeaders bool

	// Permite ejecutar el seed en producción de forma explícita.
	AllowSeed bool

	// Seguridad JWT
	JWTSecret        string
	JWTAccessSecret  string
	JWTRefreshSecret string
	JWTIssuer        string
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

	// Advertencia si JWT_SECRET o TOKEN_PEPPER son débiles en producción.
	env := strings.ToLower(strings.TrimSpace(getEnv("ENV", "development")))
	jwtSecret := getEnv("JWT_SECRET", "")
	tokenPepper := getEnv("TOKEN_PEPPER", "")
	jwtAccessSecret := getEnv("JWT_ACCESS_SECRET", jwtSecret)
	jwtRefreshSecret := getEnv("JWT_REFRESH_SECRET", jwtSecret)
	if env == "production" {
		if isWeakSecret(jwtSecret) {
			return nil, fmt.Errorf("JWT_SECRET no cumple requisitos de seguridad en entorno de producción")
		}
		if len(tokenPepper) < 32 || isWeakSecret(tokenPepper) {
			return nil, fmt.Errorf("TOKEN_PEPPER debe tener al menos 32 caracteres y no ser un placeholder en producción")
		}
		if len(jwtAccessSecret) < 32 || len(jwtRefreshSecret) < 32 {
			return nil, fmt.Errorf("JWT_ACCESS_SECRET y JWT_REFRESH_SECRET deben tener al menos 32 caracteres en producción")
		}
		if isWeakSecret(jwtAccessSecret) || isWeakSecret(jwtRefreshSecret) {
			return nil, fmt.Errorf("JWT_ACCESS_SECRET y JWT_REFRESH_SECRET no deben ser placeholders")
		}
		if jwtAccessSecret == jwtRefreshSecret {
			return nil, fmt.Errorf("JWT_ACCESS_SECRET y JWT_REFRESH_SECRET deben ser distintos")
		}
	}

	syncSiteTokens, err := getEnvJSONMap("SYNC_SITE_TOKENS")
	if err != nil {
		return nil, err
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
		DBTLS:      strings.ToLower(strings.TrimSpace(getEnv("DB_TLS", "false"))),
		DBTLSCA:    getEnv("DB_TLS_CA", ""),
		DBTimeout:  getEnvDuration("DB_CONNECT_TIMEOUT", "5s"),
		DBRead:     getEnvDuration("DB_READ_TIMEOUT", "10s"),
		DBWrite:    getEnvDuration("DB_WRITE_TIMEOUT", "10s"),

		EdgeSiteID:            getEnv("EDGE_SITE_ID", ""),
		SyncURL:               getEnv("SYNC_URL", ""),
		SyncToken:             getEnv("SYNC_TOKEN", ""),
		SyncSiteTokens:        syncSiteTokens,
		SyncInterval:          getEnvDuration("SYNC_INTERVAL", "30s"),
		SyncProcessorInterval: getEnvDuration("SYNC_PROCESSOR_INTERVAL", "30s"),

		TokenPepper:       tokenPepper,
		TrustProxyHeaders: getEnvBool("TRUST_PROXY_HEADERS", false),
		AllowSeed:         getEnvBool("ALLOW_SEED", false),

		JWTSecret:          jwtSecret,
		JWTAccessSecret:    jwtAccessSecret,
		JWTRefreshSecret:   jwtRefreshSecret,
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
	switch c.DBTLS {
	case "false", "true", "skip-verify", "preferred", "custom":
	default:
		return fmt.Errorf("DB_TLS debe ser false, true, skip-verify, preferred o custom")
	}
	if c.DBTLS == "custom" && strings.TrimSpace(c.DBTLSCA) == "" {
		return fmt.Errorf("DB_TLS_CA es requerido cuando DB_TLS=custom")
	}
	if c.Env == "production" && !c.IsSQLite() && (c.DBTLS == "false" || c.DBTLS == "skip-verify" || c.DBTLS == "preferred") {
		return fmt.Errorf("DB_TLS debe ser true o custom en producción")
	}
	if c.DBTimeout <= 0 || c.DBRead <= 0 || c.DBWrite <= 0 {
		return fmt.Errorf("DB_CONNECT_TIMEOUT, DB_READ_TIMEOUT y DB_WRITE_TIMEOUT deben ser mayores a cero")
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
	if c.SyncProcessorInterval <= 0 {
		return fmt.Errorf("SYNC_PROCESSOR_INTERVAL debe ser una duración válida mayor a cero")
	}
	if c.SyncURL != "" && c.SyncToken == "" {
		return fmt.Errorf("SYNC_TOKEN es requerido cuando SYNC_URL está configurado")
	}
	if c.Env == "production" && c.SyncURL != "" && !strings.HasPrefix(strings.ToLower(c.SyncURL), "https://") {
		return fmt.Errorf("SYNC_URL debe usar HTTPS en producción")
	}
	if c.Env == "production" {
		if c.SyncToken != "" && len(c.SyncToken) < 32 {
			return fmt.Errorf("SYNC_TOKEN debe tener al menos 32 caracteres en producción")
		}
		if (c.SyncToken != "" || len(c.SyncSiteTokens) > 0) && len(c.SyncSiteTokens) == 0 && !c.IsSQLite() {
			return fmt.Errorf("SYNC_SITE_TOKENS es requerido en producción para sincronización")
		}
		for siteID, token := range c.SyncSiteTokens {
			if len(siteID) != 32 || len(token) < 32 {
				return fmt.Errorf("SYNC_SITE_TOKENS inválido para la sede %s", siteID)
			}
		}
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

// isWeakSecret detecta secretos por defecto, placeholders o débiles.
func isWeakSecret(secret string) bool {
	lower := strings.ToLower(strings.TrimSpace(secret))
	if lower == "" {
		return true
	}
	weak := []string{
		"dev-secret-change-me",
		"change-me",
		"secret",
		"password",
		"123456",
		"parking-dev-secret-change-in-production",
	}
	for _, w := range weak {
		if lower == w {
			return true
		}
	}
	// Rechazar placeholders y valores de ejemplo aunque sean largos.
	for _, marker := range []string{
		"cambiar", "change", "example", "placeholder", "tu-secreto",
		"<", ">", "admin123", "device-api-key-001",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return len(secret) < 32
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

func getEnvBool(key string, fallback bool) bool {
	val := getEnv(key, "")
	if val == "" {
		return fallback
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return fallback
	}
	return b
}

// getEnvJSONMap lee un objeto JSON de variables de entorno, por ejemplo:
// SYNC_SITE_TOKENS={"<site_id>":"<token>"}.
func getEnvJSONMap(key string) (map[string]string, error) {
	raw := strings.TrimSpace(getEnv(key, ""))
	if raw == "" {
		return nil, nil
	}
	out := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("%s debe ser un objeto JSON válido: %w", key, err)
	}
	return out, nil
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
