# ADR-002: Jira Cloud como primer proveedor

Estado: aceptada. Fecha: 2026-09-06.

Decisión: REST v3 con puertos `IssueReader` y `TransitionGateway`; rutas y DTOs vivirán en `provider/jiracloud`. Búsqueda mejorada POST `/search/jql` con cursor. La base REST se deriva del tipo de token y el enlace humano siempre usa el sitio.

Consecuencia: Data Center requiere otro adaptador y pruebas; no habrá fallback por un 404 ni sustitución automática de v3 por v2. Ninguna llamada HTTP forma parte de F0.
