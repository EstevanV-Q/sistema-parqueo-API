# Smoke test de la API de parqueo.
#
# Uso (PowerShell 5.1+ en Windows, requiere curl.exe):
#   $env:SMOKE_ADMIN_PASSWORD = '<password del usuario admin>'
#   # opcional: $env:SMOKE_BASE = 'http://127.0.0.1:8082'  (API de borde SQLite)
#   powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\smoke-api.ps1
#
# El script crea una sede, zona, espacio, tipo de vehículo, cliente, vehículo,
# tarifa, tickets, pagos, abono y dispositivo de prueba. Está pensado para
# entornos de desarrollo; no ejecutarlo contra producción.
$ErrorActionPreference = 'Continue'
$Base = if ($env:SMOKE_BASE) { $env:SMOKE_BASE } else { 'http://127.0.0.1:8080' }
$AdminPassword = $env:SMOKE_ADMIN_PASSWORD
if (-not $AdminPassword) {
    Write-Error 'Define SMOKE_ADMIN_PASSWORD con la contraseña del usuario admin.'
    exit 2
}
$Tmp = Join-Path $env:TEMP 'parking_smoke_resp.json'
$BodyFile = Join-Path $env:TEMP 'parking_smoke_body.json'
$script:Pass = 0
$script:Fail = 0
$script:Failures = @()

function Call {
    param([string]$Method, [string]$Path, [string]$Body, [string]$Token, [string]$DeviceKey)
    $curlArgs = @('-sS', '-o', $Tmp, '-w', '%{http_code}', '-X', $Method, ($Base + $Path))
    if ($Token) { $curlArgs += @('-H', "Authorization: Bearer $Token") }
    if ($DeviceKey) { $curlArgs += @('-H', "X-Device-Key: $DeviceKey") }
    if ($null -ne $Body -and $Body -ne '') {
        [System.IO.File]::WriteAllText($BodyFile, $Body, (New-Object System.Text.UTF8Encoding($false)))
        $curlArgs += @('-H', 'Content-Type: application/json', '--data-binary', "@$BodyFile")
    }
    $status = & curl.exe @curlArgs
    $content = ''
    if (Test-Path $Tmp) {
        $content = Get-Content $Tmp -Raw
        Remove-Item $Tmp -Force -ErrorAction SilentlyContinue
    }
    $json = $null
    if ($content) { try { $json = $content | ConvertFrom-Json } catch { } }
    return [pscustomobject]@{ Status = [int]$status; Content = $content; Json = $json }
}

function Check {
    param([string]$Name, $Resp, [int[]]$Expected = @(200, 201, 204))
    $ok = $Expected -contains $Resp.Status
    if ($ok) {
        $script:Pass++
        Write-Host ("PASS {0} [{1}]" -f $Name, $Resp.Status)
    } else {
        $script:Fail++
        $script:Failures += "$Name -> $($Resp.Status): $($Resp.Content)"
        Write-Host ("FAIL {0} [{1}] {2}" -f $Name, $Resp.Status, $Resp.Content)
    }
    return $ok
}

function Auth { param([string]$Token) return @{ Token = $Token } }

# ---------- publicos ----------
$r = Call 'GET' '/healthz'
Check 'healthz' $r @(200) | Out-Null
$r = Call 'GET' '/readyz'
Check 'readyz' $r @(200) | Out-Null
$r = Call 'GET' '/docs'
Check 'docs' $r @(200) | Out-Null
$r = Call 'GET' '/swagger/openapi.yaml'
Check 'openapi' $r @(200) | Out-Null
$r = Call 'GET' '/api/v1/catalog/sites'
Check 'catalog sin token -> 401' $r @(401) | Out-Null

# ---------- auth ----------
$login = Call 'POST' '/api/v1/auth/login' ("{`"username`":`"admin`",`"password`":`"$AdminPassword`"}")
Check 'login admin' $login @(200) | Out-Null
$tok = $login.Json.access_token
$refresh = $login.Json.refresh_token
if (-not $tok) { Write-Host 'No token, abortando'; exit 1 }

$r = Call 'GET' '/api/v1/auth/me' '' $tok
Check 'auth me' $r @(200) | Out-Null
Check 'me usuario admin' ([pscustomobject]@{ Status = $(if ($r.Json.username -eq 'admin') { 200 } else { 500 }); Content = $r.Content }) @(200) | Out-Null

$sites = Call 'GET' '/api/v1/catalog/sites' '' $tok
Check 'listar sedes' $sites @(200) | Out-Null
$siteId = $sites.Json[0].id

$stamp = [DateTimeOffset]::Now.ToUnixTimeSeconds()
$newUser = "smoke_op_$stamp"
$r = Call 'POST' '/api/v1/auth/register' ("{`"username`":`"$newUser`",`"email`":`"$newUser@example.com`",`"password`":`"Smoke-Pass-1234`",`"role`":`"operator`",`"site_id`":`"$siteId`"}") $tok
Check 'auth register operador' $r @(201) | Out-Null

