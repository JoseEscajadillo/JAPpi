---
name: new-adr
description: Redacta un Architecture Decision Record para JAPpi en docs/adr/ con el formato y la numeración del proyecto. Úsala cuando el usuario tome o proponga una decisión de arquitectura, de tecnología, de contratos entre servicios, de seguridad o de costes, o pida "escribe un ADR".
---

# Nuevo ADR

1. Lee `docs/adr/README.md` (criterios e índice) y `docs/adr/template.md`.
2. Busca el siguiente número libre en `docs/adr/`, con 4 dígitos.
3. El título es **la decisión**, no el problema: "Usar CloudNativePG para Postgres", no "Cómo gestionar Postgres".
4. Rellena la plantilla:
   - **Contexto:** para alguien que no estuvo en la conversación. Incluye las fuerzas concretas (coste en USD, tamaño del equipo, plazo).
   - **Decisión:** en voz activa, verificable.
   - **Alternativas:** al menos dos, cada una con el motivo concreto del descarte.
   - **Consecuencias:** al menos una negativa. Si no encuentras ninguna, la decisión no está bien entendida.
5. El estado es `Propuesto`, salvo que el usuario diga que el equipo ya lo aprobó.
6. Si reemplaza a otro ADR: en el nuevo, "Reemplaza a NNNN"; en el viejo, cambia **solo** su línea de estado a "Reemplazado por NNNN" (es la única edición permitida en un ADR aceptado).
7. Añade la fila al índice de `docs/adr/README.md`.
8. Si la decisión cambia contenedores o relaciones, actualiza también `docs/c4/README.md`.

Escribe en español, con frases cortas. Nada de relleno ("es importante destacar que...").
