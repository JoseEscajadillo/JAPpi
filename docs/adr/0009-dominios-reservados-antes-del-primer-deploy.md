# 0009. Dominios reservados antes del primer deploy

- **Estado:** Aceptado
- **Fecha:** 2026-09-29
- **Decisores:** equipo JAPpi

## Contexto

El frontend necesita la URL pública del backend **durante su build** (ADR-0008). Si el dominio se asignara al terminar el primer despliegue del backend, el primer build del frontend no la tendría: habría que desplegar dos veces o forzar un orden.

## Decisión

- Al crear un proyecto, el control-plane **reserva el dominio de cada servicio** y lo guarda con una restricción `UNIQUE`.
- **Formato:** `<servicio>-<proyecto>-<hash6>.<dominio-base>`. El hash sale de `sha256(proyectoID/servicio)`, así que el dominio es estable y difícil de adivinar.
- Las etiquetas DNS se sanean y se recortan a 63 caracteres.
- Todos los servicios usan HTTPS con un **certificado wildcard** del dominio base (cert-manager con DNS-01), así que un dominio nuevo no espera la emisión de un certificado.
- Los dominios personalizados del cliente se añadirán más adelante como alias.

## Alternativas consideradas

- **Asignar el dominio al desplegar:** reintroduce el problema del huevo y la gallina.
- **Nombres aleatorios (tipo Heroku, `bold-river-1234`):** no se sabe qué servicio es cuál al mirar la URL.

## Consecuencias

- **Positivas:** frontend y backend se construyen en paralelo desde el primer push; las URLs son legibles.
- **Negativas:** renombrar un servicio no cambia su dominio (se conserva el reservado para no romper builds anteriores); requiere un proveedor DNS con API para el wildcard.
