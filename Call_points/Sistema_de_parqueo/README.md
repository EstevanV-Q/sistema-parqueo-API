# Colección Bruno — Sistema de Parqueo

Colección OpenCollection YAML para probar todos los endpoints del backend (`parking-backend`).

## Cómo usarla

1. Abrir Bruno → **Open Collection** → seleccionar la carpeta `Sistema_de_parqueo`.
2. Seleccionar el environment **Local** (arriba a la derecha).
3. Ejecutar los folders en orden: `00_health` → `01_auth` → `02_catalog` → ... → `12_realtime`.

> El primer request que debes ejecutar es `01_auth/01_login_admin.yml`. Los demás requests protegidos usan `{{accessToken}}`, que se captura automáticamente al hacer login.

## Credenciales del seed

| Rol | Usuario | Contraseña |
|---|---|---|
| admin | `admin` | `Admin123!` |
| operator | `operador` | `Oper123!` |
| auditor | `auditor` | `Audit123!` |

El dispositivo demo usa `X-Device-Key: device-api-key-001` (también está en el environment como `deviceApiKey`).

## Variables automáticas

Los scripts `after-response` capturan y encadenan los IDs necesarios:

| Variable | Se captura en |
|---|---|
| `accessToken`, `refreshToken` | `01_auth/01_login_admin.yml` |
| `operatorToken` | `01_auth/02_login_operador.yml` |
| `auditorToken` | `01_auth/03_login_auditor.yml` |
| `siteId` | `02_catalog/sites/01_listar_sedes.yml` |
| `zoneId` | `02_catalog/zones/01_listar_zonas.yml` |
| `spaceId` | `02_catalog/spaces/01_listar_espacios.yml` |
| `vehicleTypeId` | `02_catalog/vehicle-types/01_listar_tipos_vehiculo.yml` |
| `vehicleId`, `vehiclePlate` | `03_vehicles/01_listar_vehiculos.yml` |
| `customerId` | `04_customers/01_listar_clientes.yml` |
| `ratePlanId` | `05_rate-plans/01_listar_tarifas.yml` |
| `ticketId` | `06_tickets/01_crear_ticket.yml` |
| `paymentId` | `07_payments/01_crear_pago.yml` |
| `subscriptionId` | `08_subscriptions/01_crear_abono.yml` |
| `deviceId` / `deviceApiKey` | `09_devices/01_registrar_dispositivo.yml` |
| `today` | `10_reports/03_caja_diaria.yml` |

Si ejecutas un request aislado y una variable está vacía, ejecuta primero el listado/creación correspondiente.

## Orden recomendado

1. **00_health**: `healthz`, `readyz`, Swagger y OpenAPI.
2. **01_auth**: logins, refresh, `me`, registro y logout.
3. **02_catalog**: CRUD de sedes, zonas, espacios y tipos de vehículo.
4. **03_vehicles** y **04_customers**: listados y creación.
5. **05_rate-plans**: listado, creación, consulta y desactivación.
6. **06_tickets**: entrada, consulta, eventos, cotización, liquidación y anulación.
7. **07_payments**: registro, consulta, reembolso y listado.
8. **08_subscriptions**: creación, consulta, validación, listado y cancelación.
9. **09_devices**: registro, listado y evento con `X-Device-Key`.
10. **10_reports** y **11_audit**.
11. **12_realtime**: WebSocket de ocupación (conectar y esperar mensajes).

## Notas

- Las pruebas CRUD crean registros con prefijos `BRUNO-`, `TST`, `BT`, `Abono Bruno` y luego los desactivan/cancelan. No modifican los datos demo del seed (`PLAZA-01`, `XYZ987`, etc.).
- `06_tickets/07_cancelar_ticket.yml` devuelve `400` si el ticket ya fue liquidado (el ticket no está abierto). Para probar la anulación, crea un ticket nuevo y ejecuta solo ese request.
- `01_auth/07_logout.yml` revoca el refresh token; si lo ejecutas, vuelve a hacer login para continuar.
- El WebSocket `12_realtime/01_ocupacion_ws.yml` no requiere JWT; solo usa `site_id`.
- Los parámetros de query van incluidos en la URL para que funcionen igual en Bruno GUI y en Bruno CLI.
- `test live/Test_server_online.yml` se conserva como ejemplo original de prueba de conectividad.
- Si el puerto `8080` está ocupado por otro servicio, ejecuta la API en otro puerto y cambia `baseUrl`/`wsUrl` en `environments/local.yml`.

## Comando rápido con Bruno CLI

```bash
bru run "Call_points/Sistema_de_parqueo" --env Local
```

Si la API está en otro host/puerto, cambia `baseUrl` y `wsUrl` en `environments/local.yml`.
