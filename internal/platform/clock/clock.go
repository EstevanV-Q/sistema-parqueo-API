package clock

import "time"

// NowFunc es una función que devuelve la hora actual. Permite inyectar tiempo en tests.
type NowFunc func() time.Time

// Real devuelve la hora real del sistema en UTC.
var Real NowFunc = func() time.Time { return time.Now().UTC() }

// Fixed devuelve una NowFunc que siempre retorna la hora dada.
func Fixed(t time.Time) NowFunc {
	return func() time.Time { return t }
}
