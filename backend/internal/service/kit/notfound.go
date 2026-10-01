package kit

import (
	"errors"

	"aetherlink-iot/backend/pkg/errcode"

	"gorm.io/gorm"
)

// IsRecordNotFound reports gorm.ErrRecordNotFound anywhere in the chain.
func IsRecordNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }

// IsRecordNotFoundExact matches only the unwrapped sentinel. It exists for
// services whose historical contract used `err == gorm.ErrRecordNotFound`
// (a wrapped not-found falls through to the DB-error branch there).
func IsRecordNotFoundExact(err error) bool { return err == gorm.ErrRecordNotFound }

// NotFound decides how a load error becomes the resource's not-found error.
type NotFound struct {
	// Code defaults to errcode.CodeNotFound.
	Code int
	// Msg is the custom message; empty means a bare errcode.New(Code).
	Msg string
	// Match selects which load errors are "not found". nil masks every error,
	// which is the contract of the tenant-in-DAL resources (data converter,
	// integration, widget bundle, ...): any failure reads as not found so the
	// caller cannot probe other tenants' ids.
	Match func(error) bool
	// OnOther maps errors Match rejected; default DBErr(KeySQL, err).
	OnOther func(error) error
}

// Err returns the not-found error itself.
func (n NotFound) Err() error {
	code := n.Code
	if code == 0 {
		code = errcode.CodeNotFound
	}
	if n.Msg == "" {
		return errcode.New(code)
	}
	return errcode.NewWithMessage(code, n.Msg)
}

// Map converts a load error. nil stays nil.
func (n NotFound) Map(err error) error {
	if err == nil {
		return nil
	}
	if n.Match == nil || n.Match(err) {
		return n.Err()
	}
	if n.OnOther != nil {
		return n.OnOther(err)
	}
	return DBErr(KeySQL, err)
}
