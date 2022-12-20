//go:build go1.18

package lac

func New[T any](ac *Allocator) *T {
	var r *T
	ac.New(&r)
	return r
}

func NewCopy[T any](ac *Allocator, from *T) *T {
	return ac.NewCopy(from).(*T)
}

func Enum[T any](ac *Allocator, e T) *T {
	return ac.Enum(e).(*T)
}

func NewSlice[T any](ac *Allocator, len, cap int) []T {
	var r []T
	ac.NewSlice(&r, len, cap)
	return r
}

func NewMap[K comparable, V any](ac *Allocator) map[K]V {
	var r map[K]V
	ac.NewMap(&r)
	return r
}
