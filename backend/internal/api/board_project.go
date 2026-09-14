// 文件用途：看板项目分组的 HTTP 入口（native-board-provider 项目增删改）。
// 边界说明：租户边界与归属判定在 service 层；本层只做绑定、claims 提取与错误出口。
package api

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type BoardProjectApi struct{}

// Create 创建看板项目。
// POST /api/v1/board/projects
// @Summary  创建看板项目
// @Tags     BoardProjects
// @Router   /api/v1/board/projects [post]
func (*BoardProjectApi) Create(c *gin.Context) {
	var req model.CreateBoardProjectReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	project, err := service.GroupApp.BoardProject.CreateProject(req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", project)
}

// List 列出看板项目（?board_id= 反查包含该看板的项目）。
// GET /api/v1/board/projects
// @Summary  看板项目列表（board_id 反查）
// @Tags     BoardProjects
// @Router   /api/v1/board/projects [get]
func (*BoardProjectApi) List(c *gin.Context) {
	req := model.BoardProjectListReq{BoardID: c.Query("board_id")}
	claims := c.MustGet("claims").(*utils.UserClaims)
	projects, err := service.GroupApp.BoardProject.ListProjects(req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", projects)
}

// Get 项目详情。
// GET /api/v1/board/projects/:id
// @Summary  看板项目详情
// @Tags     BoardProjects
// @Router   /api/v1/board/projects/{id} [get]
func (*BoardProjectApi) Get(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	project, err := service.GroupApp.BoardProject.GetProject(c.Param("id"), claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", project)
}

// Update 更新项目。
// PUT /api/v1/board/projects/:id
// @Summary  更新看板项目
// @Tags     BoardProjects
// @Router   /api/v1/board/projects/{id} [put]
func (*BoardProjectApi) Update(c *gin.Context) {
	var req model.UpdateBoardProjectReq
	if !BindAndValidate(c, &req) {
		return
	}
	claims := c.MustGet("claims").(*utils.UserClaims)
	project, err := service.GroupApp.BoardProject.UpdateProject(c.Param("id"), req, claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", project)
}

// Delete 删除项目（只解除分组，不动看板）。
// DELETE /api/v1/board/projects/:id
// @Summary  删除看板项目
// @Tags     BoardProjects
// @Router   /api/v1/board/projects/{id} [delete]
func (*BoardProjectApi) Delete(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.BoardProject.DeleteProject(c.Param("id"), claims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"deleted": true})
}

// AddBoard 把看板加入项目。
// PUT /api/v1/board/projects/:id/boards/:board_id
// @Summary  看板加入项目
// @Tags     BoardProjects
// @Router   /api/v1/board/projects/{id}/boards/{board_id} [put]
func (*BoardProjectApi) AddBoard(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.BoardProject.AddBoard(c.Param("id"), c.Param("board_id"), claims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"added": true})
}

// RemoveBoard 把看板移出项目。
// DELETE /api/v1/board/projects/:id/boards/:board_id
// @Summary  看板移出项目
// @Tags     BoardProjects
// @Router   /api/v1/board/projects/{id}/boards/{board_id} [delete]
func (*BoardProjectApi) RemoveBoard(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	if err := service.GroupApp.BoardProject.RemoveBoard(c.Param("id"), c.Param("board_id"), claims); err != nil {
		c.Error(err)
		return
	}
	c.Set("data", gin.H{"removed": true})
}

// MembershipOf 反查看板所属项目。
// GET /api/v1/board/projects/member-of/:board_id
// @Summary  反查看板所属项目
// @Tags     BoardProjects
// @Router   /api/v1/board/projects/member-of/{board_id} [get]
func (*BoardProjectApi) MembershipOf(c *gin.Context) {
	claims := c.MustGet("claims").(*utils.UserClaims)
	project, err := service.GroupApp.BoardProject.MembershipOf(c.Param("board_id"), claims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", project)
}
