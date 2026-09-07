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

## Siguiente incremento

F2: consultas y enlaces (`mine`, `search`, `show`, `link`, `open`). F3: transiciones. F4: progreso/scripting. F5: TUI. F6: empaquetado/aceptación. F7: productividad. Criterios en el [plan completo](implementation-plan.md).
