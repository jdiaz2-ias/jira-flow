# Jira Flow

Una CLI para trabajar con Jira desde Linux y macOS. Ejecutable: `jflow`.

Estado actual: **F0, base técnica**. Puedes compilar el proyecto, consultar la versión y utilizar la ayuda en texto o JSON. Esta fase prepara la arquitectura, dependencias, contratos, pruebas y CI. La autenticación, las consultas de issues y la interfaz interactiva se implementarán en las siguientes fases.

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

En el equipo donde se creó el proyecto, Go está instalado localmente. Para usarlo en la sesión actual:

```bash
export PATH="$HOME/.local/share/jira-flow/toolchains/go1.27.1/bin:$PATH"
cd "$HOME/Documentos/jira-flow"
make build
./bin/jflow version
```

Ese `export` solo cambia la sesión actual; no es necesario modificar el perfil del shell. También puedes ejecutar `bin/jflow` directamente sin Go después de compilar.

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

Sin comando se devuelve código 2. Un comando futuro, como `mine` o `auth`, todavía no está registrado y no simula una conexión. La CLI no necesita credenciales ni accede a Jira en F0.

## Desarrollo y verificación

```bash
make check            # Formato, vet, pruebas y auditoría de dependencias
make test-race        # Detector de carreras
make build-all        # Linux/macOS × amd64/arm64, incluidos imports futuros
make security         # govulncheck fijado; necesita red la primera vez
make release-check   # Verificación de builds F0, no publica artefactos
```

`make build` produce `bin/jflow`. `make build-all` produce `bin/{linux,darwin}-{amd64,arm64}/jflow`. Estos directorios están ignorados por Git. La compilación cruzada no acredita ejecución en el sistema destino; consulta [compatibilidad y validación](docs/compatibility.md).

## Arquitectura

Go + Cobra para la CLI; Bubble Tea/Bubbles/Lip Gloss v2 están fijados y se comprueban con el build tag `foundation`. El binario F0 no carga una TUI ni accede al keyring.

```text
cmd/jflow       Entrada del proceso y señales
internal/cli    Argumentos, ayuda y códigos de salida
internal/app    Casos de uso; actualmente metadatos de versión
internal/domain Issues, progreso, workflows y errores
internal/ports  Fronteras para Jira y almacenamiento de secretos
internal/output Contrato JSON público independiente del dominio
internal/foundation Compatibilidad de dependencias y revisión keyring
tests/integration   Pruebas del binario y separación de capas
testdata/jiracloud   Respuestas sintéticas para próximos adaptadores
docs/adr        Decisiones de arquitectura
```

El módulo `jira-flow.local/jflow` es deliberadamente local y provisional. Cambiarlo por el propietario real antes de distribuir mediante `go install`; no se ha creado ningún repositorio remoto. La licencia de distribución se decidirá antes de publicar.

## Continuar la implementación

- [Plan de implementación completo](docs/implementation-plan.md).
- [Estado de fases y siguiente incremento F1](docs/roadmap.md).
- [Arquitectura y versiones fijadas](docs/architecture.md).
- [Autenticación prevista y catálogo de endpoints/scopes](docs/authentication.md).
- [Contrato JSON y errores](docs/json-contract.md).
- [Decisiones de workflows](docs/workflows.md).
- [Registro de validación F0](docs/validation-f0.md).

La siguiente fase es configuración/perfiles, credenciales y `auth login`/`me`/`doctor`. El detalle está documentado sin exigir un tenant real para empezar con fixtures.
