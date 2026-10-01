package kit

// Page is the typed form of the {"total","list"} list response. Field order
// is list-then-total so its JSON is byte-identical to the map form (encoding/json
// sorts map keys).
type Page[T any] struct {
	List  []T   `json:"list"`
	Total int64 `json:"total"`
}

// Map returns the map form services return today. The value types are kept
// (int64 total, []T list) because mobile adapters and tests type-assert them.
func (p Page[T]) Map() map[string]interface{} { return ListMap(p.Total, p.List) }

// AnyListMap is ListMap for the DAL family that returns the list as
// interface{} (gorm-gen Find results passed through untyped). The dynamic
// type of list is preserved, so type assertions on "list" keep working.
func AnyListMap(total int64, list interface{}) map[string]interface{} {
	return map[string]interface{}{"total": total, "list": list}
}

// ListMap builds map[string]interface{}{"total": total, "list": list}.
func ListMap[T any](total int64, list []T) map[string]interface{} {
	return map[string]interface{}{"total": total, "list": list}
}
