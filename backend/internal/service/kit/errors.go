// Package kit holds the small generic building blocks shared by tenant-scoped
// CRUD services: the DB-error envelope, not-found mapping, the claims gate,
// a tenant repository wrapper and the {"total","list"} page shape.
//
// It imports only pkg/errcode, pkg/utils, gorm and stdlib so that package
// service can depend on it without an import cycle. Every helper reproduces
// an existing wire contract byte for byte; the error data keys in particular
// are pinned by clients and must not be unified.
package kit

import "aetherlink-iot/backend/pkg/errcode"

// Data keys used inside the CodeDBError envelope. Both exist in the REST
// contract: dbError() emits "sql_error", most CRUD services emit "error".
const (
	KeySQL   = "sql_error"
	KeyError = "error"
)

// DBErr builds errcode.CodeDBError with data {key: err.Error()}.
func DBErr(key string, err error) error {
	return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
		key: err.Error(),
	})
}

// DBErrOp builds errcode.CodeDBError with data {"operation": op, "error": err.Error()}.
func DBErrOp(op string, err error) error {
	return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
		"operation": op,
		KeyError:    err.Error(),
	})
}

// OnDBErr returns an error mapper that wraps with DBErr(key, ·); handy as the
// onErr argument of List.
func OnDBErr(key string) func(error) error {
	return func(err error) error { return DBErr(key, err) }
}
