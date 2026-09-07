# Contrato CLI de F1

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
- Solo `plain` o `json`. Se respeta la última aparición de `--format`; `--` termina las opciones.
- Fallos de escritura: código 1, nunca éxito.
- Los diagnósticos de parseo no repiten argumentos arbitrarios; errores HTTP no incluyen cuerpos ni Authorization.

`doctor` valida los permisos de identidad, no los de issues ni transiciones. Si usa entorno/stdin, indica `keyring_checked=false`; no afirma que el llavero funcione.
