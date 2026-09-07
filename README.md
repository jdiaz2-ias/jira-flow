# Jira Flow

Una CLI para trabajar con Jira desde Linux y macOS. Ejecutable: `jflow`.

Estado actual: **F1, configuración y acceso**. Incluye perfiles, autenticación personal Jira Cloud, `me` y `doctor`. Consultas de issues y enlaces corresponden a F2. Consulta la [validación F1](docs/validation-f1.md) para resultados y límites.

## Inicio rápido

Requisitos: Go 1.27.1, Git y Make. GCC/Clang es necesario para `make test-race`; los binarios normales se construyen con `CGO_ENABLED=0`.

```bash
cd jira-flow
go mod download
make build
./bin/jflow --help
./bin/jflow version
./bin/jflow version --format json
```

## Conectar una cuenta Jira

```bash
./bin/jflow auth login --profile trabajo \
  --site https://empresa.atlassian.net \
  --email persona@example.com
./bin/jflow me
./bin/jflow doctor --format=json
./bin/jflow profile list
```

El login solicita el token con entrada oculta y valida la identidad antes de guardar. Para un token con scopes añade `--method api-token-scoped --cloud-id ID_REAL`. El cloud ID debe proporcionarse explícitamente; no se infiere del hostname.

En macOS se utiliza Keychain; en Linux se necesita `secret-tool` y Secret Service. Para automatización, `--token-stdin` lee una sola línea y `--no-store` evita persistirla. `JFLOW_TOKEN` siempre es efímero: vuelve a proporcionarlo en cada invocación. `me` y `doctor` también aceptan `--token-stdin`. No pases el token como argumento.

La configuración se guarda en `~/Library/Application Support/jflow/config.json` en macOS y `$XDG_CONFIG_HOME/jflow/config.json` o `~/.config/jflow/config.json` en Linux. `JFLOW_CONFIG` permite otra ruta. `jflow config path` muestra la ruta efectiva; `jflow config validate` valida el archivo sin mostrar secretos. Los archivos se crean con permisos `0600` y los directorios nuevos con `0700`.

`--profile` > `JFLOW_PROFILE` > perfil activo. `--email` > `JFLOW_EMAIL` > correo del perfil > correo global. `JFLOW_CA_CERT` añade una CA PEM a las raíces del sistema; el transporte respeta el proxy del entorno.

`auth status` comprueba la fuente local de credenciales; `me` y `doctor` las verifican con Jira. `auth logout --profile trabajo` elimina ese perfil y sus credenciales locales; no revoca el token en Atlassian. Una variable `JFLOW_TOKEN` sigue existiendo en el shell hasta que la retires.

Más detalles: [comandos](docs/commands.md), [autenticación](docs/authentication.md), [compatibilidad](docs/compatibility.md).

Ejemplo de salida:

```text
jflow dev
commit: unknown
Go: go1.27.1
plataforma: linux/amd64
```

El commit refleja Git cuando se compila con Make. `go run ./cmd/jflow version` conserva los valores de desarrollo.

## Comandos disponibles

| Comando | Resultado |
| --- | --- |
| `jflow --help` | Ayuda de los comandos implementados |
| `jflow help version` | Ayuda de versión |
| `jflow version` | Versión, commit, Go y plataforma |
| `jflow version --format=json` | Mismos datos en el contrato JSON v1 |
| `jflow --help --format=json` | Ayuda dentro de un único documento JSON |
| `jflow auth login/logout/status` | Autenticación y credenciales locales |
| `jflow profile list/use` | Perfiles y selección del activo |
| `jflow me` | Identidad autenticada en Jira |
| `jflow doctor` | Diagnóstico de configuración y conexión |
| `jflow config path/validate` | Ruta y validación de configuración |

Sin comando se devuelve código 2. `mine` llegará en F2. Ayuda, versión y listado de perfiles no requieren conexión a Jira; `auth login`, `me` y `doctor` sí validan identidad por red.

## Desarrollo y verificación

```bash
make check            # Formato, vet, pruebas y auditoría de dependencias
make test-race        # Detector de carreras
make build-all        # Linux/macOS × amd64/arm64, incluidos imports futuros
make security         # govulncheck fijado; necesita red la primera vez
make release-check   # Verificación de builds, no publica artefactos
```

`make build` produce `bin/jflow`. `make build-all` produce `bin/{linux,darwin}-{amd64,arm64}/jflow`. Estos directorios están ignorados por Git. La compilación cruzada no acredita ejecución en el sistema destino; consulta [compatibilidad y validación](docs/compatibility.md).

## Arquitectura

Go + Cobra para la CLI; Bubble Tea/Bubbles/Lip Gloss v2 están fijados y se comprueban con el build tag `foundation`. La TUI se implementará en F5; F1 ya integra el llavero del sistema.

```text
cmd/jflow       Entrada del proceso y señales
internal/cli    Argumentos, ayuda y códigos de salida
internal/app    Casos de uso de acceso y metadatos de versión
internal/config Rutas, perfiles y persistencia JSON
internal/provider/jiracloud Cliente HTTPS y mapeo de identidad
internal/secretstore Llavero cancelable macOS/Linux
internal/domain Issues, progreso, workflows y errores
internal/ports  Fronteras para Jira y almacenamiento de secretos
internal/output Contrato JSON público independiente del dominio
internal/foundation Compatibilidad de dependencias y revisión keyring
tests/integration   Pruebas del binario y separación de capas
testdata/jiracloud   Respuestas sintéticas para próximos adaptadores
docs/adr        Decisiones de arquitectura
```

El módulo `jira-flow.local/jflow` es deliberadamente local y provisional. El repositorio remoto es `github.com/jdiaz2-ias/jira-flow`. Cambiar el módulo antes de distribuir mediante `go install`; la licencia de distribución sigue pendiente de decisión.

## Continuar la implementación

- [Plan de implementación completo](docs/implementation-plan.md).
- [Estado de fases y siguiente incremento F2](docs/roadmap.md).
- [Arquitectura y versiones fijadas](docs/architecture.md).
- [Autenticación y catálogo de endpoints/scopes](docs/authentication.md).
- [Contrato JSON y errores](docs/json-contract.md).
- [Decisiones de workflows](docs/workflows.md).
- [Registro de validación F0](docs/validation-f0.md).
- [Registro de validación F1](docs/validation-f1.md).

La siguiente fase es F2: consultas de issues y enlaces (`mine`, `search`, `show`, `link`, `open`). La prueba contra Jira real de F1 queda pendiente de un tenant autorizado.
