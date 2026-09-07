# Estado de implementación

## F0: base técnica

- [x] Repositorio local, rama main y README.
- [x] Módulo local, toolchain y dependencias fijadas.
- [x] CLI `version`/ayuda y formato JSON.
- [x] Dominio, puertos y catálogo de errores iniciales.
- [x] Pruebas de contrato, binario y separación de capas.
- [x] Auditoría de fuente macOS keyring y prueba de regresión.
- [x] ADRs y fixtures sintéticos.
- [x] Catálogo de endpoints/scopes para próximos comandos.
- [x] CI configurada para Linux/macOS; publicación desactivada.

La evidencia de ejecución, compilación cruzada y límites se registra en [validation-f0.md](validation-f0.md). “CI configurada” no significa que haya corrido en GitHub; este repositorio sigue siendo local.

## F1: siguiente incremento

1. Implementar rutas y configuración JSON versionada con perfiles y precedencia.
2. Implementar `SecretStore`, credenciales efímeras y validación de límites antes de guardar.
3. Añadir transporte HTTP con cancelación y redacción, sin fuga de Authorization por redirects.
4. Implementar ambos métodos de token personal y mapear `myself` al dominio.
5. Exponer `auth login/logout/status`, `profile list/use`, `me`, `doctor`.
6. Probar con `httptest`, fixtures y keyring inyectado; validación real solo con tenant autorizado.

## Fases posteriores

F2: consultas/enlaces. F3: transiciones. F4: progreso y scripting. F5: TUI. F6: empaquetado/aceptación. F7: productividad. Consultar el [plan completo](implementation-plan.md) para criterios de salida y detalles.