$opLogin = Call 'POST' '/api/v1/auth/login' ("{`"username`":`"$newUser`",`"password`":`"Smoke-Pass-1234`"}")
Check 'login operador creado' $opLogin @(200) | Out-Null
$opTok = $opLogin.Json.access_token

$r = Call 'POST' '/api/v1/auth/refresh' ("{`"refresh_token`":`"$refresh`"}")
Check 'auth refresh' $r @(200) | Out-Null
$refresh2 = $r.Json.refresh_token

# ---------- catalog ----------
$vts = Call 'GET' '/api/v1/catalog/vehicle-types' '' $tok
Check 'listar tipos de vehiculo' $vts @(200) | Out-Null
$carType = ($vts.Json | Where-Object { $_.code -eq 'car' } | Select-Object -First 1)
if (-not $carType) { $carType = $vts.Json[0] }

$siteCode = "SMK$stamp"
$r = Call 'POST' '/api/v1/catalog/sites' ("{`"name`":`"Sede Smoke $stamp`",`"code`":`"$siteCode`",`"timezone`":`"America/Bogota`",`"currency`":`"COP`",`"capacity_total`":`20}") $tok
Check 'crear sede' $r @(201) | Out-Null
$newSite = $r.Json.id
$r = Call 'PUT' "/api/v1/catalog/sites/$newSite" ("{`"name`":`"Sede Smoke Actualizada $stamp`",`"code`":`"$siteCode`",`"timezone`":`"America/Bogota`",`"currency`":`"COP`",`"capacity_total`":`25}") $tok
Check 'actualizar sede' $r @(200) | Out-Null
$r = Call 'GET' "/api/v1/catalog/sites/$newSite" '' $tok
Check 'obtener sede' $r @(200) | Out-Null
$newUser2 = "smoke_op2_$stamp"
$r = Call 'POST' '/api/v1/auth/register' ("{`"username`":`"$newUser2`",`"email`":`"$newUser2@example.com`",`"password`":`"Smoke-Pass-1234`",`"role`":`"operator`",`"site_id`":`"$newSite`"}") $tok
Check 'registrar operador de la sede nueva' $r @(201) | Out-Null
$opLogin2 = Call 'POST' '/api/v1/auth/login' ("{`"username`":`"$newUser2`",`"password`":`"Smoke-Pass-1234`"}")
Check 'login operador sede nueva' $opLogin2 @(200) | Out-Null
$opTok2 = $opLogin2.Json.access_token
$r = Call 'POST' '/api/v1/catalog/sites' '{"name":"Prohibida","code":"NOPE","timezone":"America/Bogota","currency":"COP"}' $opTok
Check 'operador no puede crear sede -> 403' $r @(403) | Out-Null

$r = Call 'POST' '/api/v1/catalog/zones' ("{`"site_id`":`"$newSite`",`"name`":`"Zona Smoke`",`"type`":`"car`",`"capacity`":`20}") $tok
Check 'crear zona' $r @(201) | Out-Null
$newZone = $r.Json.id
$r = Call 'GET' ("/api/v1/catalog/zones?site_id=$newSite") '' $tok
Check 'listar zonas' $r @(200) | Out-Null
$r = Call 'PUT' "/api/v1/catalog/zones/$newZone" '{"name":"Zona Smoke Editada","type":"car","capacity":25}' $tok
Check 'actualizar zona' $r @(200) | Out-Null

$r = Call 'POST' '/api/v1/catalog/spaces' ("{`"zone_id`":`"$newZone`",`"code`":`"S-01`",`"type`":`"car`"}") $tok
Check 'crear espacio' $r @(201) | Out-Null
$newSpace = $r.Json.id
$r = Call 'GET' ("/api/v1/catalog/spaces?zone_id=$newZone") '' $tok
Check 'listar espacios' $r @(200) | Out-Null
$r = Call 'PUT' "/api/v1/catalog/spaces/$newSpace" '{"code":"S-01","type":"car","status":"free"}' $tok
Check 'actualizar espacio' $r @(200) | Out-Null
$r = Call 'PUT' "/api/v1/catalog/spaces/$newSpace/status" '{"status":"reserved"}' $tok
Check 'cambiar estado espacio' $r @(200) | Out-Null
$r = Call 'PUT' "/api/v1/catalog/spaces/$newSpace/status" '{"status":"free"}' $tok
Check 'restaurar estado espacio' $r @(200) | Out-Null

