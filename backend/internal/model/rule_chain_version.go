// 文件用途：规则链版本模型（ROADMAP P1.2 草稿/发布/回滚的持久化面）。
// 核心逻辑：一条链多个版本，(chain_id, version) 唯一；状态受数据库 CHECK 约束。
// 关键注意事项：
//  1. published 在同一条链内同时只能有一个，由部分唯一索引保证（见 93.sql），
//     不靠应用层"先查再写"。
//  2. graph_hash 参与版本身份：内容未变不产生新版本。
//  3. rolled_back_from 记录回滚来源版本，用于双向追溯；非空即表示本版本由回滚产生。
//  4. 所有查询必须带 tenant_id——跨租户数据对本层而言是"不存在"，不是"没权限"。
package model

import "time"

// RuleChainVersion 规则链版本行。
type RuleChainVersion struct {
	ID             string     `gorm:"column:id;primaryKey" json:"id"`
	TenantID       string     `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ChainID        string     `gorm:"column:chain_id;not null" json:"chain_id"`
	Version        int        `gorm:"column:version;not null" json:"version"`
	Status         string     `gorm:"column:status;not null" json:"status"`
	GraphHash      string     `gorm:"column:graph_hash;not null" json:"graph_hash"`
	Graph          []byte     `gorm:"column:graph;type:jsonb" json:"graph"`
	RolledBackFrom *int       `gorm:"column:rolled_back_from" json:"rolled_back_from"`
	CreatedAt      *time.Time `gorm:"column:created_at" json:"created_at"`
	PublishedAt    *time.Time `gorm:"column:published_at" json:"published_at"`
}

// TableName 版本表名。
func (*RuleChainVersion) TableName() string {
	return "rule_chain_versions"
}
