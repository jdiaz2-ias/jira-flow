# Compatibilidad

Go 1.27.1. Objetivos: Linux/macOS, amd64/arm64, `CGO_ENABLED=0`. `make build-all` compila las cuatro combinaciones.

F0 se ejecutó en Linux amd64; F1 se valida localmente en macOS arm64. Compilar para otra plataforma no prueba su ejecución. CI define Ubuntu 24.04 y macOS 15.

## Credenciales del sistema

macOS: `/usr/bin/security` y Keychain desbloqueado. Prueba real del backend F1:

```bash
JFLOW_TEST_KEYCHAIN=1 go test ./internal/secretstore -run TestNativeKeychainRoundTrip -count=1
```

Crea y elimina exclusivamente una referencia aleatoria con secreto sintético bajo `jflow`. No requiere credenciales Jira. Pasó en el equipo macOS arm64 de F1.

Linux: ejecutable `secret-tool` (libsecret-tools) y sesión D-Bus con Secret Service. Ejecución real pendiente; cobertura con ejecutor inyectado. Si el almacén está ausente o bloqueado, la CLI devuelve un error y permite reintentar con `--no-store` y un token efímero. No hay fallback de archivo de texto plano.

La prueba F0 bajo tag `foundation` permanece como auditoría de la biblioteca fijada; el backend activo F1 usa procesos cancelables propios.

Informes: [F0](validation-f0.md), [F1](validation-f1.md).
