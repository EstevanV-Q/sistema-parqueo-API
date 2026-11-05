package money

import (
	"database/sql/driver"
	"errors"
	"fmt"
)

// Amount representa una cantidad de dinero en centavos (int64).
// Nunca se usa float para dinero.
type Amount int64

// Zero es una cantidad nula.
const Zero Amount = 0

// FromFloat convierte una cantidad en unidades (ej. 12.50) a centavos.
// El redondeo es hacia el centavo más cercano.
func FromFloat(f float64) Amount {
	return Amount(f*100 + 0.5)
}

// Float devuelve la cantidad en unidades (ej. 1250 -> 12.50).
func (a Amount) Float() float64 {
	return float64(a) / 100
}

// String devuelve una representación legible.
func (a Amount) String() string {
	neg := ""
	v := a
	if v < 0 {
		neg = "-"
		v = -v
	}
	return fmt.Sprintf("%s%d.%02d", neg, v/100, v%100)
}

// Add suma dos cantidades.
func (a Amount) Add(b Amount) Amount {
	return a + b
}

// Sub resta dos cantidades.
func (a Amount) Sub(b Amount) Amount {
	return a - b
}

// Value implementa driver.Valuer para guardar en MySQL como BIGINT.
func (a Amount) Value() (driver.Value, error) {
	return int64(a), nil
}

// Scan implementa sql.Scanner para leer desde MySQL.
func (a *Amount) Scan(src any) error {
	switch v := src.(type) {
	case int64:
		*a = Amount(v)
	case int:
		*a = Amount(v)
	case []byte:
		var n int64
		if _, err := fmt.Sscanf(string(v), "%d", &n); err != nil {
			return err
		}
		*a = Amount(n)
	default:
		return errors.New("money: no se puede escanear tipo %T")
	}
	return nil
}