$typeCode = "TS$($stamp.ToString().Substring($stamp.ToString().Length-6))"
$typeName = "Tipo Smoke $stamp"
$editedTypeName = "Tipo Smoke Editado $stamp"
$r = Call 'POST' '/api/v1/catalog/vehicle-types' ("{`"name`":`"$typeName`",`"code`":`"$typeCode`"}") $tok
Check 'crear tipo de vehiculo' $r @(201) | Out-Null
$newType = $r.Json.id
$r = Call 'PUT' "/api/v1/catalog/vehicle-types/$newType" ("{`"name`":`"$editedTypeName`",`"code`":`"$typeCode`"}") $tok
Check 'actualizar tipo de vehiculo' $r @(200) | Out-Null
$r = Call 'POST' '/api/v1/catalog/vehicle-types' ("{`"name`":`"$editedTypeName`",`"code`":`"DUP$($stamp.ToString().Substring($stamp.ToString().Length-6))`"}") $tok
Check 'tipo de vehiculo duplicado -> 409' $r @(409) | Out-Null

# ---------- clientes / vehiculos ----------
$r = Call 'POST' '/api/v1/customers' ("{`"document`":`"SMK$stamp`",`"name`":`"Cliente Smoke`",`"phone`":`"8888-0000`",`"email`":`"smoke$stamp@example.com`"}") $tok
Check 'crear cliente' $r @(201) | Out-Null
$newCustomer = $r.Json.id
$r = Call 'GET' '/api/v1/customers?limit=10' '' $tok
Check 'listar clientes' $r @(200) | Out-Null
$r = Call 'GET' "/api/v1/customers/$newCustomer" '' $tok
Check 'obtener cliente' $r @(200) | Out-Null

$plate = "SMK$($stamp.ToString().Substring($stamp.ToString().Length-5))"
$r = Call 'POST' '/api/v1/vehicles' ("{`"plate_normalized`":`"$plate`",`"plate_raw`":`"$plate`",`"type_id`":`"$($carType.id)`",`"customer_id`":`"$newCustomer`"}") $tok
Check 'crear vehiculo' $r @(201) | Out-Null
$newVehicle = $r.Json.id
$r = Call 'GET' '/api/v1/vehicles?limit=10' '' $tok
Check 'listar vehiculos' $r @(200) | Out-Null
$r = Call 'GET' "/api/v1/vehicles/$newVehicle" '' $tok
Check 'obtener vehiculo' $r @(200) | Out-Null
$r = Call 'GET' "/api/v1/vehicles/plate/$plate" '' $tok
Check 'buscar vehiculo por placa' $r @(200) | Out-Null

# ---------- tarifas ----------
$r = Call 'POST' '/api/v1/rate-plans' ("{`"site_id`":`"$newSite`",`"name`":`"Tarifa Smoke`",`"currency`":`"COP`",`"grace_minutes`":`10,`"minimum_cents`":1000,`"fraction_minutes`":60,`"cents_per_fraction`":1500,`"daily_cap_cents`":20000,`"lost_ticket_cents`":25000,`"vehicle_type_id`":`"$($carType.id)`"}") $tok
Check 'crear tarifa' $r @(201) | Out-Null
$newPlan = $r.Json.id
$r = Call 'GET' ("/api/v1/rate-plans?site_id=$newSite") '' $tok
Check 'listar tarifas' $r @(200) | Out-Null
$r = Call 'GET' "/api/v1/rate-plans/$newPlan" '' $tok
Check 'obtener tarifa' $r @(200) | Out-Null

