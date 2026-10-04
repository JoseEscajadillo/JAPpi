# Hoja de ruta y Sprint 3

**Secuencia:** la **etapa 1 (producto)** se construye como dos monolitos modulares: backend Go y frontend Next.js. Dentro de esta etapa está el **Sprint 3**, una iteración de diez días laborables. Solo después de terminar y operar un producto funcional comienza la **etapa 2 (escala)**, donde se evaluarán microservicios y Multi-Zones. Así distinguimos las etapas que el equipo llama «Sprint 1» y «Sprint 2» de las iteraciones de trabajo.

**Estado al iniciar el Sprint 3:** existen detector, control-plane, integración de GitHub, deployer, pruebas de Kubernetes y CI. El backend todavía son varios binarios unidos por NATS; no hay builder ni API de proyectos, y `web/` no contiene una aplicación. El sprint incluye la consolidación incremental, no presupone que ya ocurrió. Ver [ADR-0013](adr/0013-monolito-modular-hasta-validar-el-producto.md) y el [contrato REST](api.md).

## Etapa 1: producto funcional

1. **Sprint 3: primer golden path.** Un usuario inicia sesión con GitHub, elige un repositorio autorizado con **un servicio HTTP y Dockerfile**, ve el análisis, lo importa, JAPpi construye la imagen, despliega en K3s de Hetzner y muestra estado y URL HTTPS. Un push posterior a la rama configurada dispara un despliegue nuevo. La UI es mínima porque el diseño de producto aún no está definido.
2. **Siguientes iteraciones de producto:** ampliar el detector a monorepos y cableado automático; variables y secretos cifrados; logs y diagnósticos; redeploy y rollback; PostgreSQL/Redis administrados; cuotas, facturación y prueba; copias de seguridad, observabilidad y recuperación. Cada capacidad entra con API, pruebas, experiencia de usuario y operación documentadas. No se anuncia como terminada hasta que el flujo real funciona.
3. **Cierre de etapa:** al menos un proyecto real de prueba recorre importación, build, HTTPS, push, fallo recuperable y rollback; hay restauración probada de PostgreSQL, métricas de duración/error, alertas básicas y una semana de operación supervisada. La revisión del equipo decide si el producto es suficientemente sólido para escalar.

## Etapa 2: escala, después de validar la etapa 1

Extraer primero el módulo que necesite ciclo de despliegue, aislamiento o capacidad independientes, manteniendo contratos y migración reversible. Introducir NATS cuando exista comunicación entre procesos que lo justifique. Dividir Next.js en Multi-Zones solo cuando áreas estables y equipos/cadencias de despliegue distintos aporten valor. La observabilidad debe mostrar el problema que resuelve cada extracción; ADR-0002 y ADR-0005 son antecedentes, no tareas del Sprint 3.

## Sprint 3 · diez días laborables

**Responsables:** César y José desarrollan el backend y, al final, conectan la UI mínima. Diego empieza con la base técnica del frontend durante los días 1–3 y desde el día 4 lidera la infraestructura en **Hetzner**. No se asigna a Diego diseño visual ni pantallas de producto antes de acordar el flujo.

**Alcance cerrado:** un proyecto por repositorio/rama, un servicio HTTP, Dockerfile obligatorio, sin variables secretas del usuario, bases de datos administradas, pagos, logs en vivo ni rollback. La API y la pantalla muestran con claridad que esas funciones llegarán después. La seguridad de webhooks y del código de clientes, TLS, cuotas y copias de seguridad sí son obligatorias.

