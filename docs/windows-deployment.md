# Compilar y ejecutar ImageRelayWorker en Windows

Esta guía explica cómo generar el `.exe`, copiarlo a la PC de Uruguay, configurarlo y ejecutarlo como proceso o servicio de Windows.

## Opción recomendada: descargar un Release

La forma más simple para el operador es descargar el ejecutable desde la sección
[Releases de GitHub](https://github.com/FranprzDev/ImageRelayWorker/releases).

Al publicar un tag con formato `vX.Y.Z`, GitHub Actions automáticamente:

1. ejecuta `fmt`, `vet`, tests y race tests;
2. compila `image-relay-worker-windows-amd64.exe`;
3. genera su checksum SHA-256;
4. publica ambos archivos en un GitHub Release.

Para crear una versión nueva, desde un checkout autorizado:

```bash
git fetch origin main
git switch main
git pull --ff-only origin main
git tag v1.0.0
git push origin v1.0.0
```

Después de que termine el workflow, descargar:

```text
image-relay-worker-windows-amd64.exe
image-relay-worker-windows-amd64.exe.sha256
```

En Windows 10 y Windows 11 de 64 bits se utiliza el mismo ejecutable. No hace falta
instalar Go en la PC operativa.

Verificación opcional del checksum en PowerShell:

```powershell
Get-FileHash .\image-relay-worker-windows-amd64.exe -Algorithm SHA256
Get-Content .\image-relay-worker-windows-amd64.exe.sha256
```

El Release contiene únicamente el binario y el checksum. Las variables de entorno,
especialmente `WORKER_TOKEN`, nunca se incluyen en el Release.

## 1. Requisitos

Para compilar desde macOS o Linux:

- Go 1.22 o superior.
- El repositorio `ImageRelayWorker` clonado localmente.
- Acceso al contrato de la API de Railway.

La PC Windows objetivo normalmente será Intel/AMD de 64 bits. Para ese caso se utiliza `GOARCH=amd64`.

## 2. Compilar el `.exe`

Ejecutar desde la raíz del proyecto, donde se encuentra `go.mod`:

```bash
mkdir -p bin
GOOS=windows GOARCH=amd64 go build -o bin/image-relay-worker.exe ./cmd/worker
```

Verificar que el binario fue creado:

```bash
ls -lh bin/image-relay-worker.exe
```

El punto de entrada es `./cmd/worker`. No se debe compilar solamente `internal/`, porque esos directorios son paquetes internos utilizados por el ejecutable.

## 3. Validar antes de transferirlo

Desde el repositorio ejecutar:

```bash
go fmt ./...
go vet ./...
go test ./...
go test -race ./...
go build ./cmd/worker
```

No usar credenciales reales para los tests. Las pruebas del proyecto utilizan servidores HTTP locales simulados.

## 4. Copiar archivos a Windows

Crear una carpeta fija, por ejemplo:

```text
C:\ImageRelayWorker\
├── image-relay-worker.exe
└── logs\
```

Copiar `bin/image-relay-worker.exe` desde la máquina de desarrollo a esa carpeta usando la red local, un pendrive o el mecanismo autorizado por el operador.

## 5. Configurar variables de entorno

Abrir PowerShell. Para una prueba temporal en la sesión actual:

```powershell
$env:API_BASE_URL="https://example-production-api.up.railway.app"
$env:WORKER_TOKEN="REEMPLAZAR_CON_EL_TOKEN"
$env:WORKER_ID="uy-image-worker-01"

$env:POLL_INTERVAL_SECONDS="5"
$env:MAX_CONCURRENT_JOBS="4"
$env:DOWNLOAD_TIMEOUT_SECONDS="30"
$env:UPLOAD_TIMEOUT_SECONDS="60"
$env:MAX_IMAGE_SIZE_MB="25"
$env:HTTP_USER_AGENT="ImageRelayWorker/1.0"
$env:RETRY_MAX_ATTEMPTS="4"
$env:RETRY_BASE_DELAY_MS="1000"
$env:ALLOW_HTTP="false"
$env:HEALTH_BIND_ADDRESS="127.0.0.1"
$env:HEALTH_PORT="8080"
$env:LOG_LEVEL="info"
```

No guardar el token en el repositorio, en `.env.example`, en capturas de pantalla ni en logs.

## 6. Ejecutar manualmente

Desde `C:\ImageRelayWorker`:

```powershell
.\image-relay-worker.exe
```

El proceso debe iniciar con un log similar a:

```text
worker started workerId=uy-image-worker-01 concurrency=4
```

La configuración por defecto expone el healthcheck solo localmente:

```text
http://127.0.0.1:8080/health
http://127.0.0.1:8080/stats
```

Desde otra ventana de PowerShell:

```powershell
Invoke-WebRequest http://127.0.0.1:8080/health
Invoke-WebRequest http://127.0.0.1:8080/stats
```

## 7. Ejecutarlo como servicio con NSSM

NSSM permite mantener el proceso activo, iniciarlo con Windows y reiniciarlo si falla.

1. Descargar e instalar NSSM desde una fuente aprobada.
2. Colocar `nssm.exe` en una carpeta disponible en `PATH` o usar su ruta completa.
3. Crear el servicio:

```powershell
nssm install ImageRelayWorker C:\ImageRelayWorker\image-relay-worker.exe
nssm set ImageRelayWorker AppDirectory C:\ImageRelayWorker
nssm set ImageRelayWorker Start SERVICE_AUTO_START
nssm start ImageRelayWorker
```

4. En la interfaz de NSSM, abrir **Environment** y agregar las variables de la sección anterior.
5. Configurar, si se desea, archivos de salida y error en las pestañas **I/O**.

Comandos útiles:

```powershell
nssm status ImageRelayWorker
nssm restart ImageRelayWorker
nssm stop ImageRelayWorker
nssm remove ImageRelayWorker confirm
```

El token debe configurarse en el entorno del servicio de NSSM, no como argumento visible del proceso.

## 8. Prueba controlada de punta a punta

Antes de dejarlo operativo:

1. Confirmar que `API_BASE_URL` apunta al entorno correcto.
2. Confirmar que `WORKER_TOKEN` es válido y pertenece al worker autorizado.
3. Crear o habilitar un único job de prueba.
4. Verificar que el worker lo reclama.
5. Verificar que Railway recibe los bytes binarios sin Base64.
6. Verificar que se ejecuta `complete`.
7. Verificar que la imagen queda persistida correctamente en Railway/S3.
8. Revisar `/health`, `/stats` y los logs.
9. Confirmar que no se generaron archivos temporales con imágenes en `C:\ImageRelayWorker`.

No comenzar con una cola grande de trabajos. Primero validar un caso exitoso y luego un caso controlado de error o retry.

## 9. Troubleshooting

### El proceso termina inmediatamente

Revisar:

- `API_BASE_URL` configurada.
- `WORKER_TOKEN` configurado.
- `WORKER_ID` configurado.
- Que los valores numéricos sean positivos.
- Que la terminal tenga permisos para ejecutar el `.exe`.

### No responde `/health`

Verificar que el proceso esté ejecutándose y que el puerto `8080` no esté ocupado:

```powershell
Get-NetTCPConnection -LocalPort 8080 -ErrorAction SilentlyContinue
```

### No reclama trabajos

Revisar:

- URL base y endpoint de Railway.
- Token y permisos.
- Que existan jobs pendientes.
- Conectividad HTTPS desde la PC uruguaya.
- Logs de respuestas `401`, `403`, `429` o `5xx`.

### La descarga es rechazada

El worker rechaza URLs no HTTPS por defecto, hosts locales, loopback, rangos privados, contenido no imagen y archivos que superen `MAX_IMAGE_SIZE_MB`.

No activar `ALLOW_HTTP=true` salvo para una prueba controlada y autorizada.

## 10. Actualizar el worker

Para actualizarlo:

1. Detener el servicio:

```powershell
nssm stop ImageRelayWorker
```

2. Respaldar el `.exe` actual.
3. Copiar el nuevo `image-relay-worker.exe`.
4. Iniciar el servicio:

```powershell
nssm start ImageRelayWorker
```

5. Ejecutar nuevamente el healthcheck y una prueba controlada.

No sobrescribir el ejecutable mientras el servicio está activo.
