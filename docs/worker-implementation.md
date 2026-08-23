Quiero que crees desde cero un proyecto independiente en Go llamado:

`ImageRelayWorker`

Objetivo:

Este worker correrá en una PC ubicada en Uruguay.

Su única responsabilidad es:

1. Consultar periódicamente a una API central desplegada en Railway.
2. Pedir un trabajo pendiente de descarga de imagen.
3. Recibir una URL de imagen.
4. Descargar la imagen desde la PC uruguaya.
5. Enviar esa imagen a Railway como binario mediante streaming HTTP.
6. Informar éxito o error.
7. Repetir el proceso continuamente.

IMPORTANTE:

* NO usar Base64.
* NO almacenar imágenes permanentemente en disco.
* NO conectarse directamente a PostgreSQL.
* NO conectarse directamente a S3.
* NO tener credenciales de S3.
* NO contener lógica específica de Selvir.
* NO contener lógica de scraping HTML.
* NO modificar imágenes.
* NO convertir formatos.
* NO comprimir/reprocesar JPEG/PNG/WebP.
* La PC uruguaya debe actuar únicamente como downloader/relay.
* Railway seguirá siendo responsable de persistencia, PostgreSQL, S3 y lógica de negocio.

---

# Stack

Usar:

* Go estable actual.
* Solo librería estándar siempre que sea razonable.
* `net/http`
* `context`
* `encoding/json`
* `io`
* `log/slog`
* `os`
* `os/signal`
* `syscall`
* `time`
* `sync`
* `crypto/sha256` si resulta útil para validación.
* Evitar frameworks HTTP innecesarios.

El proyecto debe compilar como un único binario.

---

# Arquitectura

Flujo:

Railway API
↓
Worker pregunta por trabajo
↓
Railway devuelve URL
↓
Worker descarga imagen desde origen
↓
Worker hace streaming binario hacia Railway
↓
Railway recibe archivo
↓
Railway sube a S3
↓
Railway responde éxito
↓
Worker confirma trabajo
↓
Worker pide siguiente trabajo

---

# Estructura del proyecto

Crear una estructura limpia similar a:

```text
ImageRelayWorker/
├── cmd/
│   └── worker/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── api/
│   │   └── client.go
│   ├── downloader/
│   │   └── downloader.go
│   ├── worker/
│   │   └── worker.go
│   ├── retry/
│   │   └── retry.go
│   └── model/
│       └── job.go
├── .env.example
├── .gitignore
├── go.mod
├── README.md
└── Makefile
```

Podés ajustar mínimamente la estructura si mejora claridad.

---

# Configuración por environment variables

Implementar como mínimo:

```env
API_BASE_URL=https://example-production-api.up.railway.app
WORKER_TOKEN=replace-me
WORKER_ID=uy-image-worker-01

POLL_INTERVAL_SECONDS=5

MAX_CONCURRENT_JOBS=4

DOWNLOAD_TIMEOUT_SECONDS=30
UPLOAD_TIMEOUT_SECONDS=60

MAX_IMAGE_SIZE_MB=25

HTTP_USER_AGENT=ImageRelayWorker/1.0

RETRY_MAX_ATTEMPTS=4
RETRY_BASE_DELAY_MS=1000

LOG_LEVEL=info
```

No hardcodear secrets.

Validar configuración al iniciar.

Si falta:

* API_BASE_URL
* WORKER_TOKEN
* WORKER_ID

el proceso debe fallar inmediatamente con un error claro.

---

# API contract esperado

Diseñar el worker usando este contrato.

## Pedir trabajo

```http
POST /api/image-jobs/claim
Authorization: Bearer WORKER_TOKEN
Content-Type: application/json
```

Body:

```json
{
  "workerId": "uy-image-worker-01"
}
```

Railway puede responder:

### Hay trabajo

HTTP 200

```json
{
  "job": {
    "id": "job_123",
    "imageUrl": "https://mayoristas.selvir.com.uy/path/image.jpg",
    "productId": "product_456"
  }
}
```

### No hay trabajo

HTTP 204

El worker espera `POLL_INTERVAL_SECONDS` y vuelve a consultar.

---