| Día | César · backend / integración | José · backend / calidad | Diego · frontend inicial → Hetzner | Entregable verificable |
|-----|-------------------------------|-------------------------|------------------------------------|-------------------------|
| **1** | Fijar límites de módulos y rutas REST; composición objetivo `cmd/jappi`. | Revisar esquema existente y diseñar migración de `jobs`, sesiones e idempotencia. | Crear una sola app Next.js en `web/`, TypeScript y carpetas `features/auth`, `features/projects`, `features/deployments`, `shared`. | ADR, contrato y estructura revisados; una app web compila en CI. |
| **2** | Crear servidor Go único con `/healthz`, `/readyz`, middleware de request ID, errores y autorización por cuenta. | Implementar migraciones y repositorios PostgreSQL para cuentas, proyectos, servicios, despliegues y trabajos; transacciones. | Crear cliente HTTP tipado desde `docs/api.md`, manejo uniforme de error, cookie y CSRF; ejemplos locales sin secretos. | API arranca con PostgreSQL; esquema migra desde cero; cliente frontend compila. |
| **3** | Integrar acceso con GitHub y sesiones seguras; `/me` e instalaciones autorizadas. | Mover webhook verificado al proceso único, persistir delivery ID y aceptar solo repos/ramas vinculados. | Preparar navegación técnica y estados de carga/error/vacío para el flujo, sin fijar diseño visual. | Login y webhook probados; entregas duplicadas no crean trabajo duplicado. |
| **4** | Implementar `POST /repositories/analyze` reutilizando el detector y validando Dockerfile/puerto. | Implementar descarga de código del commit exacto con token de instalación y límites de tamaño/tiempo. | Definir Hetzner como entorno objetivo: región, red privada, firewall, acceso SSH restringido, K3s y variables por entorno en infraestructura reproducible. | Análisis y fuente de un commit de prueba; plan de infraestructura revisable. |
| **5** | Implementar `POST /projects`: autorización, dominio reservado, proyecto y despliegue inicial. | Crear cola PostgreSQL con lease, reintentos acotados, idempotencia y recuperación tras reinicio. | Levantar entorno aislado en Hetzner con K3s, PostgreSQL, registry y TLS; documentar instalación sin credenciales en Git. | Importación devuelve `201` y un trabajo durable; entorno accesible de forma segura. |
| **6** | Integrar módulo de build que crea un Job aislado y publica imagen OCI por digest. | Registrar estados `queued → building`, errores seguros y límites de concurrencia/recursos del build. | Aplicar aislamiento del Job: namespace, ServiceAccount mínima, cuotas, NetworkPolicy, runtime sandbox (gVisor o equivalente), límites de CPU/memoria y limpieza. | Dockerfile de ejemplo produce imagen; ningún build corre dentro de la API. |
| **7** | Reutilizar adaptador Kubernetes para desplegar la imagen y esperar readiness. | Persistir `deploying → healthy/failed`, impedir que un commit viejo reemplace uno nuevo. | Publicar manifiestos del monolito Go y Next.js, ingress y dominio HTTPS; comprobar rollback operativo de manifiestos. | Desde importación se obtiene URL HTTPS o fallo explicable. |
| **8** | Exponer `GET /projects` y `GET /projects/{id}` con filtrado por cuenta. | Exponer historial y detalle de despliegues, paginación y pruebas de contrato HTTP. | Configurar backups automáticos de PostgreSQL y ejecutar una restauración de prueba; registrar tiempos y responsable. | El frontend puede consultar estado sin acceder a infraestructura interna. |
| **9** | Conectar en la UI mínima login, selección de repo, análisis, importación y estado/URL. | Completar redeploy por push, idempotencia y prueba de extremo a extremo con repositorio de ejemplo. | Ejecutar smoke test en Hetzner: DNS, TLS, webhook, build, despliegue, límites y reinicio; dejar evidencia. | Demo de importación y push posterior funcionando en staging. |
| **10** | Corregir integración y estados de UI; demostrar el flujo con el equipo. | Cerrar fallos, CI y documentación de API/operación; medir duración y errores de build/deploy. | Entregar runbook: alta, despliegue, recuperación, backup, restauración, costes y alertas mínimas. | Demo reproducible, PRs revisados y checklist de aceptación cumplido. |

### Dependencias y forma de trabajo

- Días 1–2 bloquean los DTO y la base de datos; el frontend consume únicamente `docs/api.md`. Todo cambio de contrato se actualiza allí y se comunica en el PR antes de integrarlo.
- Días 3–5 bloquean importación y cola; el build no empieza hasta que el commit y el trabajo estén persistidos. Días 6–7 bloquean la demo de los días 9–10.
- Cada cambio entra en un PR pequeño desde rama `feat/...`, `fix/...` o `docs/...`; requiere revisión de otro integrante. `main` debe permanecer desplegable. César coordina el contrato y la demo; José verifica persistencia y seguridad de eventos; Diego es dueño del entorno y su recuperación.
- Reunión breve diaria: progreso contra el entregable del día, bloqueo y siguiente integración. Si una dependencia se retrasa, se reduce una mejora no esencial; no se omiten firma de webhook, aislamiento de build, TLS ni persistencia.

### Aceptación del Sprint 3

- [ ] Un usuario autorizado importa un repo de prueba con Dockerfile y ve `queued → building → deploying → healthy` y su URL HTTPS.
- [ ] Un push a la rama configurada crea exactamente un despliegue por commit; webhook duplicado o commit viejo no pisa producción.
- [ ] Una compilación fallida termina en `failed` con motivo seguro para la UI; el proceso se recupera tras reiniciar la API sin perder trabajos.
- [ ] El build de código externo está aislado; las credenciales de GitHub y PostgreSQL no aparecen en navegador, logs, imágenes ni repositorio.
- [ ] Backend, frontend y migraciones compilan; pruebas unitarias, de contrato y del recorrido crítico pasan en CI; staging en Hetzner tiene TLS y restauración de backup probada.
- [ ] `docs/api.md`, instrucciones de operación y diagrama C4 reflejan lo que realmente quedó implementado; lo pendiente vuelve al backlog con responsable.
