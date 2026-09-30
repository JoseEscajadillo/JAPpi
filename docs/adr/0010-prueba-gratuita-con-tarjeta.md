# 0010. Prueba gratuita con tarjeta registrada

- **Estado:** Propuesto (pendiente de decisión del equipo)
- **Fecha:** 2026-09-29
- **Decisores:** equipo JAPpi

## Contexto

El plan es ofrecer un mes gratis y luego suscripciones de $5, $12 y $20 con Stripe. Todas las plataformas que ejecutan código ajeno gratis (Heroku, Render, Railway, Fly) han sufrido abuso masivo:

- minería de criptomonedas;
- proxies y envío de spam;
- phishing alojado en sus dominios.

Un mes de CPU gratis sin fricción es un imán para eso. El abuso quema el presupuesto y, además, puede hacer que el proveedor del VPS suspenda nuestra cuenta entera.

## Decisión propuesta

- La prueba de 30 días **exige registrar una tarjeta** (Stripe Checkout con `trial_period_days: 30` y `payment_method_collection: always`). No se cobra nada hasta el día 31.
- Durante la prueba se aplican los límites del plan de $5.
- Además, desde el primer día:
  - tráfico de salida limitado;
  - puertos SMTP (25/465/587) bloqueados;
  - alerta si un proyecto sostiene la CPU al 100 %.

## Alternativas consideradas

- **Prueba sin tarjeta:** más conversiones iniciales, pero el abuso hace inviable el coste.
- **Prueba sin tarjeta, pero con recursos mínimos y apagado por inactividad:** opción intermedia razonable si el equipo prefiere no pedir tarjeta.

## Consecuencias

- **Positivas:** corta la mayoría del abuso automatizado; Stripe gestiona el paso de la prueba al pago.
- **Negativas:** menos registros; en Latinoamérica muchos estudiantes no tienen tarjeta de crédito internacional.