# Descarga de imagen

Cuando haya trabajo:

```text
GET imageUrl
```

Usar:

```http
User-Agent: HTTP_USER_AGENT
Accept: image/avif,image/webp,image/png,image/jpeg,image/*
```

Permitir redirects HTTP razonables.

Usar timeout configurable.

Validar:

* status HTTP entre 200 y 299.
* Content-Type compatible con imagen.
* tamaño máximo.
* que haya body.
* que el archivo no esté vacío.

Content-Type válidos:

```text
image/jpeg
image/png
image/webp
image/gif
image/avif
```

Si el servidor no informa Content-Type correctamente pero devuelve datos válidos, usar `http.DetectContentType` sobre una pequeña cabecera del stream.

No cargar la imagen completa en RAM.

Implementar streaming.

Si necesitás leer los primeros bytes para detectar Content-Type, conservar esos bytes usando `io.MultiReader`.

---

# Límite de tamaño

Aplicar límite estricto configurable:

```text
MAX_IMAGE_SIZE_MB
```

Por ejemplo 25 MB.

No permitir que una respuesta infinita o incorrecta consuma memoria/disco indefinidamente.

Usar `io.LimitReader` o equivalente.

Si supera el límite:

* cancelar descarga.
* marcar job como fallido.

---

# Upload hacia Railway

Enviar imagen mediante streaming binario.

Endpoint:

```http
POST /api/image-jobs/{jobId}/upload
```

Headers:

```http
Authorization: Bearer WORKER_TOKEN
Content-Type: image/jpeg
X-Worker-Id: uy-image-worker-01
X-Source-Url: <original image URL>
X-Product-Id: <product id>
```

Body:

```text
RAW BINARY IMAGE STREAM
```

NO multipart salvo que sea realmente necesario.

Preferir raw binary body.

NO Base64.

NO buffering completo del archivo.

La descarga desde origen debe poder conectarse directamente con el upload mediante streaming.

Idealmente:

```text
remote HTTP body
→ Go io.Reader
→ Railway HTTP request body
```

sin crear archivo temporal.

---

# Problema importante: streaming y retries

Diseñá correctamente este punto.

Una vez consumido un stream HTTP, no se puede reutilizar directamente.

Por lo tanto:

* un fallo durante upload puede requerir volver a descargar la imagen desde origen.
* los retries del proceso completo deben repetir:

  * download
  * upload

NO intentar reutilizar un body ya consumido.

Implementar retry alrededor de la operación completa.

---

# Confirmación

Si Railway responde satisfactoriamente al upload:

HTTP 200 o 201.

Ejemplo:

```json
{
  "success": true,
  "imageUrl": "https://cdn.example.com/images/abc123.jpg"
}
```

Después hacer:

```http
POST /api/image-jobs/{jobId}/complete
```

Body:

```json
{
  "workerId": "uy-image-worker-01"
}
```

La API puede responder HTTP 200 o 204.

---

# Reporte de error

Si después de todos los retries falla:

```http
POST /api/image-jobs/{jobId}/fail
```

Body:

```json
{
  "workerId": "uy-image-worker-01",
  "error": "mensaje resumido del error"
}
```

Nunca enviar stack traces enormes.

Limitar mensaje de error.

---

# Concurrencia

Implementar procesamiento concurrente.

Environment:

```env
MAX_CONCURRENT_JOBS=4
```

Usar goroutines y un límite de concurrencia claro.

NO crear goroutines ilimitadas.

Cada worker concurrente debe pedir/procesar trabajos de forma segura.

Evitar que el mismo job sea procesado múltiples veces desde este proceso.

Railway debe ser la autoridad definitiva para `claim`.

---

# Polling

Cuando no haya trabajos:

esperar:

```text
POLL_INTERVAL_SECONDS
```

No hacer busy loop.

Añadir pequeño jitter aleatorio para evitar sincronización si en el futuro hay varios workers.

Ejemplo:

```text
5 segundos ± 500 ms
```

---

# Retry policy

Implementar exponential backoff.

Variables:

```env
RETRY_MAX_ATTEMPTS=4
RETRY_BASE_DELAY_MS=1000
```

Ejemplo:

