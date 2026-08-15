// 文件用途：维护 plugin\admin\client.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package admin

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
)

type clientService struct {
	a *Admin
}

func (c *clientService) mustEmbedUnimplementedClientServiceServer() {
	return
}

// List lists clients information which the session is valid in the broker (both connected and disconnected).
func (c *clientService) List(ctx context.Context, req *ListClientRequest) (*ListClientResponse, error) {
	page, pageSize := GetPage(req.Page, req.PageSize)
	clients, total, err := c.a.store.GetClients(page, pageSize)
	if err != nil {
		return &ListClientResponse{}, err
	}
	return &ListClientResponse{
		Clients:    clients,
		TotalCount: total,
	}, nil
}

// Get returns the client information for given request client id.
func (c *clientService) Get(ctx context.Context, req *GetClientRequest) (*GetClientResponse, error) {
	if req.ClientId == "" {
		return nil, ErrInvalidArgument("client_id", "")
	}
	client := c.a.store.GetClientByID(req.ClientId)
	if client == nil {
		return nil, ErrNotFound
	}
	return &GetClientResponse{
		Client: client,
	}, nil
}

// Delete force disconnect.
func (c *clientService) Delete(ctx context.Context, req *DeleteClientRequest) (*empty.Empty, error) {
	if req.ClientId == "" {
		return nil, ErrInvalidArgument("client_id", "")
	}
	if req.CleanSession {
		c.a.clientService.TerminateSession(req.ClientId)
	} else {
		client := c.a.clientService.GetClient(req.ClientId)
		if client != nil {
			client.Close()
		}
	}
	return &empty.Empty{}, nil
}
