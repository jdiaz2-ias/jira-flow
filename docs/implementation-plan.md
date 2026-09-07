# Plan de implementación: Jira Flow CLI

Fecha: 6 de septiembre de 2026. Versión del plan: 1.0. Idioma: español.

Nombre de trabajo: **Jira Flow**. Ejecutable propuesto: **`jflow`**. El nombre es provisional; comprobar disponibilidad antes de publicar. Este documento especifica un producto por implementar: los comandos, pantallas y configuraciones son contratos propuestos, no funciones ya existentes.

## 1. Objetivo y decisiones principales

Construir una herramienta de consola para Linux y macOS que permita consultar los issues de Jira asignados al usuario autenticado, revisar su estado y avance, iniciar trabajo, completar o cerrar issues, consultar sus enlaces y abrirlos en el navegador. Debe servir tanto para trabajo interactivo diario como para scripts y permitir añadir funciones sin reescribir la interfaz o el cliente de Jira.

La implementación tendrá dos entradas que compartirán los mismos casos de uso: una CLI con subcomandos y una TUI, es decir, una interfaz interactiva dentro de la terminal. No requiere servidor propio ni un proceso permanente para la primera versión.

Decisiones de diseño del proyecto:

| Aspecto | Decisión |
| --- | --- |
| Lenguaje | Go; toolchain inicial 1.27.1, sujeto a actualizar parches antes de implementar |
| CLI | Cobra para comandos, ayuda y completado |
| TUI | Bubble Tea v2, Bubbles v2 y Lip Gloss v2 |
| Integración inicial | Jira Cloud, REST API v3, cliente HTTP propio y pequeño |
| Autenticación inicial | Correo y API token, con soporte explícito de tokens con scopes y sin scopes |
| Configuración | JSON versionado, perfiles y comandos de configuración |
| Credenciales | Keychain de macOS o Secret Service de Linux; variables de entorno para sesiones sin keyring |
| Persistencia | Archivos JSON atómicos para configuración, preferencias y caché opcional |
| Distribución | Binarios para Linux/macOS, amd64/arm64; GoReleaser y posteriormente Homebrew |
| Extensibilidad | Interfaces pequeñas, registro interno de acciones y adaptadores de proveedor |
| Idioma de producto | Mensajes inicialmente en español; comandos y claves JSON en inglés |

