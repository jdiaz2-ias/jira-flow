# JSON público v1

La envoltura se implementa en `internal/output`; no se serializan directamente structs de dominio. Stdout contiene un único JSON y salto de línea. En errores de uso con JSON, stderr permanece vacío. Un fallo del propio stdout produce diagnóstico por stderr y código 1.

```json
{
  "schema_version": 1,
  "ok": true,
  "data": {
    "version": "dev",
    "commit": "unknown",
    "go_version": "go1.27.1",
    "os": "linux",
    "arch": "amd64"
  },
  "meta": {},
  "warnings": [],
  "error": null
}
```

En un error, `ok=false`, `data=null`, y `error` contiene `code`, `message`, `retryable` y `details`. `Cause` interno nunca se exporta. Errores sin clasificar se presentan como `internal_error`, sin copiar su contenido privado. Ayuda JSON utiliza `data.help`.

## Códigos reservados

| Exit | Código de dominio | Uso |
| --- | --- | --- |
| 0 | — | Éxito |
| 1 | `internal_error` | Error interno/salida |
| 2 | `invalid_input` | Uso/configuración |
| 3 | `authentication_required` | Identidad/credenciales |
| 4 | `forbidden` | Acceso rechazado |
| 5 | `not_found` | Ausente o no visible |
| 6 | `validation_failed` | Campos/ambigüedad |
| 7 | `conflict` | Cambio concurrente |
| 8 | `service_unavailable` | Red/429/lectura |
| 9 | `write_uncertain` | Escritura incierta o aceptada sin verificar |
| 10 | `partial_result` | Consulta completa/lote incompleto |
| 11 | `capability_unavailable` | Función no disponible en un contexto |
| 130 | `canceled` | Cancelación |

Los códigos 3–11 están reservados en contratos y se activarán con los casos de uso correspondientes. Las claves detalladas como `transition_ambiguous` se añadirán como refinamientos en F3 sin cambiar el significado de exit 6. No modificar el tipo de un campo de v1: los cambios incompatibles requieren nueva versión.
