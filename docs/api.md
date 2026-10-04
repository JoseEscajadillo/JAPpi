# Contrato HTTP para el primer golden path

**Estado:** contrato objetivo del Sprint 3; salvo `GET /healthz` y el webhook de GitHub en los procesos actuales, estas rutas todavía no están implementadas. El frontend puede construir su cliente contra este documento y sus ejemplos sin asumir que la API ya existe.

## Alcance y convenciones

- Una sola API pública Go, bajo `/api/v1`, delante de los módulos de identidad, proyectos, repositorios y despliegues. Next.js y API se publican en el mismo origen HTTPS; el ingress envía `/api/*` al backend. El navegador no accede a PostgreSQL, Kubernetes, al registry ni a la API de GitHub con credenciales de servidor.
- JSON en UTF-8; campos `snake_case`; identificadores opacos de texto; fechas RFC 3339 en UTC. Se ignoran campos desconocidos en respuestas; la API rechaza campos desconocidos en peticiones de escritura con `400`.
- Sesión mediante cookie `HttpOnly`, `Secure`, `SameSite=Lax`. Los cambios de estado exigen `Origin` del dominio de JAPpi y token CSRF enviado en `X-CSRF-Token`; la API nunca acepta tokens de GitHub desde el navegador.
- Los recursos siempre se filtran por la cuenta autenticada. Un ID ajeno responde `404`, sin revelar su existencia.
- Las listas devuelven `{ "items": [...], "next_cursor": null }`; `limit` por defecto 20, máximo 100; `cursor` es opaco. Orden: más reciente primero, con ID como desempate.
- Errores: `{ "error": { "code": "validation_error", "message": "Descripción legible", "details": {} }, "request_id": "..." }`. Códigos estables para el cliente; `message` es para mostrar al usuario. Toda respuesta incluye `X-Request-ID`.
- `400` petición inválida; `401` sesión ausente; `403` sin permiso o CSRF inválido; `404` recurso ausente o ajeno; `409` conflicto; `422` repositorio no desplegable; `429` límite de peticiones; `502` dependencia externa temporalmente fallida. La UI ofrece reintento solo para `429`/`502` y errores de red.
- Las operaciones lentas responden de inmediato y se siguen consultando por ID. El cliente consulta cada 3 s mientras el despliegue esté activo; al llegar a `healthy`, `failed` o `superseded` deja de consultar. `Retry-After` prevalece si el servidor lo envía.

## Autenticación y GitHub App

| Método y ruta | Uso | Respuesta |
|---------------|-----|-----------|
| `GET /api/v1/auth/github/start` | Inicia el acceso con GitHub; el servidor genera y valida `state`. | `302` a GitHub |
| `GET /api/v1/auth/github/callback` | Completa el acceso, crea la sesión y vincula la cuenta GitHub. | `302` a `/app`; cookie de sesión |
| `GET /api/v1/me` | Identidad y permisos de la sesión. | `200` `{ "id": "acc_...", "login": "cesar", "display_name": "Cesar", "csrf_token": "..." }` |
| `POST /api/v1/auth/logout` | Revoca la sesión y borra la cookie. | `204` |
| `GET /api/v1/github/installations` | Instalaciones de la GitHub App a las que el usuario tiene acceso, con repositorios autorizados. | `200` `{ "installation_url": "https://github.com/apps/.../installations/new", "items": [{ "id": 123, "account_login": "equipo", "repositories": [{ "id": 456, "full_name": "equipo/demo", "default_branch": "main" }] }] }` |

El enlace para instalar la GitHub App se entrega como `installation_url` en la respuesta de instalaciones. Solo se aceptan `installation_id` y `repository_id` que el servidor compruebe contra GitHub y contra la cuenta actual. El token de instalación se guarda y usa **solo en el backend**. `GET /me` y todas las rutas de lectura de proyectos requieren sesión; `POST` de usuario requiere además CSRF. `GET /auth/*` y `/healthz` son públicos.

## Importación, proyectos y despliegues

