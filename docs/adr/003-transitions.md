# ADR-003: intenciones y transiciones separadas

Estado: aceptada. Fecha: 2026-09-06.

Decisión: representar start/done/close/reopen como intenciones. Preparar con metadatos frescos, resolver por regla o elección y revalidar antes de aplicar. Un resultado incierto no equivale a rechazo ni a éxito confirmado.

Consecuencia: se necesitan formularios y errores explícitos; cerrar puede diferir de completar. El dominio ya expresa esos estados, pero F3 implementará el resolver. No se actualiza `fields.status` de manera arbitraria ni se simula idempotencia.
