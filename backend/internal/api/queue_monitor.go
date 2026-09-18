package api

import (
	"aetherlink-iot/backend/internal/isolatedqueue"

	"github.com/gin-gonic/gin"
)

type QueueMonitorApi struct{}

// GetStats 获取当前平台所有隔离队列（Main、HighPriority、SequentialByOriginator）的状态与度量指标。
func (a *QueueMonitorApi) GetStats(c *gin.Context) {
	mgr := isolatedqueue.GetDefaultManager()
	stats := mgr.GetAllStats()
	c.Set("data", gin.H{
		"queues": stats,
	})
}

// GetConfig 获取平台队列拓扑与策略配置。
func (a *QueueMonitorApi) GetConfig(c *gin.Context) {
	c.Set("data", gin.H{
		"standard_queues": []isolatedqueue.QueueConfig{
			{
				Name:           isolatedqueue.QueueTypeMain,
				Capacity:       10000,
				SubmitStrategy: isolatedqueue.SubmitStrategyBurst,
				DropPolicy:     isolatedqueue.DropPolicyBackpressure,
				Workers:        8,
			},
			{
				Name:           isolatedqueue.QueueTypeHighPriority,
				Capacity:       5000,
				SubmitStrategy: isolatedqueue.SubmitStrategyBurst,
				DropPolicy:     isolatedqueue.DropPolicyBackpressure,
				Workers:        4,
			},
			{
				Name:           isolatedqueue.QueueTypeSequentialByOriginator,
				Capacity:       5000,
				SubmitStrategy: isolatedqueue.SubmitStrategySequentialByOriginator,
				DropPolicy:     isolatedqueue.DropPolicyBackpressure,
				PartitionCount: 16,
			},
		},
	})
}
