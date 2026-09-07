# Compatibilidad

Toolchain fijado: Go 1.27.1. Objetivos: Linux/macOS, amd64/arm64, `CGO_ENABLED=0` para binarios. `make build-all` compila la CLI y las dependencias futuras con tag `foundation` para cada objetivo.

La ejecución local se realiza en Linux amd64. Compilar darwin o arm64 no prueba su comportamiento en esos equipos. CI está preparada con Ubuntu 24.04 y macOS 15; no se ha ejecutado remotamente al crear el repositorio.

## Keychain macOS

La prueba de lectura/escritura real es opt-in y usa un nombre único de recurso con contenido sintético. Solo ejecutarla en un Mac con llavero desbloqueado donde se acepte crear/eliminar ese ítem de prueba:

```bash
JFLOW_TEST_KEYCHAIN=1 go test -tags=foundation ./internal/foundation -run TestMacKeychainRoundTrip -count=1
```

La limpieza se limita al ítem que creó la prueba. Esta prueba no valida navegación, TUI ni un tenant Jira. Las pruebas ordinarias y CI no invocan un keyring real.

Ver el [informe F0](validation-f0.md) para resultados reales, pendientes y comandos de reproducción.
