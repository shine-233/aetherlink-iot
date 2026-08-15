// 文件用途: 覆盖 REQ-05b 设备列表 lifecycle_status 筛选在 HTTP 校验层(validator tag)的契约。
// 核心逻辑: 对真实 model.GetDeviceListByPageReq 跑 ValidateStruct,断言 oneof 白名单与 omitempty 行为。
// 关键注意事项: 必须绑真实 DTO(而非临时匿名 struct),否则证明不了生产结构体的 tag 真实生效。
// 关键注意事项: transmitted 是已成功上报过一次的派生状态,必须与前端取值逐字对齐。
package api

import (
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/model"
)

func TestDeviceListLifecycleStatusValidationContract(t *testing.T) {
	// 合法分页基线,避免其它必填字段干扰 lifecycle_status 的断言。
	base := func(lifecycle *string) model.GetDeviceListByPageReq {
		req := model.GetDeviceListByPageReq{LifecycleStatus: lifecycle}
		req.Page = 1
		req.PageSize = 10
		return req
	}

	strPtr := func(s string) *string { return &s }

	// omitempty: 不传 lifecycle_status 必须通过(保持既有 active-only 默认行为)。
	if err := ValidateStruct(base(nil)); err != nil {
		t.Fatalf("absent lifecycle_status should pass validation, got: %v", err)
	}

	validValues := []string{"activated", "inactive", "transmitted", "all"}
	for _, v := range validValues {
		v := v
		t.Run("valid_"+v, func(t *testing.T) {
			if err := ValidateStruct(base(strPtr(v))); err != nil {
				t.Fatalf("lifecycle_status=%q should pass oneof, got: %v", v, err)
			}
		})
	}

	// 关键契约(实测确认):omitempty 只豁免 nil 指针,不豁免指向空串的指针。
	// 即 gin 把 ?lifecycle_status= 绑成 &"",会进入 oneof 检查并被拒(400)。
	// 因此前端筛选项默认值绝不能发空串(已改为 value:'activated');
	// 只有"完全不传该参数"(nil)才走既有 active-only 默认。空串在此如实断言为"被拒"。
	invalidValues := []string{"installed", "transfer_complete", "active", "", "  activated  ", "ACTIVATED"}
	for _, v := range invalidValues {
		v := v
		t.Run("invalid_"+strings.ReplaceAll(v, " ", "_"), func(t *testing.T) {
			// 非白名单值(含空串)必须被 oneof 拒绝;nil 缺省已在上方单独断言通过。
			if err := ValidateStruct(base(strPtr(v))); err == nil {
				t.Fatalf("lifecycle_status=%q should be rejected by oneof, but passed", v)
			}
		})
	}
}
