---
name: systems-design
description: Diseña funcionalidades de JAPpi respetando su arquitectura (microservicios por eventos, hexagonal, SOLID, ADR, C4). Úsala antes de escribir código cuando la tarea añade o cambia un servicio, un caso de uso, un puerto o adaptador, un evento, una tabla, o toca varios servicios a la vez; también para revisar si un diseño o PR encaja con la arquitectura.
---

# Diseño de sistemas en JAPpi

Esta skill convierte una petición ("quiero rollback", "añade Stripe") en un diseño que encaja con la arquitectura **antes** de escribir código. La fuente de verdad es:

- `docs/adr/` (decisiones; sobre todo 0002, 0006 y 0008);
- `docs/c4/README.md` (arquitectura);
- `docs/system-design.md` (capacidad, cuellos de botella, SLO y costes);
- `docs/events.md` (contratos);
- `CONTRIBUTING.md` (reglas de código).

Léelos si no los tienes en contexto.

## Proceso

Sigue los pasos en orden y **presenta el diseño al usuario antes de implementar** si toca más de un servicio o crea un evento.

### 1. Ubicar la funcionalidad

- ¿Qué servicio es **dueño** de los datos o de la decisión? Una funcionalidad tiene un solo dueño. Si dudas entre dos, el dueño es quien guarda el estado.
- ¿Hace falta un servicio nuevo? Solo si tiene datos propios, un ciclo de despliegue distinto o escala de otra forma. En la duda, **no**: añade un caso de uso a uno existente. Un servicio nuevo requiere un ADR.

### 2. Modelar el dominio primero

- Las entidades, los value objects y las reglas van en `internal/domain`, en Go puro con la librería estándar. Sin JSON de APIs externas ni SQL.
- Si hay estados, modélalos como máquina de estados con transiciones explícitas (ver `domain/deployment`).
- Escribe las pruebas del dominio con tablas de casos y datos en memoria.

### 3. Caso de uso y puertos

- Hay un caso de uso por intención del usuario o del sistema (`HandlePush`, `RequestRollback`). Va en `internal/app`, es un struct con sus dependencias como campos y tiene un método `Execute`.
- Declara en `app` **solo** las interfaces que el caso de uso usa, con 1–3 métodos cada una (segregación de interfaces), nombradas por lo que hacen (`DeploymentSaver`, no `IDeploymentRepository`).
- El caso de uso no sabe si por debajo hay NATS, Postgres o memoria.

### 4. Comunicación entre servicios

- **Entre servicios, solo eventos** (ADR-0002). Si un servicio necesita saber algo de otro, se suscribe a su evento y guarda su propia copia de lo que le importa.
- **Evento nuevo:** constante y payload en `pkg/contracts/events`, fila en `docs/events.md`, y el ID que lo hace idempotente (¿de dónde sale un ID estable?).
- **Idempotencia:** responde por escrito qué pasa si el handler recibe el evento dos veces o falla a mitad de camino.
- **Secretos:** nunca van en un evento; van referencias.

### 5. Adaptadores y composición

- `adapters/in`: HTTP o consumidores que **traducen** y llaman al caso de uso. Sin lógica de negocio.
- `adapters/out`: implementaciones de los puertos. Si hay más de una implementación de un puerto, escribe una suite de contrato compartida (ver `pkg/eventbus/bustest`).
- `cmd/<servicio>/main.go`: el único sitio que instancia implementaciones concretas.

### 6. Documentar

- **ADR** (skill `new-adr`) si la decisión cumple los criterios de `docs/adr/README.md`.
- **C4:** actualiza `docs/c4/README.md` si cambian contenedores, sistemas externos o relaciones.
- **System Design:** si la funcionalidad cambia la carga esperada, crea o resuelve un cuello de botella, afecta a un SLO o cambia el coste por plan, actualiza `docs/system-design.md` (supuestos, cálculo y tabla afectada).
- **Roadmap:** mueve la tarea en `docs/roadmap.md`.

### 7. Verificar

```bash
gofmt -l . && go vet ./... && go test ./...
```

`TestHexagonalRules` falla si una capa importa algo prohibido; nunca se desactiva, se corrige el diseño.

## Formato del diseño a presentar

```markdown
**Dueño:** <servicio>
**Dominio:** <tipos y reglas nuevas>
**Caso de uso:** <Nombre>.Execute(...) — puertos: <Interfaz{métodos}>
**Eventos:** publica <tipo> (id: <origen del ID>) / consume <tipo>
**Idempotencia:** <qué pasa con una reentrega>
**Adaptadores:** in: <...>  out: <...>
**Docs:** ADR <sí/no, por qué> · C4 <nivel afectado>
**Escala:** <qué pasa con 10× la carga del §2 de system-design.md; ¿nuevo cuello de botella?>
**Riesgos:** <seguridad, coste, aislamiento entre clientes>
```

## Señales de alarma

Si aparece alguna, para y rediseña:

- Un servicio llama por HTTP a otro servicio del backend.
- `domain` importa `encoding/json` para parsear la respuesta de una API externa, o importa algo de `adapters`.
- Una interfaz con más de 5 métodos, o una interfaz con una sola implementación y sin consumidor que la necesite.
- Un `switch` sobre tipos o frameworks que crece con cada caso (usa estrategias, como los `Detector` de `domain/stack`).
- Un secreto en un log, en un evento o en el entorno de build.
- Código del cliente ejecutándose sin cuota, sin NetworkPolicy o fuera de su namespace.