# ---------- tickets ----------
$enterBody = "{`"site_id`":`"$newSite`",`"zone_id`":`"$newZone`",`"space_id`":`"$newSpace`",`"vehicle_id`":`"$newVehicle`",`"plate_normalized`":`"$plate`",`"plate_raw`":`"$plate`",`"type_id`":`"$($carType.id)`",`"rate_plan_id`":`"$newPlan`"}"
$r = Call 'POST' '/api/v1/tickets' $enterBody $tok
Check 'crear ticket (entrada)' $r @(201) | Out-Null
$ticket = $r.Json.id
$r = Call 'GET' ("/api/v1/tickets?site_id=$newSite") '' $tok
Check 'listar tickets' $r @(200) | Out-Null
$r = Call 'GET' "/api/v1/tickets/$ticket" '' $tok
Check 'obtener ticket' $r @(200) | Out-Null
$r = Call 'GET' "/api/v1/tickets/$ticket/events" '' $tok
Check 'eventos ticket' $r @(200) | Out-Null
$r = Call 'POST' "/api/v1/tickets/$ticket/quote" '' $tok
Check 'cotizar ticket' $r @(200) | Out-Null
$r = Call 'POST' "/api/v1/tickets/$ticket/settle" '{"paid_cents":0,"method":"cash"}' $tok
Check 'liquidar ticket' $r @(200) | Out-Null
$r = Call 'POST' "/api/v1/tickets/$ticket/quote" '' $tok
Check 'cotizar ticket cerrado -> 400' $r @(400) | Out-Null

# ticket para cancelar
$plate2 = "SMX$($stamp.ToString().Substring($stamp.ToString().Length-5))"
$enter2 = "{`"site_id`":`"$newSite`",`"zone_id`":`"$newZone`",`"space_id`":`"$newSpace`",`"vehicle_id`":`"$newVehicle`",`"plate_normalized`":`"$plate2`",`"plate_raw`":`"$plate2`",`"type_id`":`"$($carType.id)`",`"rate_plan_id`":`"$newPlan`"}"
$r = Call 'POST' '/api/v1/tickets' $enter2 $tok
Check 'crear segundo ticket' $r @(201) | Out-Null
$ticket2 = $r.Json.id
$r = Call 'POST' "/api/v1/tickets/$ticket2/cancel" '' $opTok2
Check 'cancelar ticket (operador)' $r @(200) | Out-Null

# ---------- pagos ----------
$idem = "smoke-idem-$stamp"
$r = Call 'POST' '/api/v1/payments' ("{`"ticket_id`":`"$ticket`",`"amount_cents`":100,`"currency`":`"COP`",`"method`":`"cash`",`"reference`":`"SMOKE`",`"idempotency_key`":`"$idem`"}") $tok
Check 'crear pago (idempotencia)' $r @(201) | Out-Null
$payment = $r.Json.id
$r2 = Call 'POST' '/api/v1/payments' ("{`"ticket_id`":`"$ticket`",`"amount_cents`":100,`"currency`":`"COP`",`"method`":`"cash`",`"reference`":`"SMOKE`",`"idempotency_key`":`"$idem`"}") $tok
Check 'repetir pago idempotente' $r2 @(201) | Out-Null
Check 'idempotencia devuelve mismo pago' ([pscustomobject]@{ Status = $(if ($r2.Json.id -eq $payment) { 200 } else { 500 }); Content = $r2.Content }) @(200) | Out-Null
$r = Call 'GET' "/api/v1/payments/$payment" '' $tok
Check 'obtener pago' $r @(200) | Out-Null
$r = Call 'GET' ("/api/v1/payments?site_id=$newSite") '' $tok
Check 'listar pagos' $r @(200) | Out-Null
$r = Call 'POST' "/api/v1/payments/$payment/refund" '{"reason":"prueba smoke"}' $tok
Check 'reembolsar pago' $r @(200) | Out-Null

