# Contrato de workflows

En F0 existen los tipos `Intent`, `Transition`, `FieldSpec`, `PreparedAction` y `ApplyResult`. El algoritmo se implementará en F3, con los escenarios del plan.

`start`, `done` y `close` no son nombres universales de estados. El resolver consultará transiciones frescas; elegirá una regla validada o solicitará decisión cuando haya ambigüedad. Resolver y Cancelar pueden llegar ambos a Done. Nunca se codifican IDs globales.

El flujo será preparar → validar campos → confirmar → revalidar → aplicar una vez → verificar. Un POST que pudo llegar al servidor no se reintentará ciegamente. Los resultados reservados son `verified`, `accepted_unverified`, `unknown`, `failed` y `noop`.

Los fixtures `transitions.json` y `transitions-ambiguous.json` incluyen destinos y campos para implementar esas pruebas. `transition-required-error.json` representa un rechazo confirmado. Todos son sintéticos.
