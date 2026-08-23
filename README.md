# ImageRelayWorker
Downloader/relay independiente en Go: reclama trabajos a Railway, descarga imágenes con streaming limitado y las sube sin Base64 ni disco local.

## Uso
Requiere Go 1.22+. Copiar `.env.example` como referencia y exportar las variables en el entorno (el binario no carga archivos automáticamente):

```powershell
$env:API_BASE_URL="https://example-production-api.up.railway.app"
$env:WORKER_TOKEN="..."
$env:WORKER_ID="uy-image-worker-01"
go build -o bin/image-relay-worker.exe ./cmd/worker
.\bin\image-relay-worker.exe
```

Todas las URLs de imágenes deben ser HTTPS y se bloquean localhost, loopback y rangos privados para reducir SSRF. `ALLOW_HTTP=true` solo para pruebas controladas. El token nunca se registra. Health local: `GET /health` y `/stats`.

La API debe implementar `POST /api/image-jobs/claim`, `POST /api/image-jobs/{id}/upload`, `/complete` y `/fail` según `docs/worker-implementation.md`.

## Servicio en Windows

Para una instalación permanente, colocar el binario en una carpeta fija y configurar **NSSM**:

```powershell
nssm install ImageRelayWorker C:\ImageRelayWorker\bin\image-relay-worker.exe
nssm set ImageRelayWorker AppDirectory C:\ImageRelayWorker
nssm set ImageRelayWorker Start SERVICE_AUTO_START
nssm start ImageRelayWorker
```

Configurar las variables de entorno desde la pestaña **Environment** de NSSM y revisar los eventos del servicio si el proceso no inicia. El endpoint local queda disponible en `127.0.0.1:8080` por defecto.

## Validación
`make fmt && make vet && make test && make test-race && make build`.