```text
1s
2s
4s
8s
```

Con jitter.

Retry para:

* timeouts.
* connection reset.
* 429.
* 500.
* 502.
* 503.
* 504.

No retry automático para:

* 400.
* 401.
* 403.
* 404 de la imagen.
* Content-Type inválido.
* imagen demasiado grande.

---

# Autenticación

Todas las llamadas hacia Railway deben enviar:

```http
Authorization: Bearer WORKER_TOKEN
```

No loguear el token.

Nunca imprimir secrets.

---

# Seguridad URLs

El worker recibirá URLs desde Railway.

Implementar validación básica:

Solo aceptar:

```text
http://
https://
```

Preferentemente permitir únicamente HTTPS por defecto.

Crear variable opcional:

```env
ALLOW_HTTP=false
```

Bloquear explícitamente URLs hacia:

```text
localhost
127.0.0.1
::1
0.0.0.0
```

También bloquear rangos privados:

```text
10.0.0.0/8
172.16.0.0/12
192.168.0.0/16
169.254.0.0/16
```

para reducir riesgo SSRF.

Resolver DNS antes de hacer la petición y verificar que no resuelva a IP privada.

No seguir redirects hacia IPs privadas.

---

# HTTP client

Crear HTTP clients reutilizables.

No crear un Transport nuevo para cada request.

Configurar:

* connection pooling.
* keep-alive.
* TLS normal.
* response header timeout.
* idle connection timeout.
* max idle connections razonable.

No desactivar TLS verification.

---

# Logs

Usar `log/slog`.

Ejemplos:

```text
level=INFO msg="worker started" workerId=uy-image-worker-01 concurrency=4
level=INFO msg="job claimed" jobId=job_123
level=INFO msg="image download started" jobId=job_123 host=mayoristas.selvir.com.uy
level=INFO msg="image upload completed" jobId=job_123 bytes=184233 contentType=image/jpeg durationMs=422
level=WARN msg="job retry" jobId=job_123 attempt=2 error="connection reset"
level=ERROR msg="job failed" jobId=job_123 error="image returned HTTP 403"
```

NO loguear:

* WORKER_TOKEN.
* contenido binario.
* headers sensibles.

---

# Métricas internas básicas

Sin agregar Prometheus todavía.

Mantener contadores en memoria:

* jobsClaimed
* jobsCompleted
* jobsFailed
* bytesTransferred
* retries

Cada 60 segundos loguear resumen:

```text
jobsClaimed=...
jobsCompleted=...
jobsFailed=...
bytesTransferred=...
retries=...
```

Usar atomics donde corresponda.

---

# Graceful shutdown

Manejar:

```text
SIGINT
SIGTERM
```

Cuando llegue señal:

1. Dejar de pedir nuevos jobs.
2. Permitir que jobs activos terminen.
3. Esperar máximo 30 segundos.
4. Cerrar conexiones.
5. Salir.

No cortar imágenes activas inmediatamente salvo timeout final.

---

# Healthcheck local

Crear un pequeño servidor HTTP local opcional.

Variables:

```env
HEALTH_PORT=8080
```

Endpoints:

```http
GET /health
```

Respuesta:

```json
{
  "status": "ok",
  "workerId": "uy-image-worker-01"
}
```

También:

```http
GET /stats
```

Ejemplo:

```json
{
  "jobsClaimed": 10,
  "jobsCompleted": 9,
  "jobsFailed": 1,
  "bytesTransferred": 9182736,
  "retries": 2
}
```

Escuchar por defecto en:

```text
127.0.0.1:8080
```

No exponer públicamente salvo configuración explícita.

Variable:

```env
HEALTH_BIND_ADDRESS=127.0.0.1
```

---

# Integridad

Durante el streaming calcular opcionalmente SHA-256 usando `io.TeeReader`.

Enviar al backend cuando sea posible:

```http
X-Image-SHA256: ...
```

Pero tener presente que el hash completo solo se conoce al finalizar el stream.

Si no es viable enviarlo como header antes de comenzar el body, incluirlo en `/complete`.

Ejemplo:

