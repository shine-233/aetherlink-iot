// Entity relation routes (ROADMAP P1.1).
package apps

import (
	"aetherlink-iot/backend/internal/api"

	"github.com/gin-gonic/gin"
)

type EntityRelation struct{}

func (*EntityRelation) InitEntityRelation(Router *gin.RouterGroup) {
	url := Router.Group("entity-relations")
	{
		url.POST("", api.Controllers.EntityRelationApi.CreateEntityRelation)
		url.GET("", api.Controllers.EntityRelationApi.ListEntityRelations)
		url.DELETE(":id", api.Controllers.EntityRelationApi.DeleteEntityRelation)
		// 按实体删除必须走显式 body：级联只能由调用方主动声明，不能靠路径默认。
		url.DELETE("by-entity", api.Controllers.EntityRelationApi.DeleteEntityRelationsForEntity)
	}
}