Go facilita entregar un ejecutable sin instalar un runtime de Python o Node. Cobra aporta estructura y completado. Las bibliotecas de Charm aportan componentes y estilos de terminal. Esta es una elección de ingeniería para este proyecto, no el resultado de un benchmark comparativo. Las versiones v2 de Charm ya están publicadas y sus rutas son `charm.land/bubbletea/v2`, `charm.land/bubbles/v2` y `charm.land/lipgloss/v2`. Fuentes: [Go](https://go.dev/doc/devel/release), [Cobra](https://github.com/spf13/cobra/blob/main/site/content/user_guide.md), [Charm v2](https://charm.land/blog/v2/).

## 2. Alcance, supuestos y entregas

Se asume un uso personal o interno de equipo con una cuenta Jira existente. No se dispone aún de dominio, proyecto, workflows, permisos ni credenciales reales; la primera fase implementará fixtures y la validación real se realizará en un proyecto de pruebas.

La compatibilidad Linux/macOS aplica desde la primera entrega. La compatibilidad con Jira Data Center es una ampliación independiente: no se debe anunciar soporte Data Center hasta implementar y probar su adaptador. No debe confundirse el sistema operativo de la CLI con el tipo de despliegue Jira.

### Entrega A: núcleo usable, requisito obligatorio

- Autenticación, perfiles, identidad actual y diagnóstico.
- Listar asignados a mí, búsqueda JQL y filtros habituales.
- Consultar detalle, estado, fechas, subtareas, vínculos y comentarios paginados.
- Mostrar progreso verificable con alcance y frescura de los datos.
- Imprimir URL y abrir el issue en el navegador.
- Descubrir transiciones e iniciar, completar y cerrar según el workflow.
- Solicitar campos de transición, confirmar y verificar resultados.
- TUI con listado y detalle, teclado, tema claro/oscuro y modo accesible.
- Salida de texto y JSON estable, códigos de salida y completado de shell.
- Binarios y pruebas para ambos sistemas operativos.

### Entrega B: productividad

- Crear comentarios, asignar a mí, editar campos comunes y reabrir mediante transiciones.
- Vistas guardadas, favoritos locales, historial de actividad y resumen diario.
- Tablero personal por categorías de estado y consulta de sprints cuando exista Jira Software.
- Registro manual de tiempo, condicionado a configuración y permisos.
- Extracción explícita de una clave Jira desde la rama Git actual.

### Entrega C: ampliaciones

- Crear issues a partir de metadatos de creación y plantillas locales.
- Adjuntos, enlaces entre issues, epics y métricas de sprint.
- Operaciones por lotes con revisión individual y reporte parcial.
- Adaptador Jira Data Center y protocolo de extensiones externas.
- Evaluar OAuth para una distribución organizacional; cronómetro local con publicación explícita.

Quedan fuera de la primera versión: administrar workflows o permisos, eliminar issues, automatizaciones que muten Jira en segundo plano, sincronización bidireccional offline, envío a chat/correo y gestión de solicitudes/aprobaciones de Jira Service Management.

## 3. Experiencia diaria esperada

Ejemplo ilustrativo del recorrido completo:

```bash
# Configuración inicial; el asistente solicita el token sin mostrarlo.
jflow auth login --profile trabajo --site https://empresa.atlassian.net
jflow auth status
jflow me

# Revisar lo que tengo pendiente.
jflow mine
jflow mine --status-category in-progress
jflow show APP-123
jflow progress APP-123

# Comenzar; antes de enviar se muestra la transición concreta.
jflow start APP-123

# Consultar o compartir la URL.
jflow link APP-123
jflow open APP-123

# Finalizar; completar campos exigidos por el workflow.
jflow done APP-123
jflow show APP-123 --refresh

# Interfaz interactiva.
jflow ui
```

`jflow` sin argumentos inicia la TUI si stdin y stdout son terminales y el entorno soporta interacción. Si no hay perfil, muestra el asistente de configuración. En una tubería imprime ayuda breve y termina con código 2; nunca debe entrar accidentalmente a pantalla completa.

`jflow mine` siempre produce un listado de una sola ejecución. La elección explícita de `ui` mantiene predecibles los scripts. Los comandos de lectura no cambian asignaciones ni estados.

## 4. Contrato de comandos y opciones

### 4.1 Comandos del núcleo

| Comando | Comportamiento |
| --- | --- |
| `jflow auth login` | Crear o actualizar credenciales del perfil y comprobar identidad |
| `jflow auth status` | Mostrar perfil, sitio, método y origen de credencial, nunca el secreto; `--verify` comprueba red |
| `jflow auth logout` | Retirar credencial local y caché de identidad del perfil; no revoca el token en Atlassian |
| `jflow me` | Obtener identidad autenticada |
| `jflow profile list` | Listar perfiles y marcar el activo |
| `jflow profile use NOMBRE` | Cambiar perfil activo localmente |
| `jflow config path` | Mostrar ruta de configuración |
| `jflow config validate` | Validar esquema y coherencia sin imprimir secretos |
| `jflow config set CLAVE VALOR` | Modificar una clave permitida con conversión según su tipo |
| `jflow mine` | Listar asignados al usuario actual, excluyendo categoría Done por defecto |
| `jflow list` | Consulta del proyecto predeterminado; exigir filtro explícito si no hay proyecto |
| `jflow search --jql CONSULTA` | Ejecutar JQL suministrado por el usuario |
| `jflow show CLAVE` | Detalle; `--comments`, `--history` habilitan secciones paginadas |
| `jflow progress CLAVE` | Estado, resolución, subtareas y datos de tiempo disponibles |
| `jflow summary` | Conteos por categoría sobre un alcance explícito o la vista personal |
| `jflow link CLAVE` | Escribir la URL navegable y salto de línea; no requiere consultar Jira |
| `jflow open CLAVE` | Abrir URL en navegador predeterminado y reportar si pudo lanzar el proceso |
| `jflow transitions CLAVE` | Listar IDs, nombres, destinos y campos de transiciones disponibles |
| `jflow transition CLAVE --id ID` | Ejecutar una transición explícita |
| `jflow start CLAVE` | Resolver intención de iniciar mediante reglas del apartado 9 |
| `jflow done CLAVE` | Resolver intención de completar mediante reglas del apartado 9 |
| `jflow close CLAVE` | Resolver intención de cerrar mediante regla configurada; no es alias ciego de done |
| `jflow workflow map` | Asistente para guardar una correspondencia validada por proyecto y tipo de issue |
| `jflow ui` | TUI; admite `--view` para una vista guardada cuando esté disponible |
| `jflow doctor` | Diagnóstico de configuración, autenticación, conectividad, terminal y keyring |
| `jflow completion bash\|zsh\|fish` | Generar completado sin instalarlo automáticamente |
| `jflow version` | Versión, commit y plataforma; formato JSON opcional |

`close` puede apuntar a la misma transición que `done` si el usuario lo configura. En workflows que distinguen Resuelto y Cerrado, conserva la diferencia. No existe un estado universal llamado Closed.

### 4.2 Opciones compartidas

| Opción | Regla |
| --- | --- |
| `--profile NOMBRE` | Selección temporal; no modifica el perfil activo |
| `--format table\|plain\|json` | Presentación; `table` solo para colecciones, rechazar combinaciones incompatibles |
| `--no-color`, `--ascii` | Quitar color o caracteres gráficos |
| `--no-input` | Prohibir preguntas, editor, selector y TUI; fallar si faltan datos |
| `--yes` | Aceptar la confirmación de una operación ya completamente resuelta |
| `--dry-run` | Preparar operación y mostrar efecto previsto; permite lecturas HTTP, no mutaciones Jira |
| `--refresh` | Omitir caché y consultar servidor |
| `--offline` | Consultar exclusivamente datos locales; incompatible con mutaciones |
| `--timeout 30s` | Plazo total del comando, incluidas esperas y verificación |
| `--verbose` | Diagnóstico redactado por stderr |

`--yes` no elige una transición ambigua ni inventa campos. `--format json` implica `--no-input`. Una mutación en JSON requiere `--yes` o `--dry-run`. La TUI rechaza `--format` y requiere una terminal apropiada. `--offline --refresh` es inválido.

Para listados: `--project`, `--status-category todo|in-progress|done`, `--type`, `--priority`, `--include-done`, `--updated-since`, `--sort`, `--limit`, `--all`, `--page-size` y `--page-token`. La opción `--status-category done` elimina automáticamente la exclusión predeterminada de completados.

`--limit` limita el total devuelto, 50 por defecto. `--page-size` limita cada solicitud, 50 por defecto. `--all` recorre hasta agotar, con un máximo protector configurable de 5.000 resultados; alcanzar ese máximo produce resultado parcial, nunca un total exacto. Para reanudar una página, usar un token opaco ligado al perfil, la consulta y sus campos. Si el límite cae a mitad de una página, solicitar solamente los elementos restantes cuando sea posible; el cursor propio deberá conservar el remanente si el proveedor entrega más.

No mezclar `search --jql` con filtros estructurados: rechazar la combinación para evitar interpretaciones inesperadas. `--sort` debe mapear a una lista permitida de campos, nunca interpolar texto arbitrario como sintaxis.

### 4.3 Funciones de productividad y ejemplos

```bash
jflow comment add APP-123 --body "Pruebas completadas; pendiente revisión."
jflow comment add APP-123 --editor
jflow assign APP-123 --me
jflow edit APP-123 --priority-id 2 --due 2026-09-15
jflow reopen APP-123
jflow view save revision --jql 'assignee = currentUser() AND status = "Code Review"'
jflow view run revision
jflow favorite add APP-123
jflow board --mine
jflow sprint list --board 42
jflow sprint show 86
jflow worklog add APP-123 --time 45m --started 2026-09-06T09:00:00-06:00
jflow context issue --from-git
jflow summary --view revision --format json
```

Los IDs y fechas son ejemplos. `--editor` lanza un ejecutable y argumentos configurados, suspende la TUI y presenta una vista previa antes de publicar. El borrador se guarda con permisos privados y se conserva si el envío falla. En la primera versión de comentarios se admite texto plano convertido a ADF; Markdown enriquecido será una ampliación explícita.

`context issue --from-git` solo lee la rama con `git symbolic-ref --short HEAD`; HEAD separado, ninguna clave o múltiples claves generan un error explicativo. La detección de Git no sucede implícitamente durante mutaciones.

## 5. Diseño visual de la terminal

### 5.1 Pantalla principal

Wireframe de referencia, no un requisito de dimensiones exactas:

```text
╭ Jira Flow · trabajo · empresa.atlassian.net ─────────── sincronizado 10:32 ╮
│ Mis pendientes  |  Favoritos  |  Vistas                  / Buscar         │
├──────────────────────────────────────┬───────────────────────────────────┤
│ CLAVE    ESTADO        RESUMEN       │ APP-123 · Mejorar autenticación   │
│ APP-123  En progreso   Mejorar auth  │ Estado: En progreso              │
│ APP-119  Por hacer     Corregir UI   │ Resolución: —                    │
│ APP-110  En revisión   Agregar logs  │ Asignado: Tú · Prioridad: Alta    │
│                                      │                                   │
│ 3 cargados · hay más resultados       │ Subtareas: 2 de 4 · 50%          │
│                                      │ Descripción y actividad…         │
├──────────────────────────────────────┴───────────────────────────────────┤
│ ↑↓ mover · Enter detalle · s iniciar · d completar · o abrir · ? ayuda  │
╰──────────────────────────────────────────────────────────────────────────╯
```

Usar bordes discretos, espacio suficiente, encabezados cortos y un acento cian/azul. To do se presenta con texto neutro, In progress en azul, Done en verde; errores en rojo y avisos en ámbar. Cada color siempre acompaña una etiqueta: el significado no depende del color.

No exigir Nerd Fonts ni emojis. Tema `auto`, `dark`, `light` y `mono`; `NO_COLOR` desactiva color independientemente del tema. Si no puede determinar el fondo, usar paleta conservadora configurable. `--ascii` sustituye bordes, barras y símbolos por caracteres simples.

### 5.2 Navegación

| Tecla | Acción |
| --- | --- |
| Flechas o `j/k` | Mover selección |
| Enter | Abrir detalle o aceptar selector activo |
| Tab / Shift+Tab | Cambiar foco |
| `/` | Filtrar localmente los elementos cargados, etiquetado como filtro local |
| `Ctrl+f` | Abrir búsqueda remota explícita |
| `s` / `d` / `x` | Preparar iniciar / completar / cerrar |
| `t` | Ver transiciones |
| `o` | Abrir navegador |
| `y` | Mostrar enlace seleccionable; copiar si existe backend disponible |
| `r` | Actualizar vista |
| `n` | Cargar siguiente página |
| `?` | Ayuda contextual |
| Esc | Volver o cancelar diálogo |
| `q` / Ctrl+C | Salir o cancelar; proteger borradores sin publicar |

Los atajos no se ejecutan mientras el foco está en un campo de texto. Una tecla de mutación abre revisión y confirmación; no envía directamente. Deshabilitar la acción mientras una solicitud está pendiente para evitar duplicados.

La copia automática al portapapeles es opcional: macOS `pbcopy`, Linux `wl-copy` o `xclip` si existen. La URL mostrada es la alternativa universal, especialmente por SSH. No emitir OSC 52 automáticamente ni presentar la copia como exitosa si el backend falla.

### 5.3 Estados y tamaños

- Ancho de 110 columnas o más: dos paneles, aproximadamente 55/45.
- De 80 a 109: listado y detalle alternables, menos columnas.
- Menos de 80: vista compacta; menos de 60 o altura menor de 15: mensaje para usar CLI o ampliar terminal.
- Carga: spinner con texto; error inicial: explicación y tecla de reintento.
- Lista vacía: distinguir consulta válida sin resultados de falta de permisos o error.
- Caché antigua: fecha visible; actualización parcial: aviso persistente.
- Cambio de tamaño: recalcular layout sin perder selección ni borrador.

Las llamadas HTTP deben ejecutarse fuera de `Update`/`View` como comandos asíncronos. Cada carga llevará un ID de generación: descartar respuestas antiguas tras cambiar filtro o perfil. Cancelar solicitudes obsoletas con `context.Context`.

Al abrir navegador o editor, restaurar/suspender terminal según corresponda. Al salir, recuperar cursor, eco y pantalla, incluso por interrupción. `TERM=dumb`, salida redirigida y lector de pantalla deben tener una alternativa funcional mediante `plain`.

## 6. Arquitectura y estructura del repositorio

Dependencias permitidas:

```text
CLI Cobra ───────┐
                ├── Casos de uso ── Dominio + puertos
TUI Bubble Tea ─┘                      ↑
                         Adaptadores HTTP, secretos,
                         caché, reloj, navegador y Git
```

El dominio no importa Cobra, Bubble Tea, JSON HTTP ni bibliotecas del keyring. La CLI/TUI traduce interacción a solicitudes de aplicación; no construye URLs de API ni decide transiciones por su cuenta.

Estructura propuesta:

```text
jira-flow/
  cmd/jflow/main.go
  internal/
    app/                 # Casos de uso; composición y cancelación
    domain/              # Issue, estado, progreso, transición, errores
    ports/               # Interfaces mínimas consumidas por app
    provider/
      jiracloud/         # DTOs, endpoints, ADF, paginación Cloud
      jiradc/            # Solo al implementar Entrega C
    transport/           # HTTP, autenticación, límites, redacción
    workflow/            # Resolución de intenciones y formularios
    cli/                 # Constructores Cobra y flags
    tui/                 # Modelos, mensajes, componentes y temas
    output/              # Texto, tabla, JSON v1
    config/              # Esquema, precedencia, migraciones y escritura
    secrets/             # Keyring y entorno
    cache/               # Memoria y persistencia opcional
    platform/            # Browser, clipboard, editor, rutas y terminal
    actions/             # Registro de acciones compartidas
  testdata/              # Fixtures sintéticos sin información privada
  tests/integration/     # Servidor HTTP simulado y pruebas del binario
  docs/
    architecture.md
    commands.md
    authentication.md
    workflows.md
    compatibility.md
    json-contract.md
    adr/
  .github/workflows/
  .goreleaser.yaml
  Makefile
  go.mod
  go.sum
  README.md
```

Dependencias externas iniciales: Cobra, las tres bibliotecas Charm, `golang.org/x/term` para detección/entrada oculta y `github.com/zalando/go-keyring`. Utilizar `net/http`, `encoding/json`, `log/slog`, `testing` y `httptest` del estándar. Evitar SDK Jira completo, ORM y base de datos hasta que un requisito lo justifique.

Fijar versiones exactas compatibles en `go.mod`/`go.sum` durante F0; no combinar componentes Charm v1/v2. Herramientas de lint y release se fijan igualmente en CI. El objetivo es `CGO_ENABLED=0`; comprobarlo con las dependencias elegidas antes de prometer binarios sin bibliotecas adicionales.

### 6.1 Contratos de dominio

Tipos mínimos; son especificaciones conceptuales que el agente debe convertir en tipos Go completos:

```go
type Issue struct {
    ID         string
    Key        string
    Summary    string
    ProjectID  string
    IssueTypeID string
    Status     Status
    Resolution *NamedID
    Assignee   *User
    UpdatedAt  time.Time
    DueDate    *LocalDate
    URL        string
}

type Status struct {
    ID       string
    Name     string
    Category string // todo | in-progress | done | unknown
}

type IssueReader interface {
    Myself(context.Context) (User, error)
    Search(context.Context, SearchRequest) (IssuePage, error)
    GetIssue(context.Context, IssueRef, DetailOptions) (IssueDetail, error)
}

type TransitionGateway interface {
    ListTransitions(context.Context, IssueRef) ([]Transition, error)
    ApplyTransition(context.Context, TransitionRequest) (ApplyResult, error)
}

type SecretStore interface {
    Get(context.Context, CredentialRef) (Secret, error)
    Set(context.Context, CredentialRef, Secret) error
    Delete(context.Context, CredentialRef) error
}
```

Otros contratos: `CommentReader`, `CommentWriter`, `IssueEditor`, `WorklogWriter`, `Browser`, `Clipboard`, `Cache`, `Clock` y `Sleeper`. Añadirlos al implementar su función, no una interfaz monolítica con métodos vacíos.

`IssueDetail` contiene descripción normalizada, subtareas, enlaces y secciones paginadas. `FieldSpec` contiene ID, nombre, requerido, tipo, valores permitidos y metadatos relevantes. `Transition` contiene ID, nombre, estado destino y campos. IDs siempre como strings salvo cuando un endpoint exija expresamente números; no inferir semántica del valor numérico.

`PreparedAction` contiene perfil, issue, intención, transición resuelta, campos, estado observado y cambios previstos; no contiene secretos. `ApplyResult` distingue `verified`, `accepted_unverified`, `unknown`, `failed` y `noop`. Los serializadores públicos son independientes de los tipos internos para poder refactorizar sin romper scripts.

## 7. Configuración, autenticación y perfiles

### 7.1 Rutas y precedencia

Linux: configuración en `$XDG_CONFIG_HOME/jflow/config.json` o `~/.config/jflow/config.json`; caché en `$XDG_CACHE_HOME/jflow` o `~/.cache/jflow`; estado en `$XDG_STATE_HOME/jflow` o `~/.local/state/jflow`.

macOS: configuración/estado en `~/Library/Application Support/jflow/` y caché en `~/Library/Caches/jflow/`. Implementar rutas en un solo módulo con pruebas; no asumir que `os.UserConfigDir` y XDG tienen idéntica semántica entre plataformas.

Precedencia: flags explícitos > variables `JFLOW_*` > perfil seleccionado > valores globales > valores predeterminados. Resolver primero la ruta `JFLOW_CONFIG`, después el perfil y después sus opciones. No cargar configuración del repositorio actual automáticamente.

Variables iniciales: `JFLOW_CONFIG`, `JFLOW_PROFILE`, `JFLOW_TOKEN`, `JFLOW_EMAIL`, `JFLOW_NO_INPUT`. `JFLOW_TOKEN` solo afecta a la invocación/perfil seleccionado y nunca se persiste automáticamente. No aceptar `--token VALOR` para evitar exponerlo en historial y argumentos del proceso.

### 7.2 Ejemplo de configuración

Los identificadores siguientes son ilustrativos. El asistente debe guardar valores reales descubiertos, nunca copiar estos IDs como predeterminados:

```json
{
  "schema_version": 1,
  "active_profile": "trabajo",
  "ui": {"theme": "auto", "ascii": false, "language": "es"},
  "cache": {"persist": false, "list_ttl_seconds": 60, "detail_ttl_seconds": 30},
  "profiles": {
    "trabajo": {
      "provider": "jira-cloud",
      "site_url": "https://empresa.atlassian.net",
      "cloud_id": "ID-REAL-DEL-SITIO",
      "auth": {
        "method": "api-token-scoped",
        "email": "persona@example.com",
        "credential_ref": "perfil-uuid-generado"
      },
      "default_project": "APP",
      "timezone": "America/Monterrey",
      "workflow_rules": [
        {
          "project_id": "10001",
          "issue_type_id": "10002",
          "intent": "start",
          "from_status_id": "1",
          "transition_id": "21",
          "expected_to_status_id": "3"
        }
      ],
      "views": {
        "mis-pendientes": "assignee = currentUser() AND statusCategory != Done ORDER BY updated DESC, key ASC"
      },
      "field_map": {"story_points": null}
    }
  }
}
```

Configurar `api-token-unscoped` para token sin scopes. `api_base_url` se deriva del método y no se guarda como un segundo valor editable que pueda contradecir sitio/cloud ID. Validar HTTPS, origen, ruta base y ausencia de usuario/contraseña en URL. En Data Center preservar el context path cuando se implemente.

Archivos privados `0600` y directorios `0700`, escritura temporal en el mismo directorio seguida de rename y bloqueo para evitar perder cambios entre procesos. Migraciones por `schema_version`, respaldo antes de migrar y rechazo de versiones futuras desconocidas. No imprimir credenciales al mostrar configuración.

### 7.3 Inicio de sesión

1. Solicitar nombre de perfil, URL del sitio, correo y tipo de token.
2. Explicar en el asistente cómo generar el token mediante un enlace oficial.
3. Para token con scopes, solicitar el cloud ID y ofrecer instrucciones de descubrimiento oficiales; no inferirlo del nombre del sitio ni asumir que endpoints OAuth aceptan Basic.
4. Recibir token con entrada oculta o `--token-stdin` para automatización.
5. Construir autenticación Basic correo:token en memoria y comprobar `myself`.
6. Mostrar la identidad encontrada y guardar la referencia y el secreto solo tras validación exitosa.
7. Probar una consulta pequeña y, opcionalmente, transiciones de un issue elegido para diagnosticar permisos de lectura/escritura sin ejecutar cambios.

Jira Cloud admite Basic con correo y API token. Los tokens con scopes utilizan `https://api.atlassian.com/ex/jira/{cloudId}`; los tokens sin scopes usan la URL del sitio. Los enlaces navegables siguen apuntando al sitio. Los tokens expiran; un 401 debe sugerir comprobar vencimiento o revocación, sin afirmar que esta sea necesariamente la causa. Fuentes: [Basic auth](https://developer.atlassian.com/cloud/jira/platform/basic-auth-for-rest-apis/), [tokens y scopes](https://support.atlassian.com/atlassian-account/docs/manage-api-tokens-for-your-atlassian-account/), [identidad actual](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-myself/).

Documentar una tabla de scopes por comando a partir de la referencia vigente de cada endpoint durante F0. No copiar scopes OAuth indiscriminadamente como si fueran intercambiables con todos los tipos de token. Se puede usar un perfil de solo lectura; las funciones de escritura deben reportar permisos/scopes insuficientes cuando corresponda.

### 7.4 Almacenamiento de secretos

Usar el servicio `jflow` y una referencia aleatoria por perfil, ligada al sitio y usuario. Linux requiere una sesión con Secret Service; macOS usa Keychain. La biblioteca propuesta depende de D-Bus/Secret Service en Linux y de `/usr/bin/security` en macOS. Fuente: [go-keyring](https://github.com/zalando/go-keyring).

En una sesión SSH sin keyring, permitir credencial solo en memoria mediante entorno o entrada estándar; no recurrir silenciosamente a texto plano en disco. Evaluar el backend macOS para evitar exposición del token en argumentos durante el guardado: si la biblioteca seleccionada no cumple, implementar un backend nativo o documentar y resolver ese punto antes de publicar autenticación persistente. La interfaz `SecretStore` permite sustituirlo.

`auth logout` elimina la copia local y purga datos privados del perfil. Si una variable de entorno sigue aportando token, indicar que aún existe esa fuente; no afirmar que se cerró toda autenticación. La revocación remota del token se realiza desde Atlassian.

OAuth 3LO se reserva para una decisión de arquitectura posterior. No incrustar un client secret en el binario, ni inventar soporte device-code/PKCE. Verificar los flujos autorizados por Atlassian y si requieren un componente confidencial antes de diseñar esa entrega. Referencia: [autenticación de integraciones](https://developer.atlassian.com/cloud/jira/platform/rest/v3/intro/).

## 8. Cliente Jira y consultas

### 8.1 Operaciones de referencia

Rutas relativas a la base autenticada correspondiente:

| Necesidad | Operación Cloud |
| --- | --- |
| Identidad | `GET /rest/api/3/myself` |
| Búsqueda | `POST /rest/api/3/search/jql` |
| Issue | `GET /rest/api/3/issue/{key}` |
| Transiciones | `GET /rest/api/3/issue/{key}/transitions?expand=transitions.fields` |
| Ejecutar transición | `POST /rest/api/3/issue/{key}/transitions` |
| Comentarios | `GET` / `POST /rest/api/3/issue/{key}/comment` |
| Historial | `GET /rest/api/3/issue/{key}/changelog` |
| Asignar | `PUT /rest/api/3/issue/{key}/assignee` |
| Editar | `PUT /rest/api/3/issue/{key}`; consultar metadatos de edición cuando aplique |
| Campos | `GET /rest/api/3/field` |
| Tiempo | `GET` / `POST /rest/api/3/issue/{key}/worklog` |
| Boards/sprints | Adaptador opcional para `/rest/agile/1.0/...` |

Referencias: [issues y transiciones](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issues/), [comentarios](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-comments/), [campos](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-fields/), [worklogs](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-worklogs/), [Jira Software](https://developer.atlassian.com/cloud/jira/software/rest/api-group-board/).

La búsqueda inicial utilizará el endpoint mejorado `/search/jql`, no el antiguo `/search` en retirada. Debe recorrer `nextPageToken` y tolerar resultados que todavía no reflejen un cambio reciente; Jira documenta `reconcileIssues` para reforzar consistencia tras cambios conocidos. Fuente: [búsqueda Jira Cloud](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-search/).

### 8.2 Construcción de consultas

Consulta predeterminada de `mine`:

```jql
assignee = currentUser()
AND statusCategory != Done
ORDER BY updated DESC, key ASC
```

Consulta de ejemplo con filtro explícito:

```jql
assignee = currentUser()
AND project = "APP"
AND statusCategory = "In Progress"
ORDER BY priority DESC, updated DESC, key ASC
```

Implementar un constructor pequeño de predicados con escape de literales y una lista de campos/operadores permitidos. El `--jql` explícito se envía como texto del usuario, sin ejecutarlo en shell. Traducir categorías internas `todo`, `in-progress`, `done` a los literales JQL correspondientes; traducir claves del proveedor `new`, `indeterminate`, `done` al dominio. Nombres de estados libres no se usan para inferir categorías.

Solicitar solo campos de listado: clave/ID y `summary,status,assignee,priority,issuetype,project,updated,duedate,resolution`. Descripción, comentarios, historial y otros campos se cargan bajo demanda. La lista no debe hacer una solicitud extra por fila.

La paginación de cada endpoint vive en su adaptador; comentarios y changelog no tienen por qué seguir el cursor de búsqueda. Detectar cursores repetidos, deduplicar por ID y marcar `complete=false` ante truncamiento o error parcial. Las búsquedas sobre datos cambiantes no equivalen a snapshots transaccionales.

### 8.3 Transporte y fallos

Política propuesta: conexión de 5 segundos, plazo total predeterminado de 30 segundos, hasta 3 intentos de lectura y concurrencia máxima inicial de 4. Son parámetros del producto, no límites publicados por Jira.

- Lecturas: reintentar errores de red transitorios, 429 y 502/503/504 con retroceso exponencial y jitter.
- Respetar `Retry-After`; si supera el tiempo disponible, terminar con error accionable y tiempo sugerido.
- Tratar POST de búsqueda como lectura por semántica, no como una mutación por el verbo.
- Escrituras: un solo envío en la primera versión. Ante respuesta ambigua, reconciliar; no repetir automáticamente comentarios, worklogs ni transiciones.
- 400: campos/JQL inválidos. 401: autenticación. 403: acceso rechazado. 404: recurso inexistente o no visible; no distinguir sin evidencia. 409: conflicto cuando el proveedor lo devuelva.
- Respuestas HTML inesperadas: posible proxy/login; devolver diagnóstico sin volcar el cuerpo completo.

Atlassian documenta respuestas 429 y `Retry-After`, y distingue límites según integración, incluyendo tráfico con API token. El cliente responde a cabeceras y errores reales; no presupone una cuota universal. Fuente: [rate limiting](https://developer.atlassian.com/cloud/jira/platform/rate-limiting/).

Usar TLS validado, proxy del entorno y CA corporativa configurable. No implementar `--insecure` por defecto. No reenviar Authorization a otro origen mediante redirects; rechazar redirecciones autenticadas fuera de la base autorizada. Limitar cuerpos recibidos y redactar errores/logs antes de mostrarlos.

### 8.4 Descripciones y ADF

Cloud v3 usa Atlassian Document Format en diversos campos enriquecidos. Fuente: [introducción API v3](https://developer.atlassian.com/cloud/jira/platform/rest/v3/intro/).

El adaptador transforma ADF en bloques internos: párrafo, encabezado, lista, código, enlace y texto. La TUI renderiza ese árbol con estilos; `plain` conserva texto y URLs. Un nodo desconocido muestra texto descendiente o una marca de contenido no soportado; no bloquea todo el issue.

Para publicar texto plano, generar un documento ADF válido con párrafos y texto. No aceptar Markdown fingiendo convertirlo integralmente. No descargar imágenes ni ejecutar HTML; los enlaces solo se abren por acción explícita.

## 9. Iniciar, completar, cerrar y reabrir

### 9.1 Principio de implementación

Una intención del usuario y una transición Jira son objetos distintos. `start`, `done`, `close` y `reopen` son intenciones locales. El servidor define qué transiciones existen, cuáles están disponibles para el usuario y qué datos exige cada una. Nunca cambiar el estado mediante un `PUT` arbitrario a `fields.status`.

El algoritmo siguiente es una política propuesta de Jira Flow. Se apoya en el catálogo de transiciones del proveedor, pero no presume que Atlassian imponga estas reglas de selección.

### 9.2 Resolución de intención

Orden determinista:

1. Obtener issue y transiciones frescas, con sus campos. La caché no autoriza una mutación.
2. Si hay `--transition-id`, buscar ese ID entre las disponibles y validar su destino.
3. En otro caso, buscar una regla exacta por perfil, proyecto, tipo de issue, intención y estado origen. Una regla inválida produce error; no elegir silenciosamente otra transición.
4. Para `start`, ofrecer transiciones a categoría In progress, excluyendo bucles al mismo estado. Para `done`, ofrecer las que llegan a categoría Done.
5. Si solo hay una candidata, preparar y mostrar su nombre/destino para confirmar. Si hay más, solicitar elección; en `--no-input`, devolver ambigüedad con candidatos.
6. Para `close`, exigir regla o elección explícita entre transiciones disponibles: categoría Done no distingue resolución de cierre. Para `reopen`, exigir igualmente regla/elección, ya que el destino puede ser To do o In progress.
7. Si no hay candidatas, mostrar estado actual y acciones disponibles. No construir rutas de varios saltos para llegar al destino.

Ejemplo: desde En progreso existen `Enviar a QA`, `Resolver` y `Cancelar`. Si Resolver y Cancelar terminan en Done, `done` presenta ambas y no asume que Cancelar representa trabajo completado. La configuración puede fijar Resolver para ese proyecto y tipo.

Para `start` sin destino explícito, un issue ya en categoría In progress devuelve `noop` e informa su estado. Para `done` sin regla/destino explícito, categoría Done permite `noop` con resolución visible; no afirmar que el issue fue entregado exitosamente si su resolución es Cancelado. Si existe regla, comparar el destino exacto esperado antes de decidir `noop`. `close` y `reopen` requieren conocer su destino específico. Una transición explícita de bucle sigue siendo ejecutable tras confirmación, porque puede tener efectos adicionales.

### 9.3 Campos requeridos

Crear un registro de editores según `FieldSpec`: texto, número, fecha, usuario, selección simple y selección múltiple. Usar IDs reales, esquemas y valores permitidos; no asumir que prioridad, resolución o story points comparten IDs entre sitios.

En modo interactivo, mostrar únicamente los campos requeridos que falten y permitir editar los opcionales disponibles. En modo no interactivo:

```bash
jflow done APP-123 --transition-id 31 --fields-file ./campos-cierre.json --dry-run
jflow done APP-123 --transition-id 31 --fields-file ./campos-cierre.json --yes --no-input
```

Archivo ilustrativo, con IDs obtenidos de los metadatos:

```json
{
  "resolution": {"id": "10000"},
  "customfield_10421": "Pruebas revisadas"
}
```

`--fields-file` es un objeto de campos, no un body HTTP completo. Validar tamaño, tipo raíz, IDs de campos y tipos conocidos; el cliente envuelve esos datos en la solicitud. Una alternativa `--field-json 'resolution={"id":"10000"}'` es opcional, nunca necesaria para completar la entrega.

Un campo no soportado permite introducir JSON explícito si su esquema puede validarse; si no es posible, explicar el campo y ofrecer `open`. Los validadores de plugins o del workflow pueden exigir condiciones no descritas en los metadatos: mostrar el rechazo del servidor y conservar los datos del formulario.

No asignar una resolución por defecto, no borrar la resolución al reabrir sin que el workflow lo permita y no completar subtareas automáticamente. Un issue Done con resolución vacía se muestra como tal, junto a una observación de posible configuración del workflow; la CLI no lo “repara”.

### 9.4 Preparación, confirmación y ejecución

```text
Intención → lectura fresca → transición resuelta → campos válidos
         → vista previa → confirmación → revalidación → envío
         → lectura del issue → resultado verificado o incierto
```

Vista previa mínima:

```text
Sitio: empresa.atlassian.net · Perfil: trabajo
APP-123 · Mejorar autenticación
Transición: Resolver (ID 31)
Estado: En progreso → Resuelto
Resolución solicitada: Fixed
¿Aplicar este cambio? [s/N]
```

Antes del POST, volver a consultar estado/transiciones si hubo interacción. Si el estado relevante o la transición cambió, invalidar la preparación y pedir nueva revisión; en automatización devolver conflicto. Este control reduce carreras, pero no garantiza compare-and-swap: el servidor sigue siendo la autoridad.

Enviar solo la transición y los campos solicitados. No añadir un comentario separado como efecto oculto de `done`. Si posteriormente se admite comentario junto con transición, definir si viaja en una única operación soportada por Jira o como dos operaciones con reporte parcial.

### 9.5 Verificación y respuestas inciertas

Después de una aceptación HTTP, invalidar detalle/listados relacionados y consultar directamente el issue. Se permiten hasta tres lecturas espaciadas dentro del plazo del comando. Comparar destino y campos relevantes; las automatizaciones pueden modificar el estado nuevamente y deben quedar reflejadas.

- `verified`: se recibió aceptación y se observó el destino esperado.
- `accepted_unverified`: Jira aceptó la solicitud, pero no fue posible confirmar el estado final. Informar que no debe repetirse ciegamente.
- `unknown`: el envío pudo haber llegado y no hubo respuesta concluyente. Puede mostrarse el estado observado posteriormente, pero no atribuir el cambio a esta CLI sin evidencia.
- `failed`: rechazo confirmado; incluir información de campos cuando exista.
- `noop`: ya se satisface la intención según las reglas descritas, sin enviar POST.

No se garantiza semántica “exactamente una vez”. No usar un header de idempotencia inventado: sin soporte del endpoint no evita duplicados. En un timeout de comentario/worklog, mostrar estado incierto y permitir inspeccionar actividad; no reenviar automáticamente.

Guardar únicamente metadatos mínimos de acciones recientes, por perfil: UUID local, clave, acción, hora y resultado. Nunca token, descripción ni cuerpo de comentario. Permitir desactivar este registro; TTL predeterminado de siete días. El UUID local facilita diagnóstico, pero no es un identificador de transacción del servidor.

## 10. Definición precisa de progreso

La palabra “progreso” puede significar varias cosas. La CLI debe presentar métricas con nombre y denominador; no convertir automáticamente En progreso en 50% ni asignar porcentajes arbitrarios a estados.

| Indicador | Cálculo o fuente | Presentación |
| --- | --- | --- |
| Estado | Estado y categoría devueltos por Jira | `En revisión · In progress` |
| Resolución | Valor actual del campo | `Fixed`, `Cancelled` o `Sin resolución` |
| Subtareas | Completadas en categoría Done / subtareas visibles consultadas | `2 de 4 subtareas · 50%` |
| Tiempo registrado | Segundos reportados por Jira | `3 h registradas` |
| Tiempo restante | Estimación restante reportada | `2 h estimadas restantes` |
| Consumo de estimación | Tiempo registrado / estimación original, si esta es positiva | `150% de estimación consumida`, sin limitar a 100 |
| Actualización | Fecha `updated` | `Actualizado hace 2 días` |
| Tiempo en estado | Historial de cambios de estado consultado completamente | `En este estado desde ...`; desconocido si no se puede determinar |
| Vencimiento | Fecha local de vencimiento y zona configurada | `Vence hoy` o `Vencido hace 2 días` |

Reglas adicionales:

1. Cero subtareas significa `Sin subtareas`, no 0% ni 100%.
2. Solo presentar porcentaje de subtareas cuando se conozca el denominador del conjunto visible. Si faltan páginas, mostrar conteos parciales y omitir porcentaje global.
3. Aclarar que se consideran issues visibles para la cuenta; no prometer detectar elementos ocultos por seguridad.
4. Subtareas completadas incluyen cualquier resolución dentro de Done. Mostrar desglose de resolución si se necesita distinguir entregadas de canceladas.
5. Un issue padre Done con subtareas abiertas muestra ambas cosas; no altera el workflow.
6. No sumar simultáneamente valores agregados de tiempo del padre y tiempos individuales de hijos.
7. Story points no equivalen a tiempo ni son un porcentaje de avance. Si se añaden, descubrir el campo por configuración validada, no hardcodear `customfield_10016`.
8. Las fechas de vencimiento sin hora se comparan como fecha local; no convertirlas arbitrariamente a medianoche UTC.

`summary` devuelve conteos por categoría y, opcionalmente, fracción completada de una población definida. Como `mine` excluye completados por defecto, un porcentaje de completitud requiere `--include-done` y alcance apropiado; la interfaz no mostrará una barra engañosa sobre solo pendientes. Un ejemplo válido es un sprint específico o un proyecto/tipo definido.

Cada resumen incluye consulta o descripción del alcance, fecha de actualización, número procesado, indicador `complete` y método de medición. No usar un conteo aproximado como denominador exacto. Una consulta truncada puede producir `7 completados de 20 cargados; total desconocido`, sin porcentaje global.

## 11. URL, navegador e integración del sistema

Construir URL como `site_url` más `/browse/{key}`, conservando el context path cuando exista. No utilizar `api.atlassian.com/ex/jira/...` para enlaces humanos. Validar la clave y escapar segmentos de ruta mediante `net/url`.

`jflow link APP-123` funciona sin red si el sitio está configurado. Escribir solo la URL en modo normal facilita pipes. `--format json` devuelve un objeto con `key` y `url`. No garantizar que el usuario tenga acceso sin consultar Jira.

`jflow open` llama `/usr/bin/open` en macOS y `xdg-open` en Linux con argumentos separados. No usar `sh -c`, concatenación de shell ni ejecutar el texto del issue. Si no hay entorno gráfico o lanzador, informar el motivo y mostrar la URL; devolver código de capacidad no disponible.

Una salida exitosa significa que se inició el lanzador, no que el navegador cargó o autenticó la página. Configuración opcional de navegador/editor mediante arrays de ejecutable y argumentos, nunca una cadena de shell evaluada.

## 12. Caché, privacidad y refresco

En la primera versión la caché es de memoria por proceso. Permitir persistencia mediante `cache.persist=true` para uso offline; explicar que almacena contenido de Jira en el equipo. El modo offline falla con mensaje claro si nunca se habilitó o no existe esa entrada.

TTL propuestos: listados 60 segundos, detalles 30 segundos, identidad 15 minutos y metadatos de campos 1 hora. Las transiciones se obtienen frescas al preparar/aplicar acciones. La TUI permite refresco manual y polling opcional cada 60 segundos, sin solicitudes superpuestas ni daemon oculto.

Clave de caché: proveedor, origen/base autenticada, perfil, identidad, consulta normalizada, campos y versión del esquema. No compartir por clave del issue únicamente. Invalidar al cambiar credencial, usuario o sitio. Una renovación de credencial incrementa una generación local de caché.

La persistencia opcional guarda archivos JSON con límite total de 25 MB, máximo de 1.000 detalles y vencimiento máximo de siete días. Usar nombres derivados de hash, no resúmenes de issues. Excluir cuerpos de comentarios/descripciones de la persistencia por defecto; una opción adicional puede habilitarlos. La configuración de caché no debe contener tokens.

Implementar `jflow cache status` y `jflow cache clear --profile trabajo`; documentar exactamente qué archivos controla la aplicación. Los resultados locales siempre incluyen hora de captura y `stale`. Un 401/403 online no se convierte silenciosamente en éxito usando caché antigua.

Sanitizar texto remoto antes de renderizar: quitar escapes ANSI, controles de terminal y OSC provenientes de resúmenes, comentarios y nombres. JSON puede conservar texto como datos escapados, sin secuencias de control literales. Las imágenes y adjuntos no se descargan automáticamente.

Logs sin headers de autenticación, cookies, tokens, cuerpos completos ni JQL privado por defecto. No incluir telemetría en el núcleo. Las pruebas usan datos sintéticos y ningún secreto se almacena en fixtures, snapshots o repositorio.

## 13. Salida para scripts y errores

Stdout se reserva para el resultado; stderr para diagnóstico y progreso. En salida redirigida, sin ANSI, spinners ni paginador. `--format json` emite exactamente un documento JSON por invocación, incluso si hay error. Para JSON, errores estructurados van en ese documento de stdout y el código de salida sigue siendo no cero; stderr puede contener diagnóstico adicional redactado.

Contrato de búsqueda propuesto:

```json
{
  "schema_version": 1,
  "ok": true,
  "data": {
    "issues": [
      {
        "id": "100123",
        "key": "APP-123",
        "summary": "Mejorar autenticación",
        "status": {"id": "3", "name": "En progreso", "category": "in-progress"},
        "resolution": null,
        "url": "https://empresa.atlassian.net/browse/APP-123"
      }
    ]
  },
  "meta": {
    "profile": "trabajo",
    "fetched_at": "2026-09-06T16:32:00Z",
    "source": "network",
    "stale": false,
    "complete": false,
    "returned": 1,
    "next_page_token": "cursor-opaco-de-ejemplo"
  },
  "warnings": [],
  "error": null
}
```

`ok=true, complete=false` es normal cuando el usuario solicitó una página o un límite. Si se pidió `--all` y un fallo o límite protector impide completarlo, usar error parcial y código 10 conservando datos disponibles.

Contrato de error propuesto:

```json
{
  "schema_version": 1,
  "ok": false,
  "data": null,
  "meta": {"profile": "trabajo"},
  "warnings": [],
  "error": {
    "code": "transition_ambiguous",
    "message": "Hay más de una transición posible para completar APP-123.",
    "retryable": false,
    "details": {"candidate_transition_ids": ["31", "41"]}
  }
}
```

Para mutaciones, `data.result` usa los estados de 9.5 y presenta estado anterior, observado y transición. `accepted_unverified`/`unknown` tienen `ok=false` y código 9, aunque exista aceptación inicial; el mensaje distingue ese caso de un rechazo. Mantener `null` para valores desconocidos, tiempos RFC3339 y campos numéricos con unidades explícitas.

| Salida | Significado |
| --- | --- |
| 0 | Éxito, lista vacía válida o noop informado |
| 1 | Error interno no clasificado |
| 2 | Argumentos/configuración inválidos |
| 3 | Autenticación ausente o rechazada |
| 4 | Permisos insuficientes |
| 5 | Recurso no encontrado o no visible |
| 6 | Ambigüedad, campos faltantes o validación rechazada |
| 7 | Conflicto detectado al revalidar |
| 8 | Red/límite de solicitudes antes de una mutación, o lectura fallida |
| 9 | Escritura aceptada sin verificar o de resultado incierto |
| 10 | Resultado parcial solicitado como completo, o lote parcial futuro |
| 11 | Capacidad no disponible: navegador, modo offline sin datos, proveedor incompatible |
| 130 | Cancelación del usuario |

La cancelación tras un envío no cancela necesariamente el cambio remoto: imprimir que el resultado puede ser incierto antes de salir. En operaciones por lotes futuras, detener envíos nuevos y conservar resultados ya recibidos.

Ejemplos de automatización:

```bash
jflow mine --format json | jq -r '.data.issues[].key'
jflow search --jql 'project = APP AND statusCategory != Done' --all --format json
jflow transition APP-123 --id 21 --yes --no-input --format json
jflow link APP-123 | pbcopy  # Solo macOS; no necesario para usar la CLI.
```

Versionar el esquema de salida por separado de la versión del binario. En v1, permitir campos nuevos y preservar nombres/tipos existentes; cambios incompatibles requieren nueva versión de esquema y guía de migración.

## 14. Diseño para ampliar funciones

### 14.1 Registro de acciones interno

Cada acción declara un ID estable, nombre visible, intención, si muta Jira, capacidades requeridas y un preparador/ejecutor. Ejemplos de IDs: `issue.start`, `issue.done`, `issue.comment.add`.

El registro permite a la TUI construir una paleta contextual y a la CLI exponer comandos, manteniendo los casos de uso compartidos. Los formularios son metadatos de entrada; no incrustar componentes Bubble Tea en el dominio. Validar IDs duplicados al arrancar.

Para añadir una función:

1. Definir el caso de uso y su contrato de entrada/salida.
2. Añadir un puerto pequeño si falta una capacidad del proveedor.
3. Implementar el adaptador y fixtures HTTP.
4. Registrar la acción y exponer el comando.
5. Incorporarla a la TUI si es útil de forma interactiva.
6. Actualizar ayuda, contrato JSON, permisos y pruebas.

No añadir reflexión ni un framework de plugins para el MVP. Tampoco usar el paquete `plugin` de Go como ABI distribuida; un registro compilado es suficiente para las primeras entregas.

### 14.2 Capacidades de proveedor

`Capabilities` expresa soporte del adaptador, por ejemplo comentarios, edición, sprints o worklogs. Diferenciar `supported`, `unsupported` y `unknown` cuando no se haya comprobado. Las capacidades no garantizan permisos sobre un issue concreto; una respuesta 403 afecta al contexto comprobado y no debe deshabilitar globalmente todo el perfil.

Jira Data Center requerirá su propio adaptador: autenticación PAT según versión, rutas y paginación correspondientes, identidades diferentes de `accountId`, texto enriquecido y metadatos propios. No implementar un fallback automático de Cloud v3 a Data Center v2 tras un 404. La disponibilidad de PAT y API debe verificarse en la versión desplegada. Fuente: [PAT Data Center](https://developer.atlassian.com/server/jira/platform/personal-access-token/).

### 14.3 Extensiones externas futuras

Si el uso real justifica extensiones, usar procesos explícitamente registrados por ruta con JSON versionado por stdin/stdout. La invocación será `jflow extension run NOMBRE ...`; no ejecutar binarios desconocidos por una errata en un comando. Tiempo máximo, tamaño de respuesta y códigos de salida definidos.

La extensión no recibe tokens en el entorno por defecto. Una extensión que necesite datos puede consumir salida JSON explícita de la CLI o solicitar capacidades a través de un protocolo posterior. No prometer sandboxing de procesos locales: una extensión instalada ejecuta código con los permisos del usuario.

## 15. Estrategia de verificación

### 15.1 Pruebas unitarias significativas

- Constructor JQL: comillas, caracteres especiales, categorías, filtros contradictorios y orden estable.
- Resolver de workflow: regla válida/inválida, dos destinos Done, close distinto de done, noop y transición de bucle explícita.
- Progreso: denominador cero, datos parciales, cancelados, estimación ausente y consumo superior al 100%.
- Mapeo Cloud: categorías desconocidas, usuario ausente, fechas y campos personalizados faltantes.
- ADF: listas/código, nodos desconocidos, texto plano de ida y sanitización ANSI/OSC.
- Configuración: precedencia, migraciones, versión futura y aislamiento entre perfiles.
- Serialización JSON: contratos públicos y campos nulos, no snapshots de estructuras internas completas.

### 15.2 Pruebas de integración con `httptest`

Casos obligatorios:

| Caso | Evidencia esperada |
| --- | --- |
| Token con scopes | Base gateway correcta; URL navegable conserva sitio |
| Token sin scopes | Base del sitio; cabecera Basic válida y nunca registrada |
| Paginación con cursor | Recupera páginas, conserva filtros y detecta cursor repetido |
| 429 con Retry-After | Espera mediante reloj falso o termina por plazo; sin espera real larga |
| Transición con campo requerido | No hay POST si falta valor; payload válido al completarlo |
| Dos transiciones Done | No se envía nada en modo no interactivo sin elección |
| Cambio entre vista previa y envío | Se invalida la preparación y no se aplica la antigua |
| Timeout después de envío | No duplica POST; resultado unknown y consulta posterior |
| 204 seguido de fallo de lectura | Resultado accepted_unverified; no éxito confirmado |
| Automatización mueve nuevamente el issue | Muestra estado observado y diferencia de destino |
| 401, 403 y 404 | Códigos distintos, sin fallback engañoso a caché |
| Redirect a otro origen | La credencial no alcanza el destino |
| Lectura parcial | Se mantienen elementos obtenidos y marca de incompletitud |

### 15.3 CLI, TUI y sistemas operativos

Ejecutar el binario desde pruebas con entradas y salidas controladas. Confirmar JSON parseable, stderr separado, comportamiento sin TTY, ayuda/completado y códigos de salida. Usar mocks de Browser/SecretStore en pruebas habituales.

Pruebas TUI: navegación, foco, formulario, cancelación, resize, respuesta HTTP atrasada y tema sin color. Los snapshots de pantallas deben usar reloj y ancho fijos. Añadir una prueba con pseudo-terminal para comprobar restauración de cursor/eco tras salir; no depender solo de snapshots.

Pruebas manuales mínimas de release: Linux con escritorio y sesión SSH; macOS con Terminal/iTerm2; terminal estrecha y `TERM=dumb`; keyring disponible/bloqueado/ausente; apertura de navegador; Unicode y texto largo. Construir arm64 no sustituye probar el binario arm64: documentar qué combinaciones fueron ejecutadas realmente.

### 15.4 Validación contra Jira real

Antes de etiquetar v1, usar un proyecto de pruebas con permiso explícito para crear/cambiar issues de prueba. Preparar al menos: un issue normal, uno con campos obligatorios, uno con dos transiciones a Done, uno cerrado y un padre con subtareas.

Comprobar login, asignados a mí, lectura, enlace, inicio, finalización, cierre mapeado y confirmación del estado en Jira web. No usar issues de producción como pruebas. Las credenciales y cuerpos reales no deben almacenarse como fixtures. Limpiar solo los recursos de prueba que se hayan autorizado para ello.

Si no hay un sitio de pruebas, se puede entregar una versión candidata con integración simulada; debe declararse que no se ha validado con un tenant real y no marcar ese criterio como cumplido.

## 16. Plan de trabajo por fases

Estimaciones orientativas en días laborables para una persona familiarizada con Go; no son compromiso de calendario. Incluyen implementación, pruebas y documentación por fase. El trabajo puede organizarse como PRs sucesivos; no se requiere delegación entre agentes para seguir este plan.

| Fase | Duración estimada | Dependencias | Resultado |
| --- | --- | --- | --- |
| F0: base técnica | 1–2 días | Ninguna | Repo, versiones, contratos y decisiones fijadas |
| F1: configuración y acceso | 2–3 días | F0 | Perfiles, secretos, login, me y doctor |
| F2: lectura CLI | 3–4 días | F1 | mine, search, show, link y open |
| F3: workflows | 4–6 días | F2 | Transiciones y start/done/close verificables |
| F4: progreso y scripting | 2–3 días | F2; F3 para resultados de escritura | Métricas, JSON y errores estabilizados |
| F5: TUI | 4–6 días | F2 y F3; integrar F4 | Interfaz navegable y acciones completas |
| F6: distribución y aceptación | 3–4 días | F1–F5 | Binarios, documentación y validación real |
| F7: productividad | 5–8 días | Entrega A | Funciones de Entrega B |

Entrega A: aproximadamente 19–28 días laborables, 4–6 semanas, sujeto a acceso al tenant y complejidad de workflows. Data Center, OAuth y extensiones se estiman después de una investigación específica y no están incluidos.

### F0. Base técnica y contratos

Tareas:

- Crear repositorio y módulo Go bajo el propietario real; usar nombre local provisional hasta conocerlo.
- Fijar toolchain, versiones de Cobra/Charm/keyring y herramientas CI; comprobar compilación de las cuatro plataformas.
- Probar viabilidad de almacenamiento de secretos en macOS sin exponer token en argumentos; registrar decisión.
- Definir tipos de dominio, errores, puertos iniciales y formato JSON v1.
- Crear ADRs para Go/CLI+TUI, Cloud-first, transiciones, secretos y caché.
- Construir fixtures sintéticos y catálogo de endpoints/scopes por comando.

Criterio de salida: `jflow version` y `--help` funcionan; el proyecto compila; CI mínima pasa; ninguna función core depende de una API de TUI.

### F1. Configuración, secretos y autenticación

Tareas:

- Implementar rutas, esquema JSON, migración y precedencia.
- Implementar los backends de credenciales y modo de sesión sin persistencia.
- Crear cliente HTTP con cancelación, URLs derivadas y redacción.
- Implementar login/logout/status, profile list/use, me y doctor.
- Cubrir ambos tipos de API token, credencial incorrecta y keyring bloqueado.

Criterio de salida: se puede configurar un perfil e identificar la cuenta; el token no aparece en archivos de configuración, salida ni logs; cambiar perfil aísla identidad y caché.

### F2. Lectura y enlaces

Tareas:

- Constructor JQL, endpoint mejorado, paginación y selección de campos.
- Implementar mine/list/search/show; normalizar ADF y secciones paginadas.
- Implementar link/open y sus adaptadores Linux/macOS.
- Caché de memoria, `--refresh`, opciones comunes y listado en texto/tabla.
- Pruebas HTTP y del binario sin TTY.

Criterio de salida: el usuario consulta sus pendientes, filtra, revisa un issue y abre su URL; no hay solicitudes extra por cada fila; los resultados truncados están identificados.

### F3. Motor de workflows y mutaciones

Tareas:

- Obtener transiciones y metadatos de campos; formularios básicos.
- Implementar prepare/confirm/revalidate/apply/verify.
- Crear reglas mapeadas, commands transitions/transition/start/done/close y `workflow map`.
- Soportar `--transition-id`, `--fields-file`, `--yes`, `--no-input` y `--dry-run`.
- Cubrir ambigüedad, validación, carreras y respuesta incierta sin duplicar envío.

Criterio de salida: iniciar/completar/cerrar funciona con nombres arbitrarios y campos exigidos; no hay IDs globales hardcodeados ni transiciones automáticas de varios pasos.

### F4. Progreso y contratos de automatización

Tareas:

- Implementar progress/summary con indicadores definidos y alcance explícito.
- Completar JSON v1 y códigos de salida para lecturas y mutaciones.
- Añadir caché persistente opt-in, comandos cache y modo offline.
- Documentar pipelines y métricas desconocidas/parciales.

Criterio de salida: un script puede procesar consultas y distinguir éxito, parcialidad e incertidumbre; ninguna barra inventa avance a partir del nombre del estado.

### F5. Interfaz interactiva

Tareas:

- Modelo principal, listado/detalle, búsqueda, paginación, formularios y diálogos.
- Conectar exclusivamente a casos de uso existentes; no duplicar el resolver de workflows.
- Incorporar estilos, tamaños, ayuda, foco, cancelación y controles de respuesta obsoleta.
- Implementar refresh y feedback posterior a acciones.
- Validar pseudo-terminal y experiencia manual en ambos sistemas.

Criterio de salida: el recorrido consultar → iniciar → revisar → completar → abrir navegador se realiza íntegramente con teclado, con terminal restaurada al salir.

### F6. Distribución y entrega

Tareas:

- Configurar GoReleaser, CI de cuatro artefactos, checksums y changelog.
- Documentar instalación/desinstalación, keyring, SSH, proxies, perfiles y workflows.
- Generar ayuda y completado desde Cobra.
- Ejecutar aceptación en Jira de pruebas y registrar versiones/sistemas probados.
- Revisar dependencias, licencias y fugas de secretos; resolver defectos core.

Criterio de salida: satisfacer la lista de aceptación de la sección 18 y entregar artefactos verificables, no solo código fuente.

### F7. Productividad

Implementar en orden: comentarios → asignación a mí → edición de campos permitidos → reabrir → vistas/favoritos → tablero personal → sprints → worklogs → contexto Git. Cada función agrega sus pruebas de error/permisos y contrato de salida. No dejar los comandos de Entrega B visibles como si funcionaran antes de implementarlos.

Para editar, obtener capacidades/metadatos y validar campos; para worklogs, hacer explícita la política de actualización de estimación restante y mostrarla antes del envío. `start` cambia estado, no inicia un reloj. Un cronómetro futuro tendrá estado local independiente y publicará tiempo solo bajo una acción explícita.

## 17. Distribución, CI y objetivos operativos

Artefactos: `jflow_<version>_linux_amd64.tar.gz`, `linux_arm64`, `darwin_amd64` y `darwin_arm64`, cada uno con binario, licencia y README de instalación. Generar checksums y, cuando exista infraestructura de firma, firma/procedencia verificable. GoReleaser es la herramienta propuesta para automatizar builds y empaquetado. Fuente: [GoReleaser](https://goreleaser.com/getting-started/).

Homebrew tendrá un tap bajo el propietario real del repositorio; no documentar un `brew install` ficticio como ya disponible. Antes de publicar, definir licencia y responsables. Una release pública de macOS debe evaluar firma/notarización y documentar el estado real de Gatekeeper; no recomendar desactivar sus controles como instalación normal.

La matriz inicial de validación se centrará en Ubuntu LTS y las dos versiones estables más recientes de macOS disponibles al publicar. Registrar versiones y arquitecturas concretas en `compatibility.md`; comprobar los mínimos del toolchain elegido y no prometer soporte de todos los Linux/macOS históricos.

CI propuesta:

```text
Formato + vet + lint
        ↓
Pruebas unitarias/integración + race donde se soporte
        ↓
Build Linux/macOS × amd64/arm64
        ↓
Smoke tests en runners disponibles + revisión de contratos JSON
        ↓
Tag de release → empaquetado → checksums → publicación autorizada
```

Comandos de desarrollo que debe implementar el Makefile:

```bash
make fmt-check
make lint
make test
make test-race
make build
make release-check
```

Base de verificaciones: `go test ./...`, `go vet ./...`, `go test -race ./...` donde esté soportado y escaneo con `govulncheck` fijado a una versión. Fijar acciones CI por commit revisado y dar permisos mínimos a jobs de publicación. Las pruebas habituales no requieren credenciales Jira.

Objetivos de rendimiento del producto, a medir y registrar en hardware/fixture conocido:

- `version`/`help`: menos de 150 ms en máquina de desarrollo de referencia.
- Primera respuesta visual de TUI: menos de 200 ms, mostrando carga si falta red.
- Navegación sobre 1.000 elementos cargados: latencia de entrada inferior a 50 ms.
- Lectura inicial de 50 issues: objetivo de menos de 2 s con servidor de prueba de latencia controlada; Jira real puede superar esa cifra.
- Memoria objetivo: menos de 100 MB con 1.000 issues de listado y un detalle abierto.

Estas cifras son objetivos por verificar, no garantías ni mediciones ya realizadas. Priorizar corrección y cancelación antes de optimizaciones especulativas.

## 18. Criterios de aceptación de la primera versión

La Entrega A solo se considera terminada cuando se cumpla lo siguiente:

- [ ] Existen binarios Linux y macOS para amd64/arm64, con matriz de ejecución real documentada.
- [ ] El usuario puede autenticar un perfil y consultar su propia identidad.
- [ ] Los tokens con scopes y sin scopes usan sus bases correctas.
- [ ] `mine` devuelve asignados a mí con filtros y paginación correctos.
- [ ] `show` presenta estado, resolución, descripción y secciones adicionales sin romperse ante campos ausentes.
- [ ] `progress` distingue datos reales, parciales y desconocidos.
- [ ] `link` genera URL correcta sin red y `open` funciona o entrega un diagnóstico útil por plataforma.
- [ ] `start`, `done` y `close` respetan transiciones y reglas de cada workflow.
- [ ] Una ambigüedad no produce ningún cambio sin resolverla.
- [ ] Se admiten campos obligatorios comunes y datos explícitos para automatización.
- [ ] `--dry-run` nunca emite una mutación Jira.
- [ ] Un timeout después del envío no causa un reintento ciego.
- [ ] La TUI permite el recorrido principal con teclado y restaura la terminal.
- [ ] La salida JSON es estable, válida y apta para scripts.
- [ ] Los secretos no aparecen en configuración, historial propio, argumentos de la CLI ni logs.
- [ ] Funciona una sesión sin keyring mediante credencial efímera.
- [ ] Caché y configuración están aisladas por perfil/identidad.
- [ ] Las pruebas definidas pasan y existe guía de instalación y uso.
- [ ] Se registra una validación contra Jira de pruebas, o se entrega explícitamente como candidata sin ese criterio cumplido.

## 19. Riesgos concretos y decisiones pendientes acotadas

| Riesgo o incógnita | Tratamiento |
| --- | --- |
| Workflow con validadores externos | Conservar formulario, mostrar error servidor y ofrecer abrir navegador |
| “Cerrar” distinto de “completar” | Resolver por transición/regla; nunca asumir equivalencia |
| Indexación tardía tras escritura | Verificar detalle, invalidar caché y reconciliar búsquedas cuando corresponda |
| Token vencido o permisos limitados | Diagnóstico preciso sin solicitar permisos administrativos generales |
| Terminal/SSH sin portapapeles o navegador | Enlace imprimible y CLI plain como funciones independientes |
| macOS keyring expone argumento al guardar | Investigar backend en F0 y resolver antes de persistencia pública |
| Campos personalizados distintos por sitio | Descubrimiento y mapeo explícito, IDs locales al perfil |
| Tipo de Jira aún desconocido | Cloud como entrega inicial; si el tenant real es Data Center, priorizar su adaptador antes de validar allí |
| Nombre/licencia/propietario por definir | Usar nombre provisional local; resolver antes de publicación |

Ninguna incógnita impide construir el núcleo con fixtures. El agente no debe inventar credenciales, cloud IDs ni permisos para cerrar esas decisiones. Si el Jira objetivo resulta ser Data Center, conservar los casos de uso y sustituir el proveedor planificado; comunicar el impacto en cronograma.

## 20. Instrucciones de entrega para el agente implementador

Comenzar por F0 y avanzar por dependencias. El objetivo de este documento es que las decisiones comunes ya estén fijadas; no requiere volver a preguntar qué lenguaje, interfaz o modelo de comandos usar. Resolver detalles menores con las reglas aquí descritas y registrar cambios de arquitectura mediante ADR.

En cada fase entregar código ejecutable, pruebas proporcionadas al riesgo y documentación de uso. Mantener una lista de pendientes con funciones efectivamente disponibles. Una interfaz atractiva con mutaciones simuladas no satisface Entrega A; tampoco la satisface una CLI funcional que omite Linux/macOS o la TUI solicitada.

Orden recomendado del primer incremento vertical: `auth login` → `me` → `mine` → `show` → `link/open`. El segundo incremento: `transitions` → preparación → `start` → `done/close` → verificación. Solo entonces completar la TUI sobre esos mismos servicios.

Al finalizar, entregar repositorio, artefactos de release, guía rápida, configuración de ejemplo sin secretos, informe de pruebas, matriz de compatibilidad y limitaciones observadas. Toda afirmación de compatibilidad con un tenant o sistema debe apoyarse en pruebas registradas.

## 21. Referencias y mantenimiento del plan

Las referencias técnicas están enlazadas junto a la decisión o comportamiento que sustentan. Consultadas el 6 de septiembre de 2026. Revalidar autenticación, endpoints, scopes y versiones antes de iniciar F0 o después de un cambio relevante de Atlassian.

La arquitectura, valores de TTL, comandos, códigos de salida, pantallas, estimaciones y criterios de aceptación son propuestas de este documento. Las fuentes oficiales respaldan las capacidades de las API/bibliotecas, pero no garantizan que el plan esté implementado ni que un tenant concreto permita todas sus operaciones.
