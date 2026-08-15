package dal

import (
	"fmt"
	"strings"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"

	"gorm.io/gen"
	"gorm.io/gen/field"

	"github.com/sirupsen/logrus"
)

// GetDeviceListByPage returns one filtered page of active tenant devices.
func GetDeviceListByPage(req *model.GetDeviceListByPageReq, tenantID string) (int64, []model.GetDeviceListByPageRsp, error) {
	return newDeviceListPageReadModel(req, tenantID).execute()
}

// CountDeviceListByFilter counts devices matching the same filters used by the
// device list endpoint without scanning display rows.
func CountDeviceListByFilter(req *model.GetDeviceListByPageReq, tenantID string) (int64, error) {
	plan, err := newDeviceListPageReadModel(req, tenantID).plan()
	if err != nil {
		return 0, logDeviceListPageError(err)
	}
	if plan.empty {
		return 0, nil
	}
	return plan.count()
}

// CountDeviceListFilteredIDs counts which provided ids are still inside the
// same filtered active-device scope.
func CountDeviceListFilteredIDs(req *model.GetDeviceListByPageReq, tenantID string, ids []string) (int64, error) {
	plan, err := newDeviceListPageReadModel(req, tenantID).plan()
	if err != nil {
		return 0, logDeviceListPageError(err)
	}
	if plan.empty {
		return 0, nil
	}
	return plan.countIDs(ids)
}

// ListDeviceIDsByFilter returns only ids for a filtered active-device scope.
func ListDeviceIDsByFilter(req *model.GetDeviceListByPageReq, tenantID string, limit int) ([]string, error) {
	plan, err := newDeviceListPageReadModel(req, tenantID).plan()
	if err != nil {
		return nil, logDeviceListPageError(err)
	}
	if plan.empty {
		return []string{}, nil
	}
	return plan.scanIDs(limit)
}

// GetDeviceListRowsByFilterAndIDs loads display rows for a small selected id
// set while preserving the same filters as the device list endpoint.
func GetDeviceListRowsByFilterAndIDs(req *model.GetDeviceListByPageReq, tenantID string, ids []string) ([]model.GetDeviceListByPageRsp, error) {
	plan, err := newDeviceListPageReadModel(req, tenantID).plan()
	if err != nil {
		return nil, logDeviceListPageError(err)
	}
	if plan.empty {
		return emptyDeviceListPage(), nil
	}
	return plan.scanByIDs(ids)
}

func deviceListGroupFilterIDs(groupID string) ([]string, error) {
	groupIDs, err := GetGroupChildrenIds(groupID)
	if err != nil {
		return nil, err
	}

	ids, err := GetDeviceIdsByGroupIds(groupIDs)
	if err != nil {
		return nil, err
	}

	// Preserve the existing ID set shape used by the list filter.
	return append(ids, groupID), nil
}

func deviceListServiceIdentifierCondition(value string) gen.Condition {
	q := query.Device
	c := query.DeviceConfig

	if value == "mqtt" {
		return query.Device.Where(c.ProtocolType.Eq(value)).Or(q.DeviceConfigID.IsNull())
	}
	return c.ProtocolType.Eq(value)
}

func deviceListDeviceTypeCondition(value string) gen.Condition {
	q := query.Device
	c := query.DeviceConfig

	if value == "1" {
		return query.Device.Where(q.DeviceConfigID.IsNull()).Or(c.DeviceType.Eq(value))
	}
	return c.DeviceType.Eq(value)
}

func emptyDeviceListPage() []model.GetDeviceListByPageRsp {
	return []model.GetDeviceListByPageRsp{}
}

func logDeviceListPageError(err error) error {
	logrus.Error(err)
	return err
}

