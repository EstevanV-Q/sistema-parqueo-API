package audit

// Repository define el contrato de persistencia de auditoría.
type Repository interface {
	Insert(e *Entry) error
	List(f Filter, limit, offset int) ([]*Entry, error)
	Count(f Filter) (int, error)
}
