package events

import "context"

// Handler procesa un evento. Si devuelve un error, el bus lo reentrega más
// tarde, salvo que el error envuelva ErrPermanent. Por eso todo Handler debe
// ser idempotente: recibir dos veces el mismo evento no puede duplicar trabajo.
type Handler func(ctx context.Context, e Envelope) error
