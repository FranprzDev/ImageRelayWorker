# Política de releases

## Distribución

La única ruta oficial de distribución es GitHub Releases. No se deben compilar ni
distribuir ejecutables manualmente desde una máquina de desarrollo.

Cada release se identifica con [Semantic Versioning](https://semver.org/):

```text
vMAJOR.MINOR.PATCH
```

Ejemplo:

```bash
git tag v1.0.0
git push origin v1.0.0
```

El workflow publica el ejecutable y su checksum como assets inmutables del release.
El tag debe crearse sobre `main` después de mergear el PR correspondiente.

## Rollback

Si una versión presenta problemas:

1. detener el servicio en la PC Windows;
2. descargar el `.exe` del release estable anterior;
3. verificar su checksum;
4. reemplazar el ejecutable;
5. iniciar el servicio y verificar `/health`;
6. ejecutar un job controlado.

No se debe mover ni sobrescribir un asset ya publicado. Para una corrección se crea
una nueva versión `PATCH`, por ejemplo `v1.0.1`.

## Firma de código

La firma Authenticode del `.exe` queda como siguiente mejora operativa. Para activarla
se necesita un certificado de firma y configurar en GitHub Actions, como secretos
protegidos:

- certificado exportado en formato aprobado por la organización;
- contraseña del certificado;
- identidad del firmante.

El certificado y la contraseña nunca deben entrar al repositorio, al binario ni a los
logs. Hasta configurar esa infraestructura, verificar siempre el checksum SHA-256 y
descargar exclusivamente desde el repositorio oficial.
