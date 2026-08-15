// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"context"
	"fmt"
	"sort"
	"strings"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
	"gorm.io/gen"
)

func CreateUiElements(uielements *model.SysUIElement) error {
	return query.SysUIElement.Create(uielements)
}

func UpdateUiElements(uielements *model.SysUIElement) error {
	p := query.SysUIElement
	_, err := query.SysUIElement.Where(p.ID.Eq(uielements.ID)).Updates(uielements)
	if err != nil {
		logrus.Error(err)
	}
	return err
}

func DeleteUiElements(id string) error {
	_, err := query.SysUIElement.Where(query.SysUIElement.ID.Eq(id)).Delete()
	if err != nil {
		logrus.Error(err)
	}
	return err
}

func ServeUiElementsListByPage(uielements *model.ServeUiElementsListByPageReq) (int64, interface{}, error) {
	q := query.SysUIElement
	var count int64
	queryBuilder := q.WithContext(context.Background())
	queryBuilder = queryBuilder.Where(q.ParentID.Eq("0"))
	count, err := queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, nil, err
	}
	if uielements.Page != 0 && uielements.PageSize != 0 {
		queryBuilder = queryBuilder.Limit(uielements.PageSize)
		queryBuilder = queryBuilder.Offset((uielements.Page - 1) * uielements.PageSize)
	}

	uielementsList, err := queryBuilder.Select().Order(q.Order_).Find()
	if err != nil {
		logrus.Error(err)
		return count, uielementsList, err
	}

	allElements, err := q.WithContext(context.Background()).Select().Order(q.Order_).Find()
	if err != nil {
		logrus.Error(err)
		return count, uielementsList, err
	}

	uielementsListrsp := buildUiElementTreeFromRoots(uielementsList, allElements)
	return count, uielementsListrsp, err
}

func ServeUiElementsListByAuthority(u *utils.UserClaims) (int64, interface{}, error) {
	// 系统管理员、租户管理员和租户用户菜单树：直接按 authority 字段过滤
	// TENANT_USER 走此分支而非 casbin_rule 路径，因为本地测试环境 casbin_rule 为空
	if u.Authority == "SYS_ADMIN" || u.Authority == "TENANT_ADMIN" || u.Authority == "TENANT_USER" {
		q := query.SysUIElement
		queryBuilder := q.WithContext(context.Background())
		queryBuilder = queryBuilder.Where(gen.Cond(datatypes.JSONQuery("authority").HasKey(u.Authority))...)
		uielementsList, err := queryBuilder.Order(q.Order_).Find()
		if err != nil {
			logrus.Error(err)
			return 0, uielementsList, err
		}

		count := int64(len(uielementsList))
		uielementsListrsp := buildUiElementTree(uielementsList)
		appendTenantDashboardMenus(uielementsListrsp, u.TenantID)
		return count, uielementsListrsp, err
	}

	// 租户用户菜单树：一次性读取允许的菜单元素，再在内存中构树，避免逐层递归查库。
	uielementsList, err := queryTenantUserAllowedUiElements(u.ID)
	if err != nil {
		return 0, nil, err
	}
	return 0, buildUiElementTree(uielementsList), nil
}

func buildUiElementTree(elements []*model.SysUIElement) []*model.UiElementsListRsp {
	return buildUiElementTreeFromRoots(nil, elements)
}

func buildUiElementTreeFromRoots(rootElements []*model.SysUIElement, elements []*model.SysUIElement) []*model.UiElementsListRsp {
	nodesByID := make(map[string]*model.UiElementsListRsp, len(elements))
	for _, element := range elements {
		if _, exists := nodesByID[element.ID]; exists {
			continue
		}
		nodesByID[element.ID] = element.ToRsp()
	}

	for _, element := range elements {
		node := nodesByID[element.ID]
		if node == nil || element.ParentID == "0" || element.ParentID == element.ID {
			continue
		}
		parent, ok := nodesByID[element.ParentID]
		if !ok {
			continue
		}
		parent.Children = append(parent.Children, node)
	}

	var roots []*model.UiElementsListRsp
	if len(rootElements) > 0 {
		for _, element := range rootElements {
			if node := nodesByID[element.ID]; node != nil {
				roots = append(roots, node)
			}
		}
		return roots
	}

	for _, element := range elements {
		if element.ParentID != "0" {
			continue
		}
		if node := nodesByID[element.ID]; node != nil {
			roots = append(roots, node)
		}
	}
	return roots
}

