// 文件用途：维护 plugin\admin\utils.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：Indexer 提供 O(1) 按 ID 查找的有序列表；GetPage/GetOffsetN 解析分页参数（auth 插件复用）。
// 安全职责：page_size 上限 maxPageSize，offset 计算饱和不回绕，Iterate 访问量严格受 n 约束。

package admin

import (
	"container/list"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrNotFound represents a not found error.
var ErrNotFound = status.Error(codes.NotFound, "not found")

// Indexer provides a index for a ordered list that supports queries in O(1).
// All methods are not concurrency-safe.
type Indexer struct {
	index map[string]*list.Element
	rows  *list.List
}

// NewIndexer is the constructor of Indexer.
func NewIndexer() *Indexer {
	return &Indexer{
		index: make(map[string]*list.Element),
		rows:  list.New(),
	}
}

// Set sets the value for the id.
func (i *Indexer) Set(id string, value interface{}) {
	if e, ok := i.index[id]; ok {
		e.Value = value
	} else {
		elem := i.rows.PushBack(value)
		i.index[id] = elem
	}
}

// Remove removes and returns the value for the given id.
// Return nil if not found.
func (i *Indexer) Remove(id string) *list.Element {
	elem := i.index[id]
	if elem != nil {
		i.rows.Remove(elem)
	}
	delete(i.index, id)
	return elem
}

// GetByID returns the value for the given id.
// Return nil if not found.
// Notice: Any access to the return *list.Element also require the mutex,
// because the Set method can modify the Value for *list.Element when updating the Value for the same id.
// If the caller needs the Value in *list.Element, it must get the Value before the next Set is called.
func (i *Indexer) GetByID(id string) *list.Element {
	return i.index[id]
}

// Iterate iterates at most n elements in the list begin from offset.
// Notice: Any access to the  *list.Element in fn also require the mutex,
// because the Set method can modify the Value for *list.Element when updating the Value for the same id.
// If the caller needs the Value in *list.Element, it must get the Value before the next Set is called.
// 先跳过 offset 个元素再至多访问 n 个，不计算 offset+n，因此任何输入都不会回绕。
func (i *Indexer) Iterate(fn func(elem *list.Element), offset, n uint) {
	if n == 0 || offset >= uint(i.rows.Len()) {
		return
	}
	e := i.rows.Front()
	for skipped := uint(0); skipped < offset && e != nil; skipped++ {
		e = e.Next()
	}
	for visited := uint(0); visited < n && e != nil; visited++ {
		fn(e)
		e = e.Next()
	}
}

// Len returns the length of list.
func (i *Indexer) Len() int {
	return i.rows.Len()
}

const (
	// defaultPageSize 是未指定 page_size 时的默认分页大小。
	defaultPageSize = 20
	// maxPageSize 是单页上限（与 Filter 的 limit <= 1000 规则一致）；
	// 超过时静默钳制，保持 REST/gRPC 契约不新增错误码。避免
	// ?page_size=4294967295 在持锁状态下遍历并复制整张列表。
	maxPageSize = 1000
)

// GetPage gets page and pageSize from request params.
// pageSize 被钳制到 [1, maxPageSize]，page 为 0 时取 1。
func GetPage(reqPage, reqPageSize uint32) (page, pageSize uint) {
	page = 1
	pageSize = defaultPageSize
	if reqPage != 0 {
		page = uint(reqPage)
	}
	if reqPageSize != 0 {
		pageSize = min(uint(reqPageSize), maxPageSize)
	}
	return
}

// GetOffsetN 把页码换算为偏移量与条数。page 为 0 视为第 1 页（原实现会回绕成巨大偏移）；
// (page-1)*pageSize 溢出时饱和为最大值（结果为空页），不回绕到列表开头。
func GetOffsetN(page, pageSize uint) (offset, n uint) {
	n = pageSize
	if page <= 1 || pageSize == 0 {
		return 0, n
	}
	pages := page - 1
	if pages > ^uint(0)/pageSize {
		return ^uint(0), n
	}
	return pages * pageSize, n
}

// ErrInvalidArgument is a wrapper function for easier invalid argument error handling.
func ErrInvalidArgument(name string, msg string) error {
	errString := "invalid " + name
	if msg != "" {
		errString = errString + ":" + msg
	}
	return status.Error(codes.InvalidArgument, errString)
}
