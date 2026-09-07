# Fixtures sintéticos

Respuestas inventadas para pruebas futuras; nunca exportadas de Jira real. Claves APP, accountId, IDs y URL `example.atlassian.net` no son credenciales ni datos de una cuenta.

| Archivo | Escenario |
| --- | --- |
| myself.json | Identidad personal |
| search-page-1.json / search-page-2.json | Cursor opaco y última página |
| issue.json | Estado y descripción ADF |
| transitions.json | Iniciar y resolver con campo obligatorio |
| transitions-ambiguous.json | Dos destinos Done, incluido Cancelar |
| transition-required-error.json | Rechazo por resolución faltante |
| authentication-error.json | Autenticación fallida |
| rate-limit.json | Cuerpo ilustrativo de 429 |

El servidor httptest de F1/F2 añadirá códigos/cabeceras: `Retry-After: 2` para rate-limit y HTTP 204 sin cuerpo para transición aceptada. Estos fixtures no sustituyen validar el contrato real del proveedor.
