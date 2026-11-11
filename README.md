# Parking Backend

Backend del Sistema de Control de Parqueo/Estacionamiento — Proyecto Universitario.

## Stack

- **Go 1.27** + `chi` (router HTTP)
- **MySQL 8** (base de datos principal)
- **JWT** (access 15 min + refresh rotativo, Argon2id)
- **Docker Compose** + Caddy (despliegue)
- **UUIDv7** (ordenables por tiempo, BINARY(16))
- **Dinero en enteros** (centavos, nunca float)

## Estructura

```
cmd/
  api/          Punto de entrada de la API
  migrate/      Migraciones goose (up/down/status)
  seed/         Datos de demostración
internal/
  auth/         Usuarios, roles, login, refresh, logout, me
  catalog/      Sedes, zonas, espacios, tipos de vehículo
  parking/      Tickets, entrada/salida, motor de tarifas
  parking/pricing/  Cálculo puro de tarifas (gracia, mínimo, fracción, tope)
  billing/      Pagos, recibos internos, idempotencia
  subscriptions/# Abonados/mensualidades (validación por placa)
  vehicles/     Vehículos (placa normalizada, tipo)
  customers/    Clientes (documento, nombre, contacto)
  rateplans/    Tarifas por sede y tipo de vehículo
  devices/      Dispositivos (barrera virtual, eventos HMAC)
  realtime/     Ocupación por WebSocket (salas por sede)
  reporting/    Ocupación, ingresos, caja diaria
  platform/     DB, logger, seguridad, JWT, errores, IDs, contexto
  shared/       Money (centavos), Plate (normalización)
migrations/     Migraciones SQL versionadas (goose)
deploy/         Caddyfile (TLS)
```

## Requisitos previos

- Go 1.27+
- MySQL 8 (o Docker Compose)
- Docker (opcional, para contenedores)

## Ejecución local

```bash
# 1. Copiar variables de entorno
cp .env.example .env   # o usar .env ya creado

# 2. Levantar MySQL
docker compose up -d mysql

# 3. Ejecutar migraciones
go run cmd/migrate/main.go up

# 4. Cargar datos de demostración
go run cmd/seed/main.go

# 5. Ejecutar la API
go run ./cmd/api
```

La API escucha en `http://localhost:8080`.

### Verificar la conexión a la base de datos

```bash
go run ./cmd/dbcheck
# o: make dbcheck
```

El comando lee `.env`, prueba la conexión y lista las tablas del esquema. Si
faltan tablas, indica ejecutar `go run cmd/migrate/main.go up`.

### Pruebas de humo de los endpoints

Con la API levantada y un usuario `admin` sembrado:

```powershell
$env:SMOKE_ADMIN_PASSWORD = '<contraseña del admin>'
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\smoke-api.ps1
```

El script recorre salud, autenticación, catálogo, vehículos, clientes,
tarifas, tickets, pagos, abonos, dispositivos, reportes, auditoría y
WebSocket. Para probar la API de borde SQLite, define también
`SMOKE_BASE=http://127.0.0.1:8082`. Crea datos de prueba, por lo que debe
ejecutarse solo en desarrollo.

### API de borde (operación offline)

Cada sede puede ejecutar una instancia local con SQLite. La operación de
tickets, pagos, abonados y dispositivos funciona sin conexión; los cambios se
guardan en una outbox transaccional y se envían al central cuando hay red.

```bash
# Variables mínimas para el borde
DB_DRIVER=sqlite
SQLITE_PATH=./data/edge.db
JWT_SECRET=<secreto-largo-y-aleatorio>
SEED_ADMIN_PASSWORD=<mínimo-12>
SEED_OPERATOR_PASSWORD=<mínimo-12>
SEED_AUDITOR_PASSWORD=<mínimo-12>
SEED_DEVICE_API_KEY=<mínimo-16>

# Crea el esquema y los datos mínimos de la sede
go run ./cmd/seed

# API local de la sede
go run ./cmd/api
```

La outbox se sincroniza con `SYNC_URL` y `SYNC_TOKEN`; si `SYNC_URL` está
vacío, el borde sigue operando y guarda los eventos localmente. El central
expone `POST /api/v1/sync/events` para recibir los eventos de forma idempotente
en `sync_inbox` (requiere `SYNC_TOKEN` y el encabezado `X-Site-ID`).

