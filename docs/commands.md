# Contrato CLI de F2

| Comando | Comportamiento |
| --- | --- |
| `version` | Versión, sin red ni configuración |
| `auth login` | Valida `myself`, guarda perfil y opcionalmente credencial |
| `auth status` | Comprueba disponibilidad local del token; no valida con Jira |
| `auth logout` | Elimina perfil y credenciales locales; no revoca en Atlassian |
| `profile list` | Lista perfiles y marca el activo persistido |
| `profile use NOMBRE` | Cambia perfil activo |
| `me` | Consulta y muestra identidad Jira |
| `doctor` | Comprueba configuración, fuente de credencial, identidad, conectividad y stdin TTY |
| `config path` | Muestra ruta de configuración |
| `config validate` | Valida esquema sin mostrar contenido |

`--profile` selecciona temporalmente un perfil; `--email` cambia el correo de esa invocación. Precedencia: flags > `JFLOW_PROFILE`/`JFLOW_EMAIL` > perfil > correo global. `auth login` guarda el perfil validado y lo activa. Solo se conserva el correo de entorno al hacer un login explícito.

`auth login` recibe `--site`, `--method=api-token-unscoped|api-token-scoped`, `--cloud-id`, `--token-stdin` y `--no-store`. En un terminal puede solicitar datos faltantes y token oculto. `--no-input`, `JFLOW_NO_INPUT=1` o `JFLOW_NO_INPUT=true` desactivan las preguntas; stdin explícito continúa permitido. `me` y `doctor` también aceptan `--token-stdin`.

La credencial explícita de stdin tiene precedencia sobre `JFLOW_TOKEN`, y esta sobre el llavero. Un token de entorno nunca se persiste automáticamente. Sin llavero, usar `--no-store` y suministrar nuevamente la credencial en cada invocación. Nunca se acepta `--token VALOR`.

`--format=json` produce un solo objeto con `schema_version`, `ok`, `data`, `meta`, `warnings` y `error`. Puede ir antes o después del subcomando. La ayuda JSON está en `data.help`; `jflow help auth login` funciona. La salida normal usa texto sin colores.

- Sin subcomando: código 2; todavía no hay TUI automática.
- Argumentos/configuración inválidos: 2; autenticación: 3; permisos: 4; no encontrado: 5; servicio no disponible: 8; cancelación: 130.
- `plain` o `json`; F2 admite también `table` en listados. Se respeta la última aparición de `--format`; `--` termina las opciones.
- Fallos de escritura: código 1, nunca éxito.
- Los diagnósticos de parseo no repiten argumentos arbitrarios; errores HTTP no incluyen cuerpos ni Authorization.

`doctor` valida los permisos de identidad, no los de issues ni transiciones. Si usa entorno/stdin, indica `keyring_checked=false`; no afirma que el llavero funcione.

## F2: lectura de issues y enlaces

```bash
jflow mine --format table
jflow mine --project APP --status-category in-progress --sort=-priority,-updated
jflow mine --include-done --updated-since=-7d --limit 20
jflow config set default_project APP --profile trabajo
jflow list --type Bug
jflow search --jql 'project = APP AND priority = High ORDER BY updated DESC' --all --format json
jflow show APP-123
jflow show APP-123 --comments --history --section-limit 20
jflow link APP-123
jflow open APP-123
```

`mine` utiliza `currentUser()` y excluye categoría Done, salvo `--include-done` o una categoría explícita. `list` necesita un proyecto predeterminado o filtro explícito. Proyecto: `--project` > `JFLOW_PROJECT` > `default_project` del perfil. `search --jql` no hereda ese proyecto ni admite filtros estructurados. `--sort` permite `updated`, `created`, `priority`, `key`, `due`, separados por coma; el prefijo `-` indica descendente. Siempre hay un desempate por clave.

`--fields` selecciona entre `summary,status,assignee,priority,issuetype,project,updated,duedate,resolution`; los datos no solicitados quedan desconocidos. Clave e ID vienen del resultado Jira. No hay una solicitud por fila.

Paginación de búsqueda: `--limit 50`, `--page-size 50`, `--all`, `--max-results 5000`, `--page-token CURSOR`. `--all` y `--limit` son incompatibles. Un límite normal produce éxito con `meta.complete=false`; fallo posterior o tope de `--all` devuelve código 10 conservando los datos. JSON incluye `meta.returned` y `meta.next_page_token`; el cursor se reutiliza con el mismo perfil, JQL y campos. No hay total global inferido.

`show` carga descripción, subtareas y vínculos. `--comments` y `--history` habilitan sus endpoints; `--page-size` controla la solicitud y `--section-limit` el total por sección. `--all` recorre las secciones solicitadas hasta 5000 elementos por defecto. Para continuar, usa `--comments-start`/`--history-start` con el `next_start` de cada sección. Un fallo de sección conserva el detalle en JSON y devuelve código 10.

Lecturas: `--timeout 30s` (hasta 10m), `--token-stdin`, `--refresh`, `--offline`. Caché solo en memoria del proceso: no sobrevive a otra ejecución de `jflow`; offline sin entrada devuelve código 5. Listados duran 60 s y detalles 30 s. Un 401/403 no se sustituye por datos viejos. El transporte reintenta lecturas transitorias hasta tres envíos y respeta Retry-After dentro del presupuesto de tiempo.

`table` se admite en `mine`, `list` y `search`; otros comandos aceptan plain/JSON. `--no-color` y `--ascii` mantienen la salida sin adornos; no alteran el contenido Unicode de Jira. `--verbose` informa el comando por stderr sin tokens, cuerpos ni JQL. JSON implica `--no-input`.

`link` usa la URL navegable del sitio y no consulta Jira ni el llavero. En texto imprime exclusivamente URL y salto de línea. `open` usa `/usr/bin/open` en macOS o `xdg-open` en Linux sin shell; si no hay sesión gráfica/lanzador, devuelve código 11 y conserva la URL.
