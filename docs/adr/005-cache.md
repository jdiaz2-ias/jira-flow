# ADR-005: caché privada y opt-in

Estado: aceptada para próximas fases. Fecha: 2026-09-06.

Decisión: caché inicial en memoria; persistencia opcional en F4. Aislar por proveedor, sitio/base, perfil, identidad, consulta y esquema. Las mutaciones requieren lecturas frescas; un fallo de autenticación no se oculta con datos anteriores.

Consecuencia: F0 no crea archivos de configuración, tokens ni caché al ejecutar el binario. TTL y tamaños seguirán el plan; los resultados posteriores indicarán fecha, fuente y parcialidad.
