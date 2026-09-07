# ADR-004: secretos y viabilidad de Keychain

Estado: aceptada para diseño; validación runtime macOS pendiente. Fecha: 2026-09-06.

Decisión: mantener `SecretStore` inyectable y fijar `github.com/zalando/go-keyring v0.2.8`. Sin persistencia disponible, usar credencial de sesión; nunca fallback de texto plano.

La revisión de [keyring_darwin.go v0.2.8](https://github.com/zalando/go-keyring/blob/v0.2.8/keyring_darwin.go) muestra que Set inicia `/usr/bin/security -i` y envía el comando con el secreto por stdin. Get/Delete llevan identificadores del recurso; el valor no forma parte del argv de Set. El test `TestPinnedMacKeyringDoesNotPassSecretInArgv` verifica la estructura de esa invocación en el módulo descargado. No es una prueba dinámica de Keychain.

La implementación tiene un límite de 4096 bytes para el comando interactivo y construye/valida ese comando después de iniciar el proceso. En F1 se debe comprobar el tamaño con una cota conservadora antes de invocar Set, y diseñar cancelación/gestión del proceso: la biblioteca no acepta context. Un token grande no se truncará; se rechazará su persistencia y podrá usarse efímeramente. Si esos requisitos no se satisfacen con el wrapper, sustituir el backend mediante el puerto.

No registrar stdout/stderr de lectura de secretos; tampoco respuestas que puedan contener valores. La redacción de Secret es defensa de presentación, no cifrado en memoria. El test opt-in `TestMacKeychainRoundTrip` está preparado para macOS y se debe ejecutar antes de afirmar soporte persistente validado.

Esta evidencia resuelve la viabilidad de evitar secretos en argv para la versión fijada, sin afirmar que Linux haya probado el llavero de macOS.


## Actualización F1 (2026-09-07)

El wrapper de la biblioteca no permite cancelar sus procesos. El backend activo se sustituye por un ejecutor propio `exec.CommandContext`, con timeout de 15 segundos y sin propagar stderr. macOS sigue usando `/usr/bin/security -i`; el comando completo se limita antes de iniciar el proceso y el secreto se codifica en hexadecimal para no introducir comillas ni saltos de línea interpretables. Se valida mediante lectura posterior: el modo interactivo puede terminar con código cero tras fallar un comando. Get decodifica el hexadecimal; los identificadores son aleatorios de 32 caracteres hexadecimales.

Linux usa `secret-tool` por stdin y requiere libsecret-tools/Secret Service. Esta dependencia de runtime permite cancelación sin goroutines abandonadas de la biblioteca. No se ha probado contra Secret Service real en F1. go-keyring permanece fijado únicamente para la auditoría histórica foundation.

El backend macOS pasó una prueba real con secreto sintético (escritura/lectura/eliminación) en arm64. El límite de persistencia es 1500 bytes; una credencial excesiva se rechaza sin iniciar el proceso y puede usarse efímeramente. La sustitución conserva la credencial anterior hasta guardar la configuración nueva; referencias pendientes de limpieza permiten reintentar con logout si el llavero falla después.
