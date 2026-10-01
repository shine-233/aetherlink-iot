package dal

import "github.com/sirupsen/logrus"

// pagedFindQuery 抽象 gen 生成的 IXxxDo 接口在"计数 + 分页 + 查询"链路上的共同形状。
type pagedFindQuery[Q any, T any] interface {
	limitOffsetQuery[Q]
	Count() (int64, error)
	Find() ([]*T, error)
}

// countAndFindGenPage 对已施加筛选条件的查询先 Count，再按 page/pageSize 施加有界分页
// （applyListPagination），最后经 finish（如 Select/Order，可为 nil）后 Find。
// 计数失败返回 (count, nil, err)；查询失败返回 (count, data, err)；两类错误均记录日志。
func countAndFindGenPage[Q pagedFindQuery[Q, T], T any](qb Q, page, pageSize int, finish func(Q) Q) (int64, []*T, error) {
	count, err := qb.Count()
	if err != nil {
		logrus.Error(err)
		return count, nil, err
	}
	qb = applyListPagination(qb, page, pageSize)
	if finish != nil {
		qb = finish(qb)
	}
	data, err := qb.Find()
	if err != nil {
		logrus.Error(err)
	}
	return count, data, err
}
