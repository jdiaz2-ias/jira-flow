# Arquitectura F2

El dominio y los puertos dependen únicamente de Go estándar. CLI y futura TUI consumen casos de uso; los adaptadores implementan los puertos. La prueba de integración `TestCoreHasNoUIOrProviderDependencies` verifica esa separación con el grafo real de imports.

Tipos de dominio presentes: issues, identidad, fechas locales, bloques normalizados, consulta/página, progreso, campos de transición, intención, acción preparada, resultado de aplicación y errores. F1 añade `config` para persistencia versionada, `app.Access` para casos de uso de acceso, `provider/jiracloud` para HTTP y `secretstore` para credenciales del sistema. CLI compone las dependencias y las pruebas pueden inyectarlas.

`Secret` redacta formato, JSON y slog; `Reveal` es una salida explícita para adaptadores de autenticación/almacenamiento. La redacción evita errores de registro habituales, no cifra memoria ni garantiza borrado de strings Go.

## Dependencias fijadas

| Componente | Versión inicial | Uso |
| --- | --- | --- |
| Go | 1.27.1 | Compilador, gofmt y go vet |
| Cobra | 1.10.2 | CLI |
| Bubble Tea | 2.0.9 | Compatibilidad F0; TUI en F5 |
| Bubbles | 2.2.1 | Componentes de TUI |
| Lip Gloss | 2.0.6 | Estilos de TUI |
| go-keyring | 0.2.8 | Auditoría histórica foundation; backend activo con procesos propios |
| x/term | 0.45.0 | Entrada oculta/detección de terminal en F1 |
| govulncheck | 1.7.0 | Análisis ejecutado por `make security` |
| GoReleaser | 2.18.1 | Versión reservada en Makefile; empaquetado en F6 |

`go.mod` y `go.sum` son la autoridad sobre el grafo realmente resuelto, incluidas dependencias transitivas. `foundation` mantiene presentes los imports de dependencias futuras al ejecutar `go mod tidy`, y permite compilarlas en la matriz sin incorporar funcionalidad prematura al binario. Es una excepción deliberada de F0: retirar esos imports cuando los adaptadores/TUI reales los utilicen.

CI fija checkout/setup-go por SHA, usa versiones explícitas de SO/toolchain y no publica. El lint inicial es `go vet`, fijado por Go; añadir otro linter solo cuando aporte reglas necesarias.

## Lectura F2

`jiracloud.Session` implementa `ports.IssueReader` con un perfil y una credencial por invocación. `app.Reader` verifica la identidad, recorre páginas, firma cursores y mantiene una caché aislada. `output` convierte dominio a DTOs públicos y texto; no exporta directamente structs internos al contrato JSON. `browser` implementa el puerto de apertura con ejecutables y argumentos separados.

Los listados no cargan detalles por fila. Las secciones de comentarios/historial usan sus propios offsets. La caché vive solo en memoria; la persistencia se reserva para F4.

## Organización futura

Ampliar `provider/jiracloud` y `workflow` en F3; `cache` persistente en F4; `tui` en F5. No crear paquetes vacíos para fingir implementación. El formato público se mantendrá en `output` incluso cuando cambien los tipos internos.

Referencias consultadas el 2026-09-06: [Go](https://go.dev/dl/?mode=json), [Cobra](https://github.com/spf13/cobra/releases/tag/v1.10.2), [Charm](https://charm.land/blog/v2/), [keyring](https://github.com/zalando/go-keyring/tree/v0.2.8).
