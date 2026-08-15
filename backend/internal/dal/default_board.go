package dal

import (
	"time"

	model "aetherlink-iot/backend/internal/model"

	"github.com/go-basic/uuid"
)

// defaultBoardConfig is the tenant-facing Home board seed used by runtime
// board creation. Keep it aligned with backend/sql/7.sql.
//
// The retired classic-home card registry is no longer populated in the
// frontend, so new-tenant default Home boards stay empty and let users land on
// the supported first-device workbench / ThingsVis flow instead of compat mode.
const defaultBoardConfig = `[]`

func NewDefaultBoard(tenantID *string) *model.Board {
	tenant := ""
	if tenantID != nil {
		tenant = *tenantID
	}

	config := defaultBoardConfig
	now := time.Now().UTC()

	return &model.Board{
		ID:        uuid.New(),
		Name:      "Home",
		Config:    &config,
		TenantID:  tenant,
		CreatedAt: now,
		UpdatedAt: now,
		HomeFlag:  "Y",
		Remark:    nil,
	}
}
