# Validación F1

Fecha: 2026-09-07. Equipo local: macOS arm64. Toolchain: Go 1.27.1.

## Alcance

Perfiles JSON v1; rutas Linux/macOS; escritura atómica y bloqueo entre procesos; autenticación personal Cloud con y sin scopes; credenciales persistentes/efímeras; `auth login/logout/status`, `profile list/use`, `me`, `doctor`, `config path/validate`.

Las pruebas habituales usan credenciales sintéticas, servidor HTTPS `httptest` y almacenes inyectados. No contactan un tenant Jira ni leen credenciales reales. La prueba opt-in de macOS crea un ítem aleatorio sintético bajo el servicio `jflow` y lo elimina.

## Evidencia

- Prueba real `TestNativeKeychainRoundTrip`: pasa en macOS arm64; escritura por stdin, lectura, eliminación y ausencia posterior. Incluye comillas, barra inversa y Unicode.
- `make check`: pasa (formato, vet, pruebas y auditoría foundation).
- `make test-race`: pasa en macOS arm64.
- `make build` y `make build-all`: pasan; cuatro destinos Linux/macOS × amd64/arm64.
- `go mod verify`: todos los módulos verificados; `go mod tidy` no altera go.mod/go.sum.
- `make security`: govulncheck 1.7.0 no encontró vulnerabilidades.
- Binario local: ayuda general y ayuda anidada JSON comprobadas; contrato de proceso cubierto por pruebas.
- GitHub CI: resultado remoto se comprobará al publicar la rama; estos resultados son locales.

## Casos cubiertos

- URLs HTTPS, cloud ID explícito, ambos destinos REST y autenticación Basic.
- 401, 403, 404, 429, 5xx; redirecciones rechazadas sin segunda solicitud.
- Cancelación, respuesta inválida/excesiva e identidad saneada para terminal.
- Precedencia de perfil/correo, identidad distinta rechazada, separación de credenciales y logout por perfil.
- Token de entorno efímero; token stdin; JSON único; códigos de salida; fallos de escritura; ayuda anidada.
- Llavero bloqueado, token excesivo antes de iniciar proceso, secreto ausente de argv y errores.
- Permisos privados, actualizaciones concurrentes, bloqueo cancelable y configuración inválida sin sobrescritura.

## Límites

- Ningún tenant Jira real fue configurado ni consultado. La selección de scopes en la consola Atlassian debe comprobarse con el tenant autorizado; `myself` no demuestra permisos para consultar o cambiar issues.
- Secret Service Linux está probado mediante ejecutor inyectado y compilación; su ejecución real requiere `secret-tool` (paquete habitual `libsecret-tools`) y una sesión D-Bus con Secret Service desbloqueado.
- macOS usa `/usr/bin/security` directamente con contexto y timeout. El token se codifica en hexadecimal para evitar problemas de comillas del modo interactivo; sigue protegido por Keychain. No es compatible con entradas manuales de texto plano: las referencias son opacas y generadas por esta versión.
- Persistencia limitada a 1500 bytes por token; tokens mayores pueden usarse de forma efímera. `--token-stdin` admite hasta 64 KiB y una sola línea.
- V1 es el primer esquema persistido: F0 no generaba configuración. Versiones ausentes, anteriores o futuras se rechazan sin modificar el archivo. No hay un formato histórico real que migrar; una futura migración deberá definir su conversión y respaldo.
- La configuración implementa el subconjunto de acceso de F1. Los campos de UI, caché y workflows del ejemplo del plan pertenecen a fases posteriores y todavía se rechazan como desconocidos.
- F1 no persiste caché de issues. Logout elimina el perfil y las credenciales locales, incluidas referencias pendientes de limpieza; no revoca el token en Atlassian ni modifica el entorno del shell.