```json
{
  "workerId": "uy-image-worker-01",
  "sha256": "abcdef...",
  "bytes": 184233,
  "contentType": "image/jpeg"
}
```

Preferir esta segunda opción.

---

# Modelo

Definir aproximadamente:

```go
type ImageJob struct {
    ID        string `json:"id"`
    ImageURL  string `json:"imageUrl"`
    ProductID string `json:"productId"`
}
```

Y modelos separados para:

* claim response.
* complete request.
* fail request.

---

# Errores

Crear clasificación clara de errores.

Por ejemplo:

```go
type PermanentError struct {
    Err error
}

type RetryableError struct {
    Err error
}
```

o equivalente.

La lógica de retry debe distinguirlos.

---

# Timeouts

Separar:

```env
DOWNLOAD_TIMEOUT_SECONDS
UPLOAD_TIMEOUT_SECONDS
```

La descarga no debe quedar colgada indefinidamente.

El upload tampoco.

Cada job debe tener su propio `context.Context`.

---

# No usar disco salvo emergencia

La implementación normal debe ser completamente streaming.

No crear:

```text
/tmp/image.jpg
```

No guardar archivos localmente.

Solo si técnicamente fuera imposible resolver un caso específico mediante streaming, documentarlo, pero no implementar fallback a disco por defecto.

---

# Unit tests

Agregar tests suficientes.

Como mínimo:

## Config

* carga variables válidas.
* rechaza configuración incompleta.
* rechaza números inválidos.

## URL validation

* acepta HTTPS pública.
* bloquea localhost.
* bloquea 127.0.0.1.
* bloquea IP privada.
* bloquea esquemas no HTTP.

## Downloader

Usar `httptest.Server`.

Probar:

* JPEG válido.
* PNG válido.
* WebP válido.
* HTTP 404.
* HTTP 500.
* Content-Type inválido.
* contenido vacío.
* imagen excede límite.
* redirect válido.
* redirect inválido hacia destino bloqueado.

## API client

Usar `httptest.Server`.

Probar:

* claim 200.
* claim 204.
* upload exitoso.
* complete exitoso.
* fail exitoso.
* token enviado correctamente.

## Retry

* 500 reintenta.
* timeout reintenta.
* 404 no reintenta.
* 403 no reintenta.
* después del máximo de intentos falla.

## Worker

Probar flujo completo:

```text
claim
→ download
→ upload
→ complete
```

Y:

```text
claim
→ download fail
→ retries
→ fail endpoint
```

---

# Integration test

Crear al menos un integration test con dos `httptest.Server`:

1. Fake image origin.
2. Fake Railway API.

El fake origin devuelve una imagen pequeña.

El fake Railway:

* entrega el job.
* recibe los bytes.
* verifica que sean idénticos.
* responde éxito.
* recibe complete.

Verificar que los bytes recibidos sean exactamente iguales a los enviados por el fake origin.

---

# Makefile

Agregar:

```makefile
build:
	go build -o bin/image-relay-worker ./cmd/worker

test:
	go test ./...

test-race:
	go test -race ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

run:
	go run ./cmd/worker
```

---

# README

Documentar:

* propósito.
* arquitectura.
* instalación.
* variables.
* ejecución.
* endpoints esperados de Railway.
* cómo ejecutar en Windows.
* cómo ejecutar como servicio.
* troubleshooting.
* seguridad.

---

# Windows

La PC objetivo probablemente corre Windows.

El binario debe funcionar directamente en Windows.

Build:

```bash
go build -o image-relay-worker.exe ./cmd/worker
```

Explicar cómo ejecutarlo:

```powershell
$env:API_BASE_URL="..."
$env:WORKER_TOKEN="..."
$env:WORKER_ID="uy-image-worker-01"

.\image-relay-worker.exe
```

---

# Servicio automático en Windows

Agregar documentación para ejecutarlo permanentemente.

Preferir una de estas opciones:

* NSSM.
* Windows Task Scheduler.
* Windows Service wrapper.

NO implementar un Windows Service nativo salvo que sea extremadamente simple.

Documentar NSSM como opción recomendada.

Debe:

* iniciar automáticamente al arrancar Windows.
* reiniciarse si falla.
* ejecutar desde una carpeta fija.