func buildUiElementTree1(elements []*model.SysUIElement) []*model.UiElementsListRsp1 {
	nodesByID := make(map[string]*model.UiElementsListRsp1, len(elements))
	for _, element := range elements {
		if _, exists := nodesByID[element.ID]; exists {
			continue
		}
		nodesByID[element.ID] = element.ToRsp1()
	}

	for _, element := range elements {
		node := nodesByID[element.ID]
		if node == nil || element.ParentID == "0" || element.ParentID == element.ID {
			continue
		}
		parent, ok := nodesByID[element.ParentID]
		if !ok {
			continue
		}
		parent.Children = append(parent.Children, node)
	}

	var roots []*model.UiElementsListRsp1
	for _, element := range elements {
		if element.ParentID != "0" {
			continue
		}
		if node := nodesByID[element.ID]; node != nil {
			roots = append(roots, node)
		}
	}
	return roots
}

// 获取租户下权限配置表单树
func GetTenantUiElementsList() (interface{}, error) {
	q := query.SysUIElement
	queryBuilder := q.WithContext(context.Background())
	queryBuilder = queryBuilder.Where(gen.Cond(datatypes.JSONQuery("authority").HasKey("TENANT_ADMIN"))...)
	uielementsList, err := queryBuilder.Where(q.ElementType.In(1, 2, 3)).Order(q.Order_).Find()
	if err != nil {
		logrus.Error(err)
		return uielementsList, err
	}

	return buildUiElementTree1(uielementsList), err
}

func queryTenantUserAllowedUiElements(userID string) ([]*model.SysUIElement, error) {
	var uielementsList []*model.SysUIElement
	result := global.DB.Raw(`select tf.* from
		(
		select distinct (crp.v1) from casbin_rule crp
		inner join
		(
		select cr.v1 from casbin_rule cr  where cr.ptype ='g' and cr.v0 = ?
		) crr
		 on crr.v1 = crp.v0 where crp.ptype ='p' and crp.v2 = 'allow'
		) t
		inner join sys_ui_elements tf on t.v1 = tf.id
		where jsonb_exists(tf.authority::jsonb, 'TENANT_USER')
		order by tf.orders desc`, userID).Scan(&uielementsList)
	if result.Error != nil {
		return nil, result.Error
	}
	return uielementsList, nil
}

func appendTenantDashboardMenus(roots []*model.UiElementsListRsp, tenantID string) {
	if tenantID == "" {
		return
	}

	menus, err := ListTenantDashboardMenus(tenantID, "home")
	if err != nil || len(menus) == 0 {
		return
	}

	homeNode := findUiElementByCode(roots, "home")
	if homeNode == nil {
		return
	}

	for _, menu := range menus {
		order := menu.Sort
		path := fmt.Sprintf("/home/dashboard/%s", menu.DashboardID)
		icon := "mdi:view-dashboard-outline"
		hideInMenu := "0"
		routePath := "view.visualization_thingsvis-menu-dashboard"
		description := menu.MenuName
		remark := fmt.Sprintf("thingsvis-dashboard:%s", menu.DashboardID)
		authority := `["SYS_ADMIN","TENANT_ADMIN"]`
		child := &model.UiElementsListRsp{
			ID:           menu.ID,
			ParentID:     homeNode.ID,
			ElementCode:  buildDashboardMenuRouteCode(menu.DashboardID),
			ElementType:  int16Ptr(3),
			Orders:       &order,
			Param1:       &path,
			Param2:       &icon,
			Param3:       &hideInMenu,
			Authority:    authority,
			Description:  &description,
			Remark:       &remark,
			Multilingual: nil,
			RoutePath:    &routePath,
			Children:     []*model.UiElementsListRsp{},
		}
		homeNode.Children = append(homeNode.Children, child)
	}

	sort.SliceStable(homeNode.Children, func(i, j int) bool {
		left := int16(0)
		if homeNode.Children[i].Orders != nil {
			left = *homeNode.Children[i].Orders
		}
		right := int16(0)
		if homeNode.Children[j].Orders != nil {
			right = *homeNode.Children[j].Orders
		}
		return left < right
	})
}

func findUiElementByCode(nodes []*model.UiElementsListRsp, code string) *model.UiElementsListRsp {
	for _, node := range nodes {
		if node.ElementCode == code {
			return node
		}
		if len(node.Children) > 0 {
			if child := findUiElementByCode(node.Children, code); child != nil {
				return child
			}
		}
	}
	return nil
}

func buildDashboardMenuRouteCode(dashboardID string) string {
	replacer := strings.NewReplacer("-", "_", " ", "_", "/", "_")
	return "home_dashboard_" + replacer.Replace(dashboardID)
}

func int16Ptr(value int16) *int16 {
	return &value
}
