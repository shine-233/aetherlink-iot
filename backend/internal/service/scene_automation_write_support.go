package service

import (
	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/errcode"
)

// withSceneAutomationTransaction wraps scene automation definition writes so
// multi-table changes commit atomically.
func withSceneAutomationTransaction(fn func(*query.QueryTx) error) error {
	tx, err := dal.StartTransaction()
	if err != nil {
		return sceneAutomationDBError(err)
	}
	committed := false
	defer func() {
		if !committed {
			dal.Rollback(tx)
		}
	}()

	if err := fn(tx); err != nil {
		return err
	}

	if err := dal.Commit(tx); err != nil {
		return sceneAutomationDBError(err)
	}
	committed = true
	return nil
}

func normalizeSceneAutomationEnabled(enabled string) string {
	if enabled == "Y" {
		return "Y"
	}
	return "N"
}

func sceneAutomationDBError(err error) error {
	return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
		"sql_error": err.Error(),
	})
}