---

# .gitignore

Incluir:

```text
.env
bin/
*.exe
*.log
```

---

# .env.example

Crear completo y usable:

```env
API_BASE_URL=https://example-production-api.up.railway.app
WORKER_TOKEN=change-me
WORKER_ID=uy-image-worker-01

POLL_INTERVAL_SECONDS=5
MAX_CONCURRENT_JOBS=4

DOWNLOAD_TIMEOUT_SECONDS=30
UPLOAD_TIMEOUT_SECONDS=60

MAX_IMAGE_SIZE_MB=25

HTTP_USER_AGENT=ImageRelayWorker/1.0

RETRY_MAX_ATTEMPTS=4
RETRY_BASE_DELAY_MS=1000

ALLOW_HTTP=false

HEALTH_BIND_ADDRESS=127.0.0.1
HEALTH_PORT=8080

LOG_LEVEL=info
```

---

# Requisitos funcionales finales

El resultado debe poder hacer esto:

```text
START
↓
leer config
↓
validar config
↓
iniciar health server
↓
iniciar N workers
↓
claim job
↓
validar URL
↓
descargar imagen
↓
validar status/content-type/tamaño
↓
stream directamente hacia Railway
↓
calcular bytes + sha256
↓
Railway responde upload OK
↓
complete job
↓
volver a claim
```

En error:

```text
error retryable
↓
backoff
↓
volver a descargar
↓
volver a subir
```

Si agota intentos:

```text
/api/image-jobs/{jobId}/fail
```

---

# Importante sobre streaming

NO quiero una implementación que haga:

```go
io.ReadAll(response.Body)
```

para imágenes normales.

NO quiero:

```go
[]byte
```

con la imagen completa en memoria.

NO quiero Base64.

La ruta normal tiene que usar streams.

Ejemplo conceptual:

```go
resp, err := client.Do(downloadReq)

uploadReq, err := http.NewRequestWithContext(
    ctx,
    http.MethodPost,
    uploadURL,
    resp.Body,
)
```

Puede ser necesario envolver el reader para:

* contar bytes.
* calcular SHA-256.
* limitar tamaño.
* detectar Content-Type.

Usar:

```go
io.Reader
io.LimitReader
io.TeeReader
io.MultiReader
```

según corresponda.

---

# Atención al tamaño desconocido

Si `Content-Length` existe y supera MAX_IMAGE_SIZE_MB:

rechazar inmediatamente.

Si no existe:

limitar el reader a:

```text
maxBytes + 1
```

Si se transfieren más de `maxBytes`:

cancelar y marcar error permanente.

---

# Observabilidad

Cada job debe tener logs con:

```text
jobId
productId
sourceHost
attempt
duration
bytes
contentType
```

Nunca imprimir la URL completa si contiene query params potencialmente sensibles.

Podés loguear:

```text
scheme + host + path
```

sin querystring.

---

# Resultado esperado del trabajo

Quiero que Codex:

1. Cree todos los archivos.
2. Implemente toda la lógica.
3. Implemente tests.
4. Ejecute:

```bash
go fmt ./...
go vet ./...
go test ./...
go test -race ./...
go build ./cmd/worker
```

5. Corrija cualquier error encontrado.
6. Deje el proyecto en estado compilable y funcional.

NO quiero pseudocódigo.

NO quiero solamente arquitectura.

NO quiero TODOs críticos.

NO quiero funciones vacías.

NO quiero que me preguntes decisiones adicionales.

Tomá decisiones razonables cuando haya ambigüedad.

Al finalizar entregame únicamente:

* resumen de arquitectura implementada.
* estructura de archivos.
* variables de entorno.
* resultado de fmt.
* resultado de vet.
* resultado de tests.
* resultado de race tests.
* resultado de build.
* riesgos pendientes reales.
* comando exacto para ejecutarlo en Windows.

No hagas deploy.

No conectes con producción.

No uses credenciales reales.

No hagas llamadas a URLs reales durante tests.

Todo debe quedar listo para que después solo configuremos:

```text
API_BASE_URL
WORKER_TOKEN
WORKER_ID
```

y lo ejecutemos.

:w!