## Endpoints principales

### Auth (público, excepto register)
- `POST /api/v1/auth/login` — iniciar sesión
- `POST /api/v1/auth/refresh` — renovar access token
- `POST /api/v1/auth/logout` — cerrar sesión
- `GET /api/v1/auth/me` — usuario autenticado
- `POST /api/v1/auth/ws-ticket` — ticket efímero para WebSocket (autenticado)
- `POST /api/v1/auth/register` — crear usuario (solo admin)

### Parqueo (tickets)
- `POST /api/v1/tickets` — entrada de vehículo
- `POST /api/v1/tickets/{id}/quote` — cotizar salida
- `POST /api/v1/tickets/{id}/settle` — pagar y salir
- `POST /api/v1/tickets/{id}/cancel` — cancelar ticket
- `GET /api/v1/tickets?site_id=&status=` — listar tickets

### Pagos
- `POST /api/v1/payments` — registrar pago
- `GET /api/v1/payments` — listar pagos
- `GET /api/v1/payments/{id}` — obtener pago

### Abonados
- `POST /api/v1/subscriptions` — crear abono
- `GET /api/v1/subscriptions/validate?vehicle_id=&site_id=` — validar abono

### Catálogo
- `POST /api/v1/catalog/sites` — crear sede
- `GET /api/v1/catalog/sites` — listar sedes
- `POST /api/v1/catalog/zones` — crear zona
- `POST /api/v1/catalog/spaces` — crear espacio
- `POST /api/v1/catalog/vehicle-types` — crear tipo de vehículo
- `POST /api/v1/catalog/rate-plans` — crear tarifa

### Reportes
- `GET /api/v1/reports/occupancy?site_id=` — ocupación en tiempo real
- `GET /api/v1/reports/revenue?site_id=&from=&to=` — ingresos por día
- `GET /api/v1/reports/cash-daily?site_id=&date=` — caja diaria

### Dispositivos (barrera virtual simulada)
- `POST /api/v1/devices` — registrar dispositivo
- `POST /api/v1/devices/events` — evento de barrera/LPR/RFID
- `GET /api/v1/devices` — listar dispositivos

### Tiempo real
- `GET /api/v1/ws/occupancy?site_id=&ticket=` — WebSocket de ocupación (ticket de `/auth/ws-ticket`)

### Salud
- `GET /healthz` — liveness
- `GET /readyz` — readiness (incluye DB)

## Datos de demostración (seed)

El seed usa usuarios `admin`, `operador` y `auditor`, pero no define contraseñas
por defecto. Antes de ejecutarlo configura `SEED_ADMIN_PASSWORD`,
`SEED_OPERATOR_PASSWORD`, `SEED_AUDITOR_PASSWORD` y `SEED_DEVICE_API_KEY`.

- **Sede**: Centro Comercial Plaza (PLAZA-01)
- **Espacios**: A-01 a A-10 (zona General - Carros)
- **Vehículo abonado**: XYZ-987 (plan Mensual Carro, 30 días)
- **Dispositivo**: Barrera Principal (clave tomada de `SEED_DEVICE_API_KEY`)

## Arquitectura

Capas estrictas: `handler → service → repository (interfaz) → MySQL/SQLite`.
El dominio y el motor de tarifas son puros (sin dependencias de infraestructura).

- **Central**: MySQL, catálogo/tarifas/usuarios, consolidación y reportes.
- **Borde**: API por sede con SQLite WAL, login offline con hashes cacheados,
  tickets y pagos locales, y outbox transaccional hacia el central.
- **Sincronización**: eventual e idempotente por evento; el central recibe en
  `sync_inbox` y no edita tickets/pagos del borde en v1.
- **Consistencia**: una operación local y su evento de outbox se confirman en
  la misma transacción. Los conflictos quedan visibles para revisión.

## Próximos pasos (pendiente)

- Procesador del `sync_inbox` para consolidar tickets/pagos en tablas centrales.
- Descarga de catálogo/tarifas/usuarios desde el central hacia el borde.
- Panel de conflictos y reintentos manuales de sincronización.
- Swagger/OpenAPI (`/swagger/*`)
- Frontend (React/Vue/Go templates)
- Tests de integración con MySQL real (testcontainers)

## Licencia

Uso académico — Proyecto universitario.