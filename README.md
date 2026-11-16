# Sistema de Control de Parqueo

Proyecto universitario. Controla la entrada y salida de vehículos, asigna
espacios automáticamente y calcula el cobro según el tiempo transcurrido.
Incluye una interfaz web y se distribuye como un único ejecutable de Windows.

## Uso rápido (Windows)

1. Descargar `parqueo.exe` y hacer doble clic. No requiere instalar nada.
2. Se abre el navegador en `http://localhost:8080` (si el puerto está ocupado,
   usa el siguiente libre; la consola muestra la dirección).
3. Ingresar con un usuario de prueba:

| Usuario    | Contraseña         | Puede                                  |
|------------|--------------------|----------------------------------------|
| `operador` | `parqueo-operador` | Registrar entradas, salidas y cobros   |
| `admin`    | `parqueo-admin`    | Todo, incluida la caja y los reportes  |
| `auditor`  | `parqueo-auditor`  | Consultar caja, reportes y auditoría   |

Los datos se guardan en la carpeta `datos/` junto al ejecutable. Para empezar
de cero, cerrar el programa y borrar esa carpeta. Para detenerlo, cerrar la
ventana de consola.

### Datos de prueba

- Sede **Parqueo Central**: 20 espacios de carro (A-01 a A-20) y 5 de moto (M-01 a M-05).
- Tarifas: carro ₡1 000/hora y moto ₡500/hora, 10 minutos de gracia, cada hora
  iniciada se cobra completa, tope diario de 12 horas.
- Vehículo abonado `XYZ987`: entra y sale sin pagar.

## Funciones

- **Entrada**: se escribe la placa y el tipo; el sistema asigna el primer
  espacio libre compatible (o uno elegido a mano). Rechaza placas que ya están
  dentro y avisa cuando el parqueo está lleno.
- **Salida y cobro**: calcula el monto (gracia, fracciones, mínimo y tope
  diario), registra el pago (efectivo con vuelto, tarjeta o SINPE), libera el
  espacio e imprime un recibo.
- **Mapa de espacios**, **vehículos dentro**, **caja del día**, **ingresos** y **tarifas**.

## Compilar

Requiere Go 1.27 o superior.

```bash
make exe        # genera bin/parqueo.exe (Windows, sin dependencias)
make desktop    # ejecuta la versión de escritorio en el equipo actual
make test       # pruebas unitarias e integración con SQLite
```

Sin `make`:

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/parqueo.exe ./cmd/parqueo
```

## Estructura

```
cmd/
  parqueo/      Ejecutable de escritorio (SQLite + interfaz + datos de prueba)
  api/          Servidor HTTP configurable por variables de entorno
  migrate/      Migraciones de MySQL (goose)
  seed/         Carga de datos de prueba
  healthcheck/  Verificación de salud para Docker
internal/
  app/          Arma servicios, handlers y rutas
  parking/      Tickets: entrada, cotización, cobro, cancelación, asignación de espacio
  parking/pricing/  Cálculo de la tarifa (función pura)
  catalog/      Sedes, zonas, espacios y tipos de vehículo
  billing/      Pagos e idempotencia
  rateplans/    Tarifas por sede y tipo de vehículo
  subscriptions/  Abonados
  vehicles/, customers/  Vehículos y clientes
  reporting/    Ocupación, ingresos y caja diaria
  auth/         Usuarios, roles y JWT
  audit/        Bitácora de operaciones
  seed/         Datos de prueba (idempotente)
  platform/     Base de datos, errores, middleware, seguridad
web/static/     Interfaz web (HTML, CSS y JS sin dependencias externas)
migrations/     Esquema MySQL
api/            Especificación OpenAPI (disponible en /docs)
```

Arquitectura por capas: `handler → service → repository → MySQL/SQLite`.
Los repositorios usan las mismas consultas en ambos motores.

## Modo servidor (MySQL)

```bash
cp .env.example .env            # completar contraseñas y JWT_SECRET
docker compose up -d mysql
go run ./cmd/migrate up
go run ./cmd/seed
go run ./cmd/api
```

Con `DB_DRIVER=sqlite` y `SQLITE_PATH=./datos/parqueo.db` se usa SQLite sin
MySQL ni migraciones (el esquema se crea solo).

## API principal

Documentación interactiva en `/docs`. Todas las rutas (salvo login) requieren
`Authorization: Bearer <token>`. Los montos van en céntimos: ₡1 000 = `100000`.

| Método | Ruta | Descripción |
|---|---|---|
| POST | `/api/v1/auth/login` | Iniciar sesión |
| POST | `/api/v1/tickets` | Registrar entrada (`space_id` opcional) |
| POST | `/api/v1/tickets/{id}/quote` | Cotizar salida |
| POST | `/api/v1/tickets/{id}/settle` | Pagar y registrar salida |
| POST | `/api/v1/tickets/{id}/cancel` | Anular ticket |
| GET | `/api/v1/tickets?site_id=&status=` | Listar tickets |
| GET | `/api/v1/catalog/spaces?zone_id=` | Espacios y su estado |
| GET | `/api/v1/reports/occupancy?site_id=` | Ocupación actual |
| GET | `/api/v1/reports/cash-daily?site_id=&date=` | Caja del día |
| GET | `/api/v1/reports/revenue?site_id=&from=&to=` | Ingresos por día |

La colección de Bruno en `Call_points/` prueba todos los endpoints.

## Licencia

Uso académico.