func joinDeviceListLatestTelemetry(builder query.IDeviceDo, deviceIDs []string) query.IDeviceDo {
	q := query.Device
	t := query.TelemetryCurrentData
	t2 := query.TelemetryCurrentData.As("t2")
	latestTelemetry := t.Select(t.T.Max().As("ts"), t.DeviceID).Group(t.DeviceID)
	if len(deviceIDs) > 0 {
		latestTelemetry = latestTelemetry.Where(t.DeviceID.In(deviceIDs...))
	}
	return builder.LeftJoin(latestTelemetry.As("t2"), t2.DeviceID.EqCol(q.ID))
}

func buildDeviceListPageScanQuery(builder query.IDeviceDo) query.IDeviceDo {
	q := query.Device
	return builder.Select(deviceListPageSelectFields()...).
		Order(q.CreatedAt.Desc())
}

func deviceListPageSelectFields() []field.Expr {
	q := query.Device
	c := query.DeviceConfig

	return []field.Expr{
		q.ID,
		q.DeviceNumber,
		q.Name,
		q.DeviceConfigID,
		q.ActivateFlag,
		q.ActivateAt,
		q.BatchNumber,
		q.Description,
		q.Location,
		q.CurrentVersion,
		q.CreatedAt,
		q.IsOnline,
		q.AccessWay,
		q.TenantID,
		q.OwnerUserID,
		q.ParentID,
		q.SubDeviceAddr,
		c.ProtocolType,
		c.DeviceType,
		c.Name.As("DeviceConfigName"),
		c.ImageURL,
		q.LastOfflineTime,
		q.AdditionalInfo,
	}
}

func hydrateDeviceListLatestTelemetry(deviceList *[]model.GetDeviceListByPageRsp) error {
	if deviceList == nil || len(*deviceList) == 0 {
		return nil
	}

	deviceIDs := make([]string, 0, len(*deviceList))
	for _, device := range *deviceList {
		deviceIDs = append(deviceIDs, device.ID)
	}

	t := query.TelemetryCurrentData
	rows, err := t.Select(t.DeviceID, t.T).
		Where(t.DeviceID.In(deviceIDs...)).
		Order(t.T.Desc()).
		Find()
	if err != nil {
		return err
	}

	latestByDevice := make(map[string]*model.TelemetryCurrentData, len(deviceIDs))
	for _, row := range rows {
		if row == nil || row.DeviceID == "" || row.T.IsZero() || latestByDevice[row.DeviceID] != nil {
			continue
		}
		latestByDevice[row.DeviceID] = row
	}
	for index := range *deviceList {
		if latest := latestByDevice[(*deviceList)[index].ID]; latest != nil {
			ts := latest.T
			(*deviceList)[index].Ts = &ts
		}
	}
	return nil
}
func hasDeviceListValue(s *string) bool {
	return s != nil && strings.TrimSpace(*s) != ""
}

func deviceListLike(f field.String, v string) field.Expr {
	return f.Like(fmt.Sprintf("%%%s%%", v))
}

// rdiDeviceSharedStatusCondition keeps the SQL-side list filter aligned with
// service.rdiDeviceSharedStatus: shared means the RDI recipient array exists
// and contains at least one non-empty user_id.
func rdiDeviceSharedStatusCondition(additionalInfo field.String, shared bool) gen.Condition {
	normalized := additionalInfo.
		Replace(" ", "").
		Replace("\n", "").
		Replace("\r", "").
		Replace("\t", "")
	nonEmptyUserIDs := normalized.Replace(`"user_id":""`, "")
	const (
		recipientsKeyPattern = `%"rdi_share_recipients":[%`
		nonEmptyUserPattern  = `%"user_id":"%`
	)

	if shared {
		return query.Device.Where(
			normalized.Like(recipientsKeyPattern),
			nonEmptyUserIDs.Like(nonEmptyUserPattern),
		)
	}

	return query.Device.Where(additionalInfo.IsNull()).
		Or(normalized.NotLike(recipientsKeyPattern)).
		Or(nonEmptyUserIDs.NotLike(nonEmptyUserPattern))
}
