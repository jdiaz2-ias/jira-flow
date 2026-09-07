# ADR-004: secretos y viabilidad de Keychain

Estado: aceptada para diseño; validación runtime macOS pendiente. Fecha: 2026-09-06.

Decisión: mantener `SecretStore` inyectable y fijar `github.com/zalando/go-keyring v0.2.8`. Sin persistencia disponible, usar credencial de sesión; nunca fallback de texto plano.

La revisión de [keyring_darwin.go v0.2.8](https://github.com/zalando/go-keyring/blob/v0.2.8/keyring_darwin.go) muestra que Set inicia `/usr/bin/security -i` y envía el comando con el secreto por stdin. Get/Delete llevan identificadores del recurso; el valor no forma parte del argv de Set. El test `TestPinnedMacKeyringDoesNotPassSecretInArgv` verifica la estructura de esa invocación en el módulo descargado. No es una prueba dinámica de Keychain.

La implementación tiene un límite de 4096 bytes para el comando interactivo y construye/valida ese comando después de iniciar el proceso. En F1 se debe comprobar el tamaño con una cota conservadora antes de invocar Set, y diseñar cancelación/gestión del proceso: la biblioteca no acepta context. Un token grande no se truncará; se rechazará su persistencia y podrá usarse efímeramente. Si esos requisitos no se satisfacen con el wrapper, sustituir el backend mediante el puerto.

No registrar stdout/stderr de lectura de secretos; tampoco respuestas que puedan contener valores. La redacción de Secret es defensa de presentación, no cifrado en memoria. El test opt-in `TestMacKeychainRoundTrip` está preparado para macOS y se debe ejecutar antes de afirmar soporte persistente validado.

Esta evidencia resuelve la viabilidad de evitar secretos en argv para la versión fijada, sin afirmar que Linux haya probado el llavero de macOS.
