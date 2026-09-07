# ADR-001: Go y entradas CLI/TUI compartidas

Estado: aceptada para F0. Fecha: 2026-09-06.

Decisión: usar Go 1.27.1, Cobra y Charm v2. El dominio/casos de uso no importan frameworks de presentación. TUI y CLI comparten servicios futuros.

Consecuencia: entrega como binario, compilación cruzada y pruebas sin terminal. F0 solo registra comandos funcionales. El tag foundation comprueba las dependencias de la futura TUI sin iniciarla ni incluirlas en el ejecutable normal.

El módulo es `jira-flow.local/jflow`, provisional hasta conocer repositorio/propietario de publicación. No se asigna un remoto inventado.
