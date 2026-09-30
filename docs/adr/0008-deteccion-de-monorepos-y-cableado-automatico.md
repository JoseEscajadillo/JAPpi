# 0008. Detección de monorepos y cableado automático de variables

- **Estado:** Aceptado
- **Fecha:** 2026-09-29
- **Decisores:** equipo JAPpi

## Contexto

Este es el diferencial de JAPpi. Hoy, desplegar un frontend y un backend que se hablan exige configurar a mano:

- la URL del backend en el frontend;
- el origen del frontend en el backend (CORS);
- las cadenas de conexión de Postgres y Redis.

Además hay una trampa: `NEXT_PUBLIC_*` y `VITE_*` se incrustan **en el build**, así que la URL del backend tiene que existir antes de construir el frontend.

Railway (con sus reference variables) y Render (con sus Blueprints) resuelven parte, **pero requieren que el usuario escriba la configuración**. Nosotros queremos que funcione sin configuración.

## Decisión

1. **Detección** (`services/control-plane/internal/domain/stack`):
   - **Workspaces:** leemos las declaraciones de pnpm, npm, yarn y `go.work`, y marcamos Turborepo o Nx si están presentes; si no hay ninguna, revisamos las carpetas convencionales (`apps/*`, `frontend`, `backend`...).
   - **Servicios:** cada carpeta pasa por una lista de `Detector`s (Node, Go, Python, Dockerfile). El resultado dice el rol, el framework, el puerto y el prefijo público del framework.
   - **Add-ons:** se deducen de las dependencias (drivers de Postgres, clientes de Redis, `schema.prisma`).
   - **Librerías:** lo que no es desplegable se ignora.
2. **Pistas de nombres:** leemos `.env.example` y el código (`process.env.X`, `import.meta.env.X`, `os.Getenv("X")`, `os.environ[...]`, `$env/static/...`) para averiguar **cómo llama ya el código a cada variable**.
3. **Cableado** (`domain/wiring`):
   - **Qué se inyecta:** `PORT` en todos; la URL pública de cada backend en los frontends; los orígenes de los frontends en los backends para CORS; y referencias a los secretos de Postgres y Redis.
   - **Qué nombre se usa:**
     1. primero, uno conocido que el código ya use;
     2. después, una pista que encaje con el patrón;
     3. por último, un nombre por defecto (`NEXT_PUBLIC_API_URL`, `DATABASE_URL`...).
   - **Reglas:** las variables del usuario nunca se sobrescriben (se informa del conflicto), y **los secretos nunca entran en el entorno de build**, solo las URLs públicas.
4. **Redespliegue selectivo:** en un monorepo, un push que solo toca `apps/web` reconstruye solo `web`. Si cambió un archivo compartido (lockfile, paquete común), o la lista de archivos está incompleta, se redespliega todo (regla conservadora).

## Alternativas consideradas

- **Archivo de configuración obligatorio (`jappi.toml`):** predecible, pero es justo la fricción que queremos eliminar. Se podrá añadir más adelante como **override opcional**.
- **Buildpacks o Railpack para detectar:** detectan cómo construir **un** servicio, pero no las relaciones entre servicios. Los usaremos en el builder, no para esto.

## Consecuencias

- **Positivas:** importar un monorepo típico (Next + Express + Prisma) no requiere configurar nada; el dashboard puede explicar cada variable (`reason`).
- **Negativas:**
  - Es heurística: habrá repos que se detecten mal, así que el dashboard debe permitir corregir el plan antes del primer deploy.
  - Hay que mantener la lista de frameworks.
  - Si hay varios backends, el frontend recibe la URL de todos (no sabemos cuál usa).
- **Pendiente:** seguir las dependencias `workspace:*` para detectar, por ejemplo, un Prisma que vive en `packages/db`.
