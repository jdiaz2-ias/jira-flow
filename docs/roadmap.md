# Estado de implementación

## F0: base técnica completada

CLI, dominio/puertos, JSON v1, dependencias fijadas, pruebas base, fixtures y CI. Evidencia histórica: [validation-f0.md](validation-f0.md). El repositorio ya está publicado en GitHub bajo `jdiaz2-ias/jira-flow`.

## F1: configuración y acceso implementados

- [x] Rutas Linux/macOS, JSON v1, permisos privados, bloqueo y escritura atómica.
- [x] Perfiles, precedencia y rechazo de esquemas desconocidos; F0 no tenía formato persistido que migrar.
- [x] SecretStore cancelable, referencias aleatorias, validación de límites y credenciales efímeras.
- [x] HTTP con TLS/proxy/CA corporativa, cancelación, límites y rechazo de redirects.
- [x] Ambos métodos de token personal Cloud y mapeo de `myself`.
- [x] `auth login/logout/status`, `profile list/use`, `me`, `doctor`, `config path/validate`.
- [x] Pruebas con HTTPS simulado, keyring inyectado y Keychain real macOS con secreto sintético.
- [ ] Validación real con tenant autorizado y Secret Service Linux; no bloquean las pruebas sintéticas, pero limitan las afirmaciones de compatibilidad.

Evidencia y límites: [validation-f1.md](validation-f1.md).

## F2: lectura y enlaces implementados

- [x] JQL escapado, filtros, orden permitido y selección de campos de listado.
- [x] POST search/jql, cursores por perfil/identidad/consulta, deduplicación y resultados parciales.
- [x] `mine`, `list`, `search`, `show`, descripción ADF, subtareas y vínculos.
- [x] Comentarios e historial paginados bajo demanda, con offsets y límites por sección.
- [x] `link`, `open`, adaptadores nativos y proyecto predeterminado.
- [x] Caché acotada por proceso, refresco, offline explícito y salida texto/tabla/JSON.
- [x] Pruebas HTTP, CLI sin TTY, cancelación, reintentos, límites e aislamiento.
- [ ] Validación de consultas F2 contra un tenant real.

Evidencia y límites: [validation-f2.md](validation-f2.md). El usuario confirmó la conexión real de F1; esa confirmación no acredita aún las lecturas de F2.

## Siguiente incremento

F3: transiciones. F4: progreso/scripting. F5: TUI. F6: empaquetado/aceptación. F7: productividad. Criterios en el [plan completo](implementation-plan.md).
