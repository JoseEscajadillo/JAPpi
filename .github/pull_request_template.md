## Qué

<!-- Qué cambia, en una o dos frases. -->

## Por qué

<!-- El problema que resuelve. Enlaza el ADR o el punto del roadmap si aplica. -->

## Cómo probarlo

<!-- Comandos o pasos para que quien revisa lo compruebe. -->

## Definición de terminado ([CONTRIBUTING](../CONTRIBUTING.md#definición-de-terminado))

- [ ] `./scripts/dev.sh check` pasa (gofmt, vet, pruebas)
- [ ] Pruebas para la lógica nueva; prueba de regresión si es un bug
- [ ] Handlers de eventos idempotentes (explica por qué abajo si aplica)
- [ ] Si cambió la API pública, `docs/api.md` y sus ejemplos están actualizados
- [ ] Ningún secreto en código, logs, eventos ni entorno de build
- [ ] ADR / C4 / `docs/events.md` / `docs/system-design.md` actualizados si cambió la arquitectura
- [ ] `docs/roadmap.md` refleja el avance
