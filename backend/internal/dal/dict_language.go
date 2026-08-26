// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
)

func CreateDictLanguage(dictLanguage *model.SysDictLanguage, tx *query.QueryTx) error {
	if tx != nil {
		return tx.SysDictLanguage.Create(dictLanguage)
	} else {
		return query.SysDictLanguage.Create(dictLanguage)
	}
}

func DeleteDictLanguageById(id string) error {
	_, err := query.SysDictLanguage.Where(query.SysDictLanguage.ID.Eq(id)).Delete()
	return err
}

// tenant-scope: system-table?2026-08-26 ?????
func GetDictLanguageByDictIdListAndLanguageCode(dictIdList []string, languageCode string) (dictLanList []*model.SysDictLanguage, err error) {
	q := query.SysDictLanguage
	if len(languageCode) != 0 {
		dictLanList, err = q.Select(q.ALL).Where(q.DictID.In(dictIdList...)).Where(q.LanguageCode.Eq(languageCode)).Find()
		if len(dictIdList) == 0 {
			return dictLanList, nil
		}
	} else {
		dictLanList, err = q.Select(q.ALL).Where(q.DictID.In(dictIdList...)).Find()
	}
	return dictLanList, err
}

// tenant-scope: system-table?2026-08-26 ?????
func GetDictLanguageListByDictId(dictId string) ([]*model.SysDictLanguage, error) {
	q := query.SysDictLanguage
	var d []*model.SysDictLanguage
	d, err := q.Select(q.ALL).Where(q.DictID.Eq(dictId)).Order(q.LanguageCode).Find()
	return d, err
}
