# 0002. Microservicios orientados a eventos sobre NATS JetStream

- **Estado:** Aceptado
- **Fecha:** 2026-09-29
- **Decisores:** equipo JAPpi

## Contexto

El ciclo de vida de un despliegue tiene pasos lentos y que pueden fallar: recibir un push, construir la imagen (minutos), aplicarla en el clúster, comprobar su salud. Si cada paso llamara al siguiente por HTTP, una caída del builder perdería trabajo y bloquearía a quien lo llamó. Además queremos que el sistema sea extensible: notificaciones, métricas o facturación por uso deberían poder engancharse sin tocar los servicios existentes.

## Decisión

- Los servicios se comunican **solo por eventos** publicados en **NATS JetStream**, en un stream `JAPPI` con subjects `jappi.<tipo>`.
- Cada servicio usa un **consumidor durable** con su nombre: si se cae, al volver retoma donde lo dejó.
- Todo evento lleva un **ID estable** (cabecera `Nats-Msg-Id`); JetStream descarta duplicados en una ventana de 10 minutos, y los handlers además son idempotentes.
- Los contratos viven en `pkg/contracts/events` y se documentan en [`docs/events.md`](../events.md).
- Solo el dashboard habla HTTP con el control-plane (consultas y comandos del usuario).

## Alternativas consideradas

- **Kafka / Redpanda:** más potentes para volúmenes enormes, pero mucho más pesados de operar en un VPS pequeño. No necesitamos particiones ni retención de meses.
- **RabbitMQ:** buen broker, pero NATS da en un binario de ~20 MB colas persistentes, request/reply y key-value, y su cliente de Go es de primera.
- **HTTP/gRPC síncrono entre servicios:** más simple de depurar, pero acopla disponibilidad y hace frágiles los flujos largos.

## Consecuencias

- **Positivas:** los servicios se despliegan y fallan por separado; añadir un consumidor nuevo no toca a nadie; los reintentos vienen gratis.
- **Negativas:** consistencia eventual (el dashboard puede ir un segundo por detrás); depurar un flujo exige seguir eventos (mitigado con logs estructurados que incluyen el ID del evento); hay que escribir handlers idempotentes siempre.
