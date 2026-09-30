# 0005. Next.js con Multi-Zones para los microfrontends

- **Estado:** Aceptado
- **Fecha:** 2026-09-29
- **Decisores:** equipo JAPpi

## Contexto

El dashboard tiene áreas bastante independientes: la web pública con precios y documentación, la gestión de proyectos y despliegues (con logs en vivo), y la facturación. Queremos que cada área se pueda desarrollar y desplegar por separado, igual que los servicios del backend, sin que un cambio en la facturación obligue a redesplegar el visor de logs.

## Decisión

Usaremos **Next.js (App Router)** con **Multi-Zones**, el mecanismo oficial de Next.js para microfrontends. Cada zona es una aplicación Next independiente, montada bajo un prefijo de ruta:

| Zona | Rutas | Contenido |
|------|-------|-----------|
| `web/apps/shell` | `/`, `/pricing`, `/docs` | Web pública, login; enruta a las demás zonas |
| `web/apps/console` | `/app/*` | Proyectos, servicios, despliegues, logs, variables |
| `web/apps/billing` | `/billing/*` | Planes, prueba gratuita, portal de Stripe |

- La zona `shell` reescribe (`rewrites`) las rutas hacia las demás zonas.
- La UI común vive en un paquete compartido, `web/packages/ui`.
- Las zonas comparten dominio, así que la sesión (cookie) es común.
- Entre zonas se navega con `<a>`, no con `<Link>`: es una carga completa de página.
- El frontend es un monorepo pnpm + Turborepo. JAPpi se desplegará a sí mismo: su propio dashboard es el primer cliente del detector de monorepos (ADR-0008).

## Alternativas consideradas

- **Una sola app Next.js:** más simple, pero pierde el despliegue independiente que buscamos.
- **Module Federation (Webpack):** permite componer en tiempo de ejecución, pero su soporte en el App Router de Next es inestable y añade mucha complejidad.
- **Web Components o iframes:** aislamiento fuerte, pero mala experiencia (estilos, SEO, rendimiento).

## Consecuencias

- **Positivas:** cada zona se construye, prueba y despliega sola; los fallos quedan acotados; encaja con el reparto por personas del equipo.
- **Negativas:** saltar entre zonas recarga la página completa; las dependencias (React, Next) se repiten por zona y hay que mantenerlas en la misma versión (Turborepo y el lockfile compartido ayudan); más de una app que operar. **Si en la práctica la navegación entre zonas resulta molesta, se reconsidera fusionar `console` y `billing`.**
