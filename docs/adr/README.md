# Architecture Decision Records (ADR)

Cada decisión de arquitectura que cueste deshacer queda escrita aquí: qué se decidió, por qué y qué se sacrificó. Así nadie tiene que adivinar dentro de seis meses por qué algo es como es.

## Cuándo escribir un ADR

Escribe uno si la decisión cumple **al menos una** de estas condiciones:

- Añade o cambia una tecnología (base de datos, bus, lenguaje, librería central).
- Cambia cómo se comunican los servicios o el contrato de un evento de forma incompatible.
- Crea o elimina un servicio.
- Afecta a la seguridad, al aislamiento entre clientes o a los costes.
- Se discutió más de 15 minutos en el equipo.

## Proceso

1. Copia [`template.md`](template.md) como `NNNN-titulo-en-kebab.md` con el siguiente número libre.
2. Estado `Propuesto`, abre un PR y pide revisión a los otros dos.
3. Al aprobarse, cambia el estado a `Aceptado` en el mismo PR.
4. Un ADR aceptado **no se edita**: si la decisión cambia, se escribe uno nuevo que lo `Reemplaza` y se marca el viejo como `Reemplazado por NNNN`.

## Índice

| # | Decisión | Estado |
|---|----------|--------|
| [0001](0001-registrar-decisiones-con-adr.md) | Registrar las decisiones de arquitectura con ADR | Aceptado |
| [0002](0002-microservicios-orientados-a-eventos-con-nats.md) | Microservicios orientados a eventos sobre NATS JetStream | Aceptado |
| [0003](0003-go-para-los-servicios.md) | Go para los servicios del backend | Aceptado |
| [0004](0004-k3s-sobre-vps.md) | K3s sobre VPS como plano de datos | Aceptado |
| [0005](0005-nextjs-multi-zones-para-microfrontends.md) | Next.js con Multi-Zones para los microfrontends | Aceptado |
| [0006](0006-arquitectura-hexagonal-y-solid.md) | Arquitectura hexagonal y SOLID en cada servicio | Aceptado |
| [0007](0007-monorepo-con-un-solo-modulo-go.md) | Monorepo con un solo módulo Go | Aceptado |
| [0008](0008-deteccion-de-monorepos-y-cableado-automatico.md) | Detección de monorepos y cableado automático de variables | Aceptado |
| [0009](0009-dominios-reservados-antes-del-primer-deploy.md) | Dominios reservados antes del primer deploy | Aceptado |
| [0010](0010-prueba-gratuita-con-tarjeta.md) | Prueba gratuita con tarjeta registrada | Propuesto |
| [0011](0011-deployer-sin-estado-con-deploy-requested.md) | Deployer sin estado, alimentado por deploy.requested | Aceptado |
| [0012](0012-aislamiento-de-proyectos-en-el-cluster.md) | Aislamiento de los proyectos en el clúster | Aceptado |
| [0013](0013-monolito-modular-hasta-validar-el-producto.md) | Monolitos modulares hasta validar el producto; extracción posterior | Propuesto |

Los ADR 0002, 0005 y 0011 describen decisiones que siguen reflejadas en el código actual. El ADR-0013 propone la arquitectura objetivo; al aprobarlo y completar la migración, se actualizará el estado histórico de esos ADR según el proceso anterior.
