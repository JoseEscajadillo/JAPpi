# 0013. Monolitos modulares hasta validar el producto

- **Estado:** Propuesto
- **Fecha:** 2026-10-03
- **Decisores:** César, José y Diego (pendiente de revisión del equipo)
- **Reemplaza como arquitectura objetivo:** ADR-0002 (microservicios y NATS), ADR-0005 (Multi-Zones) y la parte de transporte entre procesos del ADR-0011. Las decisiones de aislamiento del ADR-0012 siguen vigentes.

## Contexto

JAPpi tiene cimientos útiles: detector, contratos de eventos, control-plane, integración de GitHub y deployer. Sin embargo, aún no puede recorrer el camino completo de conectar un repositorio, construirlo y abrir su URL. Tres personas tendrían que integrar y operar varios binarios, un bus y varias aplicaciones Next.js antes de validar ese camino. La UI de producto todavía no está diseñada.

El equipo decidió que la primera etapa entregue un producto funcional y fuerte. La separación en microservicios y Multi-Zones pertenece a la segunda etapa, cuando el flujo completo ya esté medido y sus límites sean claros. **Sprint 3 es una iteración de la primera etapa**, no una orden de extraer servicios ahora.

## Decisión

### Backend

- Un solo proceso desplegable Go expone la API REST, recibe webhooks y ejecuta un trabajador de tareas persistidas. Se compone en `cmd/jappi`; dentro del mismo módulo Go, los módulos `identity`, `repositories`, `projects`, `builds` y `deployments` tienen dominio, casos de uso y adaptadores propios.
- La comunicación entre módulos es por llamadas a interfaces Go pequeñas. PostgreSQL guarda los datos y una cola transaccional de trabajos: al crear un despliegue se guarda el trabajo en la misma transacción. El trabajador toma trabajos con lease y reintentos acotados, y las operaciones deben ser idempotentes. No se requiere NATS para el primer producto.
- El build de código de clientes **no se ejecuta dentro del proceso de la API**. El módulo `builds` crea un Job aislado en K3s/BuildKit, espera su resultado y registra la imagen por digest. El módulo `deployments` usa el adaptador Kubernetes ya existente para aplicar y observar el rollout. Un módulo puede extraerse a otro proceso después sin cambiar su dominio.
- Se reutiliza de forma incremental el código existente de `services/control-plane`, `services/github-integration` y `services/deployer`. Sus binarios y consumidores NATS siguen siendo el estado actual del repositorio hasta que el golden path equivalente funcione en el nuevo proceso; se retiran con pruebas de regresión y un cambio explícito de despliegue.
- La API pública versionada y sus estados se documentan en [`docs/api.md`](../api.md). PostgreSQL es la fuente de verdad; la API lee estado persistido, no la memoria del trabajador.

### Frontend

- Una sola aplicación Next.js en `web/`, un build y un despliegue. La organización interna es por funcionalidades (`auth`, `projects`, `deployments`, `shared`), con componentes y cliente HTTP compartidos. No hay `shell`, `console` ni `billing` como aplicaciones separadas en la primera etapa.
- Primero se construyen la base técnica y el cliente tipado de la API. La experiencia visual se diseña después de cerrar el flujo y las respuestas; el Sprint 3 solo necesita una interfaz mínima de importación, estado y URL.

### Infraestructura y evolución

- Hetzner es el proveedor elegido. La primera instalación usa un entorno pequeño de K3s, PostgreSQL, registry, TLS y copias de seguridad verificadas; se documentan credenciales, restauración y límites antes de exponer proyectos externos. El código de cliente corre aislado por proyecto según ADR-0012.
- La **segunda etapa** podrá extraer módulos a microservicios y dividir la app Next.js en Multi-Zones. Se exige antes: golden path operable, contratos HTTP/eventos estables, métricas de carga o necesidad real de aislamiento de despliegues, y plan de migración reversible. Una fecha o un nombre de sprint por sí solos no justifican la extracción.

## Alternativas consideradas

- **Continuar directamente con microservicios y NATS:** aprovecha el código actual, pero multiplica integraciones, despliegues y fallos operativos antes de tener usuarios.
- **Un único paquete Go y una sola carpeta de páginas Next:** reduce archivos, pero mezcla responsabilidades y dificulta extraer módulos más adelante.
- **Multi-Zones desde el primer dashboard:** permite despliegues independientes de áreas que aún no existen; añade configuración y navegación entre aplicaciones sin valor inmediato.

## Consecuencias

- **Positivas:** un recorrido de producto más corto, menos componentes que operar y contratos claros entre módulos; el código actual útil se conserva durante la transición.
- **Negativas:** la API y el trabajador comparten ciclo de despliegue y recursos; PostgreSQL asume la cola; hay trabajo de migración desde los binarios y eventos actuales. Se mitiga con módulos acotados, trabajos aislados, límites de concurrencia y métricas antes de extraer.
- **Cambios de documentación:** roadmap, API, C4, guías de contribución y desarrollo deben distinguir el código actual de la arquitectura objetivo. Los ADR antiguos permanecen como registro histórico hasta que este ADR sea aceptado y la migración termine.
