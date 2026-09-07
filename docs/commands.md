# Contrato CLI de F0

`jflow version` produce texto sin colores ni acceso a red. `--format=json` produce un solo objeto con `schema_version`, `ok`, `data`, `meta`, `warnings` y `error`. La opción puede ir antes o después del subcomando.

`jflow --help`, `jflow help` y `jflow help version` muestran solo los comandos actuales. Con formato JSON, la ayuda está en `data.help`. No hay completado instalado ni TUI automática todavía.

Reglas:

- Sin subcomando: código 2 y diagnóstico de uso; en JSON es un error estructurado.
- Comando desconocido, argumentos extra o flags inválidos: código 2.
- Solo se admite `plain` o `json`; `table` aparecerá con colecciones en F2.
- `--` termina las opciones. Lo que sigue es un argumento posicional.
- Se respeta la última aparición de `--format`.
- Errores de escritura no se reportan como éxito: código 1.
- Cancelación de un comando: código 130.
- No se imprimen argumentos arbitrarios en diagnósticos de parseo.

La ausencia de TUI en F0 es temporal y explícita. Los comportamientos finales del plan se incorporarán conforme se implementen sus funciones, conservando scripts de `version`.
