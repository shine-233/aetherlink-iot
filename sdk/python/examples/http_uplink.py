"""HTTP 通道示例:OpenAPI Key(X-API-Key)调用平台数据服务(标准库,无额外依赖)。"""
from aetherlink import AetherLinkClient, ClientConfig

config = ClientConfig(host="platform.example.com", api_base="https://platform.example.com/api/v1", api_key="your-openapi-key")
client = AetherLinkClient(config)

result = client.post_json("/device/data/report", {"device_number": "dev-001", "data": {"temperature": 26.5}})
print(result)
