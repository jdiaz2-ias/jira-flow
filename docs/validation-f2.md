# Validación F2

Fecha: 2026-09-07. Entorno local: macOS arm64, Go 1.27.1.

## Alcance implementado

- `mine`, `list`, `search`, `show`, `link`, `open` y `config set default_project`.
- Constructor JQL con literales escapados, categorías y orden permitido; POST `/rest/api/3/search/jql` con campos seleccionados.
- Páginas de búsqueda, deduplicación, cursores ligados al perfil/identidad/JQL/campos, conservación de filas sobrantes y resultados parciales.
- Descripción ADF normalizada, subtareas y vínculos; comentarios/historial paginados bajo demanda.
- Caché limitada en memoria; refresco, diagnóstico offline explícito, plazo total, texto/tabla/JSON sin controles de terminal.
- Adaptadores de navegador macOS/Linux con argumentos separados y URL disponible si el lanzador falla.

## Evidencia local

Las pruebas usan exclusivamente datos sintéticos. El usuario confirmó previamente que F1 conecta con su Jira real; F2 no ha consultado sus issues ni modificado esa configuración durante el desarrollo.

- `make check`: pasa; formato, vet, pruebas de aplicación/CLI/HTTP y auditoría foundation.
- `make test-race`: pasa; pruebas con detector de carreras.
- `make build-all`: pasa; CLI y foundation para Linux/macOS × amd64/arm64.
- `make build`: pasa; binario local `bin/jflow`.
- `go mod verify`: todos los módulos verificados; `go mod tidy` no altera go.mod/go.sum.
- `make security`: govulncheck 1.7.0 no encontró vulnerabilidades.
- CI remoto se consulta en el PR; los resultados anteriores corresponden al equipo local.

Casos comprobados: ambos métodos de token; selección de campos sin solicitudes por fila; estados/campos ausentes; 400/401/403/404 y respuestas inválidas; reintentos 429 con Retry-After; redirecciones rechazadas; cancelación; resultados vacíos, límites, cursores repetidos, duplicados y reanudación con remanentes; firma y aislamiento de cursores; TTL/refresco/fallos de autenticación sin fallback; ADF desconocido y ANSI/OSC; secciones parciales; navegador simulado; binario real sin TTY y un único documento JSON incluso con error.

## Límites observados

- Integración F2 con Jira real pendiente de prueba del usuario. Las pruebas de HTTP son HTTPS locales; no equivalen a verificar permisos de proyectos o scopes del tenant.
- No se ha lanzado un navegador real en las pruebas: se verifica el ejecutor inyectado, sin abrir ventanas.
- Caché únicamente por proceso: 128 entradas, hasta 16 MiB en total y 4 MiB por entrada; TTL listado 60 s y detalle 30 s. Cada invocación normal de CLI comienza vacía. `--offline` no recupera datos de comandos anteriores; persistencia corresponde a F4.
- Búsquedas: 50 resultados/página por defecto; tamaño máximo de página 100, límite protector máximo 5000 IDs por cadena de cursores y 1000 solicitudes por invocación. Los resultados no son snapshots transaccionales de Jira.
- Los cursores están firmados, no cifrados. Pueden contener filas sobrantes de la página (datos de Jira), pero no incluyen el token de autenticación ni el JQL. Son inválidos al cambiar perfil, identidad, credencial, consulta o campos. No se guardan en archivos por la aplicación.
- `show --comments/--history` carga hasta 50 elementos por sección por defecto. `--all` aumenta el límite a 5000, ajustable con `--section-limit`; los offsets se informan por sección. No se asume que una lista de subtareas visibles permita contar subtareas ocultas.
- ADF se normaliza a bloques/texto: no reproduce todos los estilos visuales. Nodos desconocidos conservan descendientes o muestran una marca; imágenes y adjuntos no se descargan. Límite de profundidad 64 y 10000 nodos, con advertencia.
- `open` confirma el éxito del lanzador, no la carga o autenticación del navegador.

Referencias oficiales reconsultadas: [búsqueda mejorada](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-search/), [issues e historial](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issues/), [comentarios](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-comments/), [ADF](https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/), [límites y reintentos](https://developer.atlassian.com/cloud/jira/platform/rate-limiting/).