# ---------- abonados ----------
$start = (Get-Date).ToUniversalTime().AddDays(-1).ToString('yyyy-MM-ddTHH:mm:ssZ')
$end = (Get-Date).ToUniversalTime().AddDays(30).ToString('yyyy-MM-ddTHH:mm:ssZ')
$subBody = "{`"customer_id`":`"$newCustomer`",`"vehicle_id`":`"$newVehicle`",`"site_id`":`"$newSite`",`"plan_name`":`"Plan Smoke`",`"price_cents`":50000,`"starts_at`":`"$start`",`"expires_at`":`"$end`"}"
$r = Call 'POST' '/api/v1/subscriptions' $subBody $tok
Check 'crear abono' $r @(201) | Out-Null
$subscription = $r.Json.id
$r = Call 'GET' ("/api/v1/subscriptions?site_id=$newSite") '' $tok
Check 'listar abonos' $r @(200) | Out-Null
$r = Call 'GET' "/api/v1/subscriptions/$subscription" '' $tok
Check 'obtener abono' $r @(200) | Out-Null
$r = Call 'GET' ("/api/v1/subscriptions/validate?vehicle_id=$newVehicle&site_id=$newSite") '' $tok
Check 'validar abono' $r @(200) | Out-Null
$r = Call 'DELETE' "/api/v1/subscriptions/$subscription" '' $tok
Check 'cancelar abono' $r @(204) | Out-Null
$r = Call 'GET' ("/api/v1/subscriptions/validate?vehicle_id=$newVehicle&site_id=$newSite") '' $tok
Check 'validar abono cancelado -> 404' $r @(404) | Out-Null

# ---------- dispositivos ----------
$devKey = "smoke-device-key-$stamp"
$r = Call 'POST' '/api/v1/devices' ("{`"site_id`":`"$newSite`",`"name`":`"Barrera Smoke`",`"type`":`"barrier`",`"api_key`":`"$devKey`"}") $tok
Check 'registrar dispositivo' $r @(201) | Out-Null
$r = Call 'GET' ("/api/v1/devices?site_id=$newSite") '' $tok
Check 'listar dispositivos' $r @(200) | Out-Null
$eventID = "evt-$stamp"
$plate3 = "SMY$($stamp.ToString().Substring($stamp.ToString().Length-5))"
$evtBody = "{`"event_id`":`"$eventID`",`"event_type`":`"entry_detected`",`"plate_normalized`":`"$plate3`"}"
$r = Call 'POST' '/api/v1/devices/events' $evtBody '' $devKey
Check 'evento dispositivo entrada' $r @(200) | Out-Null
$r2 = Call 'POST' '/api/v1/devices/events' $evtBody '' $devKey
Check 'evento dispositivo idempotente' $r2 @(200) | Out-Null
$r = Call 'POST' '/api/v1/devices/events' $evtBody '' 'clave-invalida-123456'
Check 'evento dispositivo clave invalida -> 401' $r @(401) | Out-Null

# ---------- reportes ----------
$r = Call 'GET' ("/api/v1/reports/occupancy?site_id=$newSite") '' $tok
Check 'reporte ocupacion' $r @(200) | Out-Null
$today = (Get-Date).ToString('yyyy-MM-dd')
$r = Call 'GET' ("/api/v1/reports/revenue?site_id=$newSite&from=$today&to=$today") '' $tok
Check 'reporte ingresos' $r @(200) | Out-Null
$r = Call 'GET' ("/api/v1/reports/cash-daily?site_id=$newSite&date=$today") '' $tok
Check 'reporte caja diaria' $r @(200) | Out-Null
$r = Call 'GET' '/api/v1/reports/occupancy' '' $tok
Check 'ocupacion sin site_id -> 400' $r @(400) | Out-Null

# ---------- auditoria ----------
$r = Call 'GET' '/api/v1/audit?limit=10' '' $tok
Check 'listar auditoria' $r @(200) | Out-Null
$r = Call 'GET' ("/api/v1/audit?site_id=$newSite&limit=10") '' $tok
Check 'auditoria por sede' $r @(200) | Out-Null

# ---------- realtime ----------
$ws1 = Call 'POST' '/api/v1/auth/ws-ticket' '' $tok
Check 'auth ws-ticket' $ws1 @(200) | Out-Null
$wsTicket = $ws1.Json.ticket
$wsBase = $Base -replace '^http', 'ws'
$wsUrl = "$wsBase/api/v1/ws/occupancy?site_id=$newSite&ticket=$wsTicket"
try {
    $ws = New-Object System.Net.WebSockets.ClientWebSocket
    $cts = New-Object System.Threading.CancellationTokenSource
    $cts.CancelAfter(5000)
    $ws.ConnectAsync([Uri]$wsUrl, $cts.Token).Wait()
    $state = $ws.State
    try { $ws.CloseAsync([System.Net.WebSockets.WebSocketCloseStatus]::NormalClosure, 'bye', [Threading.CancellationToken]::None).Wait() | Out-Null } catch { $ws.Abort() }
    Check 'websocket ocupacion conecta' ([pscustomobject]@{ Status = $(if ($state -eq 'Open') { 101 } else { 500 }); Content = $state }) @(101) | Out-Null
} catch {
    $e = $_.Exception
    while ($e.InnerException) { $e = $e.InnerException }
    Check 'websocket ocupacion conecta' ([pscustomobject]@{ Status = 500; Content = "$($e.Message) | url=$wsUrl" }) @(101) | Out-Null
}

# ---------- logout ----------
$r = Call 'POST' '/api/v1/auth/logout' ("{`"refresh_token`":`"$refresh2`"}") $tok
Check 'logout' $r @(200) | Out-Null

# ---------- resumen ----------
Write-Host ''
Write-Host ("RESULTADO: {0} OK, {1} FALLOS" -f $script:Pass, $script:Fail)
if ($script:Fail -gt 0) {
    Write-Host '--- FALLOS ---'
    $script:Failures | ForEach-Object { Write-Host $_ }
    exit 1
}
exit 0