| Método y ruta | Petición | Respuesta y comportamiento |
|---------------|----------|---------------------------|
| `POST /api/v1/repositories/analyze` | `{ "installation_id": 123, "repository_id": 456, "branch": "main" }` | `200` con `repository`, `commit_sha` y `services` detectados. En Sprint 3 se acepta un único servicio HTTP con `Dockerfile`; si no existe, `422 unsupported_stack`. No crea recursos. |
| `POST /api/v1/projects` | `{ "installation_id": 123, "repository_id": 456, "branch": "main", "analysis_sha": "012345...", "slug": "demo", "service": { "path": ".", "port": 3000 } }` y cabecera `Idempotency-Key` obligatoria | `201` con `{ "project": {...}, "initial_deployment": {...} }`. Reserva el dominio, guarda proyecto + despliegue `queued` + trabajo pendiente en una transacción. Repetir la misma clave devuelve el mismo resultado; otra carga con la misma clave da `409`. |
| `GET /api/v1/projects` | `?limit=20&cursor=...` | `200` lista de proyectos de la cuenta. |
| `GET /api/v1/projects/{project_id}` | — | `200` detalle del proyecto, servicio y último despliegue. |
| `GET /api/v1/projects/{project_id}/deployments` | `?limit=20&cursor=...` | `200` historial, más reciente primero. |
| `GET /api/v1/projects/{project_id}/deployments/{deployment_id}` | — | `200` estado, commit, URL, timestamps y fallo seguro para mostrar. |
| `POST /webhooks/github` | Webhook firmado de la GitHub App, fuera de `/api/v1` | `202` cuando el push se registra durablemente; `401` firma inválida. El `X-GitHub-Delivery` deduplica entregas. No usa la cookie del usuario. |
| `GET /healthz` y `GET /readyz` | Sondeos internos | `200`; `readyz` comprueba conexión a PostgreSQL. No exponen datos de usuarios. |

`POST /api/v1/projects` exige el SHA completo que devolvió el análisis. Antes de encolar el build, el backend comprueba que la rama aún apunta a ese SHA; si cambió, devuelve `409 stale_analysis` y la UI repite el análisis. El despliegue almacena el SHA exacto y la imagen OCI por digest. Un push posterior crea otro despliegue para el mismo proyecto; los eventos repetidos no crean duplicados. Un push de una rama distinta se ignora. `Idempotency-Key` es un UUID generado por intento y se conserva 24 horas junto a la respuesta; repetir la misma clave y carga devuelve el mismo `201`.

Ejemplo de respuesta de análisis:

```json
{
  "repository": { "id": 456, "full_name": "equipo/demo", "branch": "main" },
  "commit_sha": "0123456789abcdef0123456789abcdef01234567",
  "services": [{ "name": "web", "path": ".", "port": 3000, "build": "dockerfile" }]
}
```

### Formas de respuesta para la UI

```json
{
  "project": {
    "id": "prj_01...",
    "slug": "demo",
    "repository": "equipo/demo",
    "branch": "main",
    "service": { "id": "svc_01...", "name": "web", "path": ".", "port": 3000 },
    "url": "https://web-demo-...jappi.app",
    "created_at": "2026-10-03T12:00:00Z"
  },
  "initial_deployment": {
    "id": "dep_...",
    "service_id": "svc_01...",
    "commit_sha": "0123456789abcdef0123456789abcdef01234567",
    "status": "queued",
    "image": null,
    "failure_reason": null,
    "created_at": "2026-10-03T12:00:00Z",
    "updated_at": "2026-10-03T12:00:00Z"
  }
}
```

`GET /projects/{id}` devuelve el objeto `project` anterior y `latest_deployment`; `GET /projects` devuelve los mismos objetos `project` dentro de `items`. La lista de despliegues usa la misma forma que `initial_deployment`; el detalle devuelve ese objeto directamente. `status` es uno de `queued`, `building`, `deploying`, `healthy`, `failed` o `superseded`. La URL reservada aparece desde la creación del proyecto; la UI solo muestra «Abrir aplicación» cuando el último despliegue está `healthy`. `failure_reason` es un código estable (`source_unavailable`, `build_failed`, `deploy_failed`, `healthcheck_failed`) y nunca contiene logs crudos ni secretos. La UI lo traduce a un mensaje y ofrece reintento solo mediante un nuevo push en Sprint 3. `image` es `null` hasta terminar el build.

### Flujo mínimo que integra el frontend

1. `GET /api/v1/me`; si devuelve `401`, navegar a `/api/v1/auth/github/start`.
2. `GET /api/v1/github/installations`; mostrar repositorios autorizados o el enlace de instalación.
3. Elegir repositorio y rama; `POST /api/v1/repositories/analyze`; mostrar el servicio y sus datos detectados, con edición de `slug` y `port`.
4. `POST /api/v1/projects` con una clave de idempotencia generada por el cliente para ese intento; navegar al proyecto recibido.
5. Consultar el despliegue hasta `healthy`, `failed` o `superseded`; mostrar URL o fallo y ofrecer volver al proyecto.

## Fuera del Sprint 3

Variables secretas del usuario, bases de datos administradas, facturación, logs en vivo, rollback, múltiples servicios por proyecto y edición avanzada del plan detectado quedan fuera de este contrato inicial. Cada ampliación se añade a `/api/v1` con ejemplos y pruebas de contrato antes de que el frontend la integre. El frontend no debe inventar campos ni depender de estructuras internas de Go.
