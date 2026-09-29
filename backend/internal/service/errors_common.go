package service

import "aetherlink-iot/backend/pkg/errcode"

// dbError is the single constructor for the service-layer database error
// envelope: code 101001 with data {"sql_error": <driver message>}. The JSON
// shape is part of the REST contract, so every former inline copy of this
// map now routes through here.
func dbError(err error) error {
	return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
		"sql_error": err.Error(),
	})
}
