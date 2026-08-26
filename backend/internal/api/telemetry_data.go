// TelemetryDataApi owns telemetry HTTP and WebSocket entry points.
// Keep shared WebSocket authentication helpers in telemetry_ws_auth.go.
package api

import (
	"context"
	"strconv"
	"strings"

	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type TelemetryDataApi struct{}

func setTelemetryQueryData[T any](c *gin.Context, req *T, query func(*T, *utils.UserClaims) (any, error)) {
	if !BindAndValidate(c, req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := query(req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// HandleCurrentData returns current telemetry for one device.
func (*TelemetryDataApi) HandleCurrentData(c *gin.Context) {
	deviceId := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	date, err := service.GroupApp.TelemetryData.GetCurrentTelemetrData(deviceId, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", date)
}

// HandleCurrentDataKeys returns available current telemetry keys.
func (*TelemetryDataApi) HandleCurrentDataKeys(c *gin.Context) {
	var req model.GetTelemetryCurrentDataKeysReq
	setTelemetryQueryData(c, &req, func(req *model.GetTelemetryCurrentDataKeysReq, userClaims *utils.UserClaims) (any, error) {
		return service.GroupApp.TelemetryData.GetCurrentTelemetrDataKeys(req, userClaims)
	})
}

// ServeHistoryData returns historical telemetry for the requested query.
func (*TelemetryDataApi) ServeHistoryData(c *gin.Context) {
	var req model.GetTelemetryHistoryDataReq
	setTelemetryQueryData(c, &req, func(req *model.GetTelemetryHistoryDataReq, userClaims *utils.UserClaims) (any, error) {
		return service.GroupApp.TelemetryData.GetTelemetrHistoryData(req, userClaims)
	})
}

// DeleteData deletes telemetry data matching the validated request.
func (*TelemetryDataApi) DeleteData(c *gin.Context) {
	var req model.DeleteTelemetryDataReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.TelemetryData.DeleteTelemetrData(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", nil)
}

// ServeCurrentDetailData returns detailed current telemetry for one device.
func (*TelemetryDataApi) ServeCurrentDetailData(c *gin.Context) {
	deviceId := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	date, err := service.GroupApp.TelemetryData.GetCurrentTelemetrDetailData(deviceId, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", date)
}

// ServeHistoryDataByPage returns paged telemetry history.
func (*TelemetryDataApi) ServeHistoryDataByPage(c *gin.Context) {
	serveHistoryDataByPage(c)
}

func serveHistoryDataByPage(c *gin.Context) {
	var req model.GetTelemetryHistoryDataByPageReq
	if !BindAndValidate(c, &req) {
		return
	}

	// Keep the historical time-range guard disabled here; the service layer
	// owns retention and query-window validation for this endpoint.

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.TelemetryData.GetTelemetrHistoryDataByPageV2(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// ServeSetLogsDataListByPage returns paged telemetry set logs.
func (*TelemetryDataApi) ServeSetLogsDataListByPage(c *gin.Context) {
	var req model.GetTelemetrySetLogsListByPageReq
	setTelemetryQueryData(c, &req, func(req *model.GetTelemetrySetLogsListByPageReq, userClaims *utils.UserClaims) (any, error) {
		return service.GroupApp.TelemetryData.GetTelemetrSetLogsDataListByPage(req, userClaims)
	})
}

// ServeDeadLetterList returns paged telemetry write failures for operator handling.
func (*TelemetryDataApi) ServeDeadLetterList(c *gin.Context) {
	var req model.GetTelemetryDeadLetterListReq
	setTelemetryQueryData(c, &req, func(req *model.GetTelemetryDeadLetterListReq, userClaims *utils.UserClaims) (any, error) {
		return service.GroupApp.TelemetryData.GetTelemetryDeadLetterList(req, userClaims)
	})
}

// UpdateDeadLetterStatus marks or replays a telemetry dead-letter row.
func (*TelemetryDataApi) UpdateDeadLetterStatus(c *gin.Context) {
	var req model.UpdateTelemetryDeadLetterStatusReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.TelemetryData.UpdateTelemetryDeadLetterStatus(c.Param("id"), &req, userClaims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// DrainDeadLetters replays a bounded batch of ready telemetry dead-letter rows.
func (*TelemetryDataApi) DrainDeadLetters(c *gin.Context) {
	var req model.DrainTelemetryDeadLetterReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.TelemetryData.DrainTelemetryDeadLetters(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// ServeEchoData echoes simulation telemetry context for diagnostics.
// @Summary Get simulation echo data
// @Description Echoes simulation telemetry context (client IP, resolved topic, credentials) for a device to help operators verify a simulated MQTT client wire-up before publishing.
// @Tags TelemetrySimulation
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param device_id query string true "Device ID"
// @Success 200 {string} string "Ready-to-run mosquitto_pub command for the device"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router /api/v1/telemetry/datas/simulation [get]
func (*TelemetryDataApi) ServeEchoData(c *gin.Context) {
	var req model.ServeEchoDataReq
	if !BindAndValidate(c, &req) {
		return
	}

	// Use Gin's resolved client IP so simulation diagnostics match request context.
	clientIP := resolveSimulationClientIP(c)
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	date, err := service.GroupApp.TelemetryData.ServeEchoData(&req, clientIP, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", date)
}

func resolveSimulationClientIP(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.ClientIP())
}

// SimulationTelemetryData publishes simulated telemetry for a validated command.
// @Summary Publish simulated telemetry
// @Description Parses a mosquitto_pub command and publishes it as if it originated from a real device, so operators can rehearse dashboards or rules without touching physical hardware. Restricted to SYS_ADMIN.
// @Tags TelemetrySimulation
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body model.SimulationTelemetryDataReq true "Simulation telemetry payload"
// @Success 200 {object} response.Response "Publish acknowledged; data is null"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router /api/v1/telemetry/datas/simulation [post]
func (*TelemetryDataApi) SimulationTelemetryData(c *gin.Context) {
	var req model.SimulationTelemetryDataReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	_, err := service.GroupApp.TelemetryData.TelemetryPub(req.Command, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// GetSimulationInit returns initialization data for telemetry simulation.
// @Summary Get telemetry simulation init data
// @Description Returns MQTT connection hints, default topics and default payload templates so operators can bootstrap the telemetry simulation panel.
// @Tags TelemetrySimulation
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param device_id query string true "Target device ID"
// @Success 200 {object} model.SimulationInitResp "Simulation init payload"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router /api/v1/telemetry/datas/simulation/init [get]
func (*TelemetryDataApi) GetSimulationInit(c *gin.Context) {
	var req model.SimulationInitReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.TelemetryData.GetSimulationInit(req.DeviceId, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// SimulationSend sends a simulation command for a permitted device.
// @Summary Send simulated telemetry via broker
// @Description Publishes a simulation payload to the MQTT broker for a permitted device using the supplied server/port/topic overrides.
// @Tags TelemetrySimulation
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body model.SimulationSendReq true "Simulation send payload"
// @Success 200 {object} object "Simulation send accepted"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router /api/v1/telemetry/datas/simulation/send [post]
func (*TelemetryDataApi) SimulationSend(c *gin.Context) {
	var req model.SimulationSendReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.TelemetryData.SimulationSend(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// ServeCurrentDataByWS upgrades the request and streams current telemetry.
func (*TelemetryDataApi) ServeCurrentDataByWS(c *gin.Context) {
	conn, closeConn, ok := upgradeTelemetryWSSession(c, "telemetry websocket connected")
	if !ok {
		return
	}
	defer closeConn()

	handshake, ok := readTelemetryCurrentWSHandshake(conn)
	if !ok {
		return
	}

	wsClient, cleanup, ok := subscribeTelemetryWSStream(conn, handshake, "telemetry websocket")
	if !ok {
		return
	}
	defer cleanup()
	// 与 device status 端点对称：显式声明 handler 对写队列的关闭权；
	// CloseSend 自身幂等，与 cleanup→UnsubscribeDevice 的隐式关闭互不冲突。
	defer closeTelemetryWSClientSend(wsClient)

	if !queueInitialCurrentTelemetryData(wsClient, handshake.deviceID, handshake.claims) {
		return
	}

	runTelemetryWSHeartbeatLoop(wsClient, handshake.deviceID, true)
}

// ServeDeviceStatusByWS streams device online/offline status changes.
func (*TelemetryDataApi) ServeDeviceStatusByWS(c *gin.Context) {
	conn, closeConn, ok := upgradeTelemetryWSSession(c, "device status websocket connected")
	if !ok {
		return
	}
	defer closeConn()

	msgType, deviceID, claims, ok := readDeviceStatusWSHandshake(conn)
	if !ok {
		return
	}

	isOnline, ok := queryDeviceStatusInitialState(conn, msgType, deviceID, claims)
	if !ok {
		return
	}

	localClient := newTelemetryWSClient(conn, msgType, deviceID, claims, nil)
	startTelemetryWSWriter(localClient, false)
	defer closeTelemetryWSClientSend(localClient)

	queueInitialDeviceStatus(localClient, deviceID, isOnline)

	// Background 是有意为之：该 ctx 的生命周期=WS 连接本身（连接级转发器），
	// 不能绑 HTTP 请求上下文——升级后请求对象语义不可靠。
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	closeForwarder := startDeviceStatusRedisForwarder(ctx, deviceID, localClient)
	defer closeForwarder()

	runDeviceStatusWSReadLoop(conn, localClient, cancel)
}

// ServeCurrentDataByKey streams selected telemetry keys over WebSocket.
func (*TelemetryDataApi) ServeCurrentDataByKey(c *gin.Context) {
	conn, closeConn, ok := upgradeTelemetryWSSession(c, "telemetry key websocket connected")
	if !ok {
		return
	}
	defer closeConn()

	handshake, ok := readTelemetryKeyCurrentWSHandshake(conn)
	if !ok {
		return
	}

	data, ok := fetchInitialTelemetryKeyData(conn, handshake)
	if !ok {
		return
	}

	wsClient, cleanup, ok := subscribeTelemetryWSStream(conn, handshake, "telemetry key websocket")
	if !ok {
		return
	}
	defer cleanup()
	// 与 device status 端点对称：显式声明 handler 对写队列的关闭权；
	// CloseSend 自身幂等，与 cleanup→UnsubscribeDevice 的隐式关闭互不冲突。
	defer closeTelemetryWSClientSend(wsClient)

	if !queueInitialTelemetryData(wsClient, handshake.deviceID, data, "telemetry key") {
		return
	}

	runTelemetryWSHeartbeatLoop(wsClient, handshake.deviceID, true)
}

// ServeStatisticData returns aggregate telemetry statistics.
func (*TelemetryDataApi) ServeStatisticData(c *gin.Context) {
	var req model.GetTelemetryStatisticReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	date, err := service.GroupApp.TelemetryData.GetTelemetrServeStatisticData(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", date)
}

// TelemetryPutMessage accepts a manual telemetry message.
// @Summary Publish manual telemetry
// @Description Publishes a manual telemetry payload to the message bus on behalf of the caller for the specified device.
// @Tags Telemetry
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param request body model.PutMessage true "Manual telemetry payload"
// @Success 200 {object} object "Telemetry accepted"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router /api/v1/telemetry/datas/pub [post]
func (*TelemetryDataApi) TelemetryPutMessage(c *gin.Context) {
	var req model.PutMessage
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.TelemetryData.TelemetryPutMessage(c, userClaims.ID, &req, strconv.Itoa(constant.Manual))
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// ServeMsgCountByTenant returns the telemetry message count for a tenant.
// @Summary Get tenant telemetry message count
// @Description Returns the total telemetry message count for the caller's tenant. Requires a non-empty tenant scope on the caller claims.
// @Tags Telemetry
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} object "Message count payload"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router /api/v1/telemetry/datas/msg/count [get]
func (*TelemetryDataApi) ServeMsgCountByTenant(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	if userClaims.TenantID == "" {
		c.Error(errcode.New(201001))
		return
	}
	cnt, err := service.GroupApp.TelemetryData.ServeMsgCountByTenantId(userClaims.TenantID)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", map[string]interface{}{"msg": cnt})
}

// ServeStatisticDataByDeviceId returns batch telemetry statistics for devices.
// @Summary Batch telemetry statistics
// @Description Returns aggregated telemetry statistics for multiple devices and keys in a single call.
// @Tags Telemetry
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param device_ids query []string true "Device IDs; count must match keys"
// @Param keys query []string true "Telemetry keys; count must match device_ids"
// @Param time_type query string true "Aggregate window" Enums(hour, day, week, month, year)
// @Param aggregate_method query string true "Aggregate function" Enums(avg, sum, max, min, count, diff)
// @Param limit query int false "Maximum points per series (1-1000)"
// @Success 200 {array} model.ChartValue "Aggregated chart points"
// @Failure 400 {object} errcode.Error "Parameter validation error"
// @Router /api/v1/telemetry/datas/statistic/batch [get]
func (*TelemetryDataApi) ServeStatisticDataByDeviceId(c *gin.Context) {
	var req model.GetTelemetryStatisticByDeviceIdReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.TelemetryData.GetTelemetryStatisticDataByDeviceIds(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}
