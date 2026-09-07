# Validación F0

Fecha: 2026-09-06. Entorno de ejecución: Linux Mint 22.3, amd64. Toolchain: Go 1.27.1 instalado en el directorio local del usuario, sin modificar archivos de inicio del shell. El archivo oficial se verificó por SHA-256 antes de extraerlo.

## Resultados registrados

| Comprobación | Resultado |
| --- | --- |
| `make check` | Pasa: formato, vet, pruebas CLI/contratos/proceso/capas y auditoría keyring |
| `make test-race` | Pasa en Linux amd64; la prueba de CLI se repitió tras su ajuste final |
| `go mod verify` | Todos los módulos verificados |
| `make build` | Produce `bin/jflow`, ejecutado en este equipo |
| `make build-all` | Compila las cuatro plataformas, tanto CLI como imports `foundation` |
| `jflow --help` | Solo muestra ayuda y versión como comandos actuales |
| `jflow version --format=json` | JSON v1 válido con metadatos del binario |
| govulncheck 1.7.0 | Sin vulnerabilidades encontradas; ejecutado también con `-tags=foundation` |
| Fixtures | Nueve JSON sintéticos parseables |

Se ejecutaron diez funciones de prueba en Linux, una de ellas con diecisiete casos de argumentos/salida. Las comprobaciones incluyen un proceso real de `jflow`, estados de salida, un solo documento JSON, redacción de secretos, fallos de escritura y ausencia de imports de UI/proveedor en el núcleo.

La primera ejecución de la auditoría de fuente necesitó completar metadatos del módulo mediante `go mod download`; después pasó sin llamadas al keyring. Los resultados de seguridad corresponden a la base consultada en esta fecha, no a una garantía futura.

## Matriz observada

| Objetivo | Compilación CLI + foundation | Ejecución del binario | Keyring real |
| --- | --- | --- | --- |
| Linux amd64 | Sí | Sí | No utilizado en F0 |
| Linux arm64 | Sí | No | No |
| macOS amd64 | Sí | No | No |
| macOS arm64 | Sí | No | No |

`file` identifica los dos binarios Linux como ELF estáticos y los de macOS como Mach-O de su arquitectura. CGO está desactivado para estos builds; macOS sigue dependiendo de componentes del sistema operativo.

## Límites explícitos

- El workflow de CI está creado, pero no se ha ejecutado en GitHub: el repositorio es local.
- La viabilidad de SecretStore macOS se revisó en fuente y mediante una prueba estructural. La prueba opt-in real queda preparada para un Mac con keychain desbloqueado.
- No hay tenant Jira ni credenciales configuradas; F0 no envía solicitudes Jira.
- Autenticación, TUI, caché, workflows ejecutables y empaquetado de releases pertenecen a fases posteriores.

## Reproducción

```bash
export PATH="$HOME/.local/share/jira-flow/toolchains/go1.27.1/bin:$PATH"
go mod download
make check test-race build-all build
make security
./bin/jflow --help
./bin/jflow version --format=json
```

En otros equipos, instalar la versión fijada de Go y omitir el export específico de esta instalación.
