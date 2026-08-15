// 文件用途: 覆盖 REQ-05b 设备生命周期状态筛选(lifecycle_status)的 opt-in 语义边界。
// 核心逻辑: 断言 deviceListLifecycleCondition 对 nil/缺省/activated/inactive/all/非法值的分支行为。
// 关键注意事项: activated/inactive/all 分支会触及 query.Device 全局单例,需先经 setupDeviceDALTestDB 初始化。
//
//	不测 "带空格" 输入: DTO 校验是 oneof=activated|inactive|all(精确匹配、不 trim),带空格值在绑定层即被拒,
//	不可能到达 DAL; 断言其成功会把 validator 与 DAL(自带 TrimSpace)的层间不一致当成正确行为焊死。
//
// 重构建议: 若后续新增"已安装/传输完成"状态字段,同步扩展本用例的期望分支。
package dal

import (
	"testing"

	"aetherlink-iot/backend/internal/model"
)

func strPtrLifecycle(s string) *string {
	return &s
}

func TestDeviceListLifecycleConditionOptInSemantics(t *testing.T) {
	// activated/inactive 分支会触及 query.Device 全局单例,需先初始化;否则 nil 解引用 panic。
	setupDeviceDALTestDB(t)

	cases := []struct {
		name        string
		req         *model.GetDeviceListByPageReq
		wantApplies bool
		wantCondNil bool
	}{
		{
			name:        "nil request falls back to historical active-only",
			req:         nil,
			wantApplies: false,
			wantCondNil: true,
		},
		{
			name:        "absent lifecycle status falls back to historical active-only",
			req:         &model.GetDeviceListByPageReq{},
			wantApplies: false,
			wantCondNil: true,
		},
		{
			name:        "activated applies an explicit condition",
			req:         &model.GetDeviceListByPageReq{LifecycleStatus: strPtrLifecycle("activated")},
			wantApplies: true,
			wantCondNil: false,
		},
		{
			name:        "inactive applies an explicit condition",
			req:         &model.GetDeviceListByPageReq{LifecycleStatus: strPtrLifecycle("inactive")},
			wantApplies: true,
			wantCondNil: false,
		},
		{
			name:        "transmitted applies an explicit telemetry-backed condition",
			req:         &model.GetDeviceListByPageReq{LifecycleStatus: strPtrLifecycle("transmitted")},
			wantApplies: true,
			wantCondNil: true,
		},
		{
			name:        "all applies with no condition (shows every lifecycle)",
			req:         &model.GetDeviceListByPageReq{LifecycleStatus: strPtrLifecycle("all")},
			wantApplies: true,
			wantCondNil: true,
		},
		{
			// 防御性回退: 若非法值绕过 validator 到达 DAL,应安全退回历史 active-only,
			// 而非放行全部。这是有意的安全契约,值得锁定。
			name:        "unknown value falls back to historical active-only",
			req:         &model.GetDeviceListByPageReq{LifecycleStatus: strPtrLifecycle("bogus")},
			wantApplies: false,
			wantCondNil: true,
		},
		{
			name:        "whitespace value fails closed instead of being trimmed",
			req:         &model.GetDeviceListByPageReq{LifecycleStatus: strPtrLifecycle(" transmitted ")},
			wantApplies: false,
			wantCondNil: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cond, applies := deviceListLifecycleCondition(tc.req)
			if applies != tc.wantApplies {
				t.Fatalf("applies = %v, want %v", applies, tc.wantApplies)
			}
			if (cond == nil) != tc.wantCondNil {
				t.Fatalf("cond == nil is %v, want %v", cond == nil, tc.wantCondNil)
			}
		})
	}
}
