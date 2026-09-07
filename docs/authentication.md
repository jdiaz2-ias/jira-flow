# Autenticación prevista y catálogo para F1–F4

F0 no autentica ni almacena credenciales. Esta especificación permite implementar los adaptadores posteriores con fixtures sintéticos.

## Métodos personales Cloud

| Método | Autorización | Base REST | URL navegable |
| --- | --- | --- | --- |
| API token sin scopes | Basic correo:token | `https://sitio.atlassian.net` | El sitio |
| API token con scopes | Basic correo:token | `https://api.atlassian.com/ex/jira/{cloudId}` | El sitio |

Los tokens de service accounts y otros tipos de integración no se infieren de este contrato. Cloud ID debe ser real y proporcionado/descubierto explícitamente. No extraerlo del hostname ni enviar credenciales a un endpoint distinto para “probar”. Fuente: [tokens personales](https://support.atlassian.com/atlassian-account/docs/manage-api-tokens-for-your-atlassian-account/).

## Endpoints y permisos

La columna scopes enumera los scopes **clásicos OAuth publicados en la referencia REST**, como catálogo técnico, no como promesa de que toda modalidad de token acepte una lista idéntica. En F1, documentar la selección que ofrezca la consola de tokens del tenant probado. Los permisos de proyecto y visibilidad siguen siendo necesarios.

| Comando futuro | Método/ruta | Scope clásico de referencia | Permisos/contexto |
| --- | --- | --- | --- |
| `auth login`, `me`, `doctor` | GET `/rest/api/3/myself` | `read:jira-user` | Acceso a Jira |
| `mine`, `list`, `search`, `summary` | POST `/rest/api/3/search/jql` | `read:jira-work` | Browse Projects y seguridad de issue |
| `show`, `progress` | GET `/rest/api/3/issue/{key}` | `read:jira-work` | Issue visible |
| `transitions`, preparar acción | GET `/rest/api/3/issue/{key}/transitions` | `read:jira-work` | Transiciones disponibles para esa identidad |
| `start`, `done`, `close`, `transition` | POST `/rest/api/3/issue/{key}/transitions` | `write:jira-work` | Browse Projects y Transition Issues |
| `show --comments` | GET `/rest/api/3/issue/{key}/comment` | `read:jira-work` | Visibilidad de issue/comentario |
| `show --history` | GET `/rest/api/3/issue/{key}/changelog` | `read:jira-work` | Issue visible |
| Descubrir campos | GET `/rest/api/3/field` | `read:jira-work` | Campos visibles en el contexto |
| `link`, `open` | Ninguna llamada Jira | Ninguno | Configuración local |

Scopes granulares documentados para los primeros endpoints:

- `myself`: `read:application-role:jira`, `read:group:jira`, `read:user:jira`, `read:avatar:jira`.
- POST `search/jql`: `read:issue-details:jira`, `read:field.default-value:jira`, `read:field.option:jira`, `read:field:jira`, `read:group:jira`.
- GET transitions: `read:issue.transition:jira`, `read:status:jira`, `read:field-configuration:jira`.
- POST transitions: `write:issue:jira`, `write:issue.property:jira`.

No solicitar permisos administrativos para un flujo personal de consulta/transiciones. No reutilizar la lista granular de GET search para POST search; los scopes publicados pueden diferir. Los comandos que preparan y aplican necesitan la unión de sus lecturas y escritura.

Fuentes: [myself](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-myself/), [búsqueda](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-search/), [issues/transiciones](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issues/), [comentarios](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-comments/), [campos](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-fields/). Revisión: 2026-09-06; revalidar al activar cada operación.

## Credenciales y macOS

El puerto `SecretStore` ya está definido; todavía no hay backend invocado por la CLI. Ver [ADR-004](adr/004-secrets.md) para la revisión de la versión fijada y sus límites. En F1, keyring disponible permite persistencia; una sesión sin keyring utiliza token efímero por entorno/stdin. No hay fallback de archivo de texto plano.
