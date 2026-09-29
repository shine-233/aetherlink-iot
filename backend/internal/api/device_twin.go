package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type DeviceTwinApi struct{}

// HandleDeviceTwin returns a read-only desired-vs-reported twin aggregation for
// one device. It is intentionally thin and delegates permission checks plus data
// shaping to the service layer.
func (*DeviceTwinApi) HandleDeviceTwin(c *gin.Context) {
	HandlePath(c, "id", func(deviceID string, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DeviceTwin.GetDeviceTwin(deviceID, userClaims)
	})
}

// HandleDeviceTwinDriftIndex returns a read-only fleet-level drift index that
// enumerates a bounded set of tenant devices, reuses the single-device twin
// classification, and aggregates it into a severity-ranked queryable index.
func (*DeviceTwinApi) HandleDeviceTwinDriftIndex(c *gin.Context) {
	Handle(c, func(req *model.DeviceTwinDriftIndexReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DeviceTwin.GetDeviceTwinDriftIndex(req, userClaims)
	})
}

func (*DeviceTwinApi) UpsertDeviceTwinDesired(c *gin.Context) {
	HandlePathBody(c, "id", func(deviceID string, req *model.UpsertDeviceTwinDesiredReq, userClaims *utils.UserClaims) (interface{}, error) {
		return service.GroupApp.DeviceTwin.UpsertDesired(deviceID, req, userClaims)
	})
}
