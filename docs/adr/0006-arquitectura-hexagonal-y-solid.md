# 0006. Arquitectura hexagonal y SOLID en cada servicio

- **Estado:** Aceptado
- **Fecha:** 2026-09-29
- **Decisores:** equipo JAPpi

## Contexto

Cada servicio mezcla lógica de negocio valiosa (qué desplegar, cómo cablear variables, qué plan permite qué) con infraestructura cambiante (NATS, Postgres, Kubernetes, la API de GitHub, Stripe). Si la lógica depende de la infraestructura, no se puede probar sin levantar medio clúster, y cambiar de proveedor obliga a reescribirla.

## Decisión

Cada servicio (`services/<nombre>`) es un **hexágono** (puertos y adaptadores):

```
services/<nombre>/
├── cmd/<nombre>/main.go     raíz de composición: crea adaptadores e inyecta
└── internal/
    ├── domain/              reglas de negocio puras
    ├── app/                 casos de uso + puertos (interfaces) que necesitan
    └── adapters/
        ├── in/              lo que llama al hexágono: HTTP, consumidores de eventos, CLI
        └── out/             lo que el hexágono llama: Postgres, NATS, Kubernetes, GitHub
```

Reglas de dependencia (siempre hacia dentro), **comprobadas por `pkg/archtest` en `go test`**:

| Capa | Puede importar |
|------|----------------|
| `domain` | Solo la librería estándar y el propio dominio |
| `app` | Librería estándar, `domain` y `pkg/contracts` |
| `adapters`, `cmd` | Cualquier cosa |

Aplicamos **SOLID** con el sabor de Go (ver [CONTRIBUTING](../../CONTRIBUTING.md#solid-en-go-con-ejemplos-de-este-repo)): interfaces pequeñas declaradas por quien las usa, estrategias en vez de `switch` crecientes, suites de contrato para las implementaciones de un puerto, y dependencias inyectadas en `main.go`.

## Alternativas consideradas

- **Capas clásicas (handler → service → repository) sin reglas:** más rápido al principio, pero la infraestructura se filtra en la lógica con el tiempo.
- **Clean Architecture completa (entities, use cases, interface adapters, frameworks):** más capas de las que un equipo de tres puede mantener con disciplina en Go.

## Consecuencias

- **Positivas:** el dominio se prueba en milisegundos con datos en memoria; cambiar Postgres, NATS o el proveedor de Git toca solo un adaptador; la regla se hace cumplir sola.
- **Negativas:** más archivos y algo de código repetido al traducir entre capas; hay que resistir la tentación de crear interfaces "por si acaso" (en Go se crean cuando existe un consumidor).
