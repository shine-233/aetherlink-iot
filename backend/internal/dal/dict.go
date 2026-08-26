// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"fmt"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"gorm.io/gen/field"
)

func CreateDict(dict *model.SysDict, tx *query.QueryTx) error {
	if tx != nil {
		return tx.SysDict.Create(dict)
	} else {
		return query.SysDict.Create(dict)
	}
}

// tenant-scope: system-table?2026-08-26 ?????
func GetDictById(dictId string) (*model.SysDict, error) {
	dict, err := query.SysDict.Where(query.SysDict.ID.Eq(dictId)).First()
	if err != nil {
		return nil, err
	}
	return dict, err
}

func DeleteDictById(dictId string) error {
	_, err := query.SysDict.Where(query.SysDict.ID.Eq(dictId)).Delete()
	return err
}

// tenant-scope: system-table?2026-08-26 ?????
func GetDictListByCode(dictCode string) ([]*model.SysDict, error) {
	dict, err := query.SysDict.Where(query.SysDict.DictCode.Eq(dictCode)).Find()
	if err != nil {
		return nil, err
	}
	return dict, err
}

// tenant-scope: system-table?2026-08-26 ?????
func GetDictListByPage(dictListReq *model.GetDictLisyByPageReq, claims *utils.UserClaims) (count int64, dictList interface{}, err error) {
	q := query.SysDict

	if claims.Authority != SYS_ADMIN {
		return count, nil, fmt.Errorf("authority exception")
	}

	if dictListReq.DictCode != nil {
		dictList, err = q.Select(q.ALL).
			Where(field.Attrs(map[string]interface{}{"dict_code": dictListReq.DictCode})).
			Order(q.CreatedAt.Desc()).
			Offset((dictListReq.Page - 1) * dictListReq.PageSize).
			Limit(dictListReq.PageSize).
			Find()
	} else {
		dictList, err = q.Select(q.ALL).
			Order(q.CreatedAt.Desc()).
			Offset((dictListReq.Page - 1) * dictListReq.PageSize).
			Limit(dictListReq.PageSize).
			Find()
	}

	if err != nil {
		return count, dictList, err
	}

	if dictListReq.DictCode != nil {
		count, err = q.Where(field.Attrs(map[string]interface{}{"dict_code": dictListReq.DictCode})).Count()

	} else {
		count, err = q.Count()

	}

	return count, dictList, err
}

// 根据字典标识符和多语言标识符获取字典
// tenant-scope: system-table?2026-08-26 ?????
func GetDictLanguageByDictCodeAndLanguageCode(dictCode, languageCode string) ([]map[string]interface{}, error) {
	var data []map[string]interface{}
	// P1 修复（2026-08-24，见 VALIDATION.md）：字典多语言翻译改走 raw global.DB 链，
	// 消除包级单例 SysDict LeftJoin(SysDictLanguage)+Scan 在高并发下跨请求残留 Statement
	// 读到空/旧翻译的风险；协议菜单等 i18n 每请求路径直接受益。
	err := global.DB.Table("sys_dict").
		Select("sys_dict.dict_value, sys_dict_language.translation").
		Joins("LEFT JOIN sys_dict_language ON sys_dict_language.dict_id = sys_dict.id").
		Where("sys_dict.dict_code = ?", dictCode).
		Where("sys_dict_language.language_code = ?", languageCode).
		Scan(&data).Error
	if err != nil {
		return nil, err
	}
	return data, err
}
