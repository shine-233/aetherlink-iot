// Package casbinadapter persists Casbin policies in the existing casbin_rule
// table through the application's GORM connection. It deliberately does not
// open a database, migrate schemas, or add another deployment service.
package casbinadapter

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"strings"

	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	"gorm.io/gorm"
)

const tableName = "casbin_rule"

// Adapter implements Casbin's persistence contracts on the database already
// owned by the backend. The production connection is PostgreSQL; tests may
// inject another GORM dialect with the same table contract.
type Adapter struct {
	db *gorm.DB
}

type rule struct {
	ID    int64   `gorm:"column:id;primaryKey;autoIncrement:true"`
	Ptype *string `gorm:"column:ptype"`
	V0    *string `gorm:"column:v0"`
	V1    *string `gorm:"column:v1"`
	V2    *string `gorm:"column:v2"`
	V3    *string `gorm:"column:v3"`
	V4    *string `gorm:"column:v4"`
	V5    *string `gorm:"column:v5"`
}

func (*rule) TableName() string { return tableName }

var (
	_ persist.Adapter      = (*Adapter)(nil)
	_ persist.BatchAdapter = (*Adapter)(nil)
)

// New binds an adapter to an existing GORM connection.
func New(db *gorm.DB) (*Adapter, error) {
	if db == nil {
		return nil, errors.New("casbin adapter requires a database")
	}
	return &Adapter{db: db}, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func pointer(value string) *string { return &value }

func newRule(ptype string, values []string) (*rule, error) {
	if ptype == "" {
		return nil, errors.New("casbin policy type is empty")
	}
	if len(values) > 6 {
		return nil, fmt.Errorf("casbin policy has %d values, maximum is 6", len(values))
	}
	columns := [6]string{}
	copy(columns[:], values)
	return &rule{
		Ptype: pointer(ptype), V0: pointer(columns[0]), V1: pointer(columns[1]),
		V2: pointer(columns[2]), V3: pointer(columns[3]), V4: pointer(columns[4]),
		V5: pointer(columns[5]),
	}, nil
}

func (r *rule) policyLine() (string, error) {
	values := []string{
		stringValue(r.Ptype), stringValue(r.V0), stringValue(r.V1),
		stringValue(r.V2), stringValue(r.V3), stringValue(r.V4), stringValue(r.V5),
	}
	for len(values) > 1 && values[len(values)-1] == "" {
		values = values[:len(values)-1]
	}
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	if err := writer.Write(values); err != nil {
		return "", err
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buffer.String(), "\n"), nil
}

// LoadPolicy loads every persisted p/g policy into the supplied Casbin model.
func (a *Adapter) LoadPolicy(casbinModel model.Model) error {
	var rules []rule
	if err := a.db.Order("id ASC").Find(&rules).Error; err != nil {
		return fmt.Errorf("load casbin policies: %w", err)
	}
	for index := range rules {
		line, err := rules[index].policyLine()
		if err != nil {
			return fmt.Errorf("encode casbin policy %d: %w", rules[index].ID, err)
		}
		persist.LoadPolicyLine(line, casbinModel)
	}
	return nil
}

// SavePolicy atomically replaces all persisted policies with the model state.
func (a *Adapter) SavePolicy(casbinModel model.Model) error {
	rows := make([]rule, 0)
	for _, section := range []string{"p", "g"} {
		for ptype, assertion := range casbinModel[section] {
			for _, policy := range assertion.Policy {
				row, err := newRule(ptype, policy)
				if err != nil {
					return err
				}
				rows = append(rows, *row)
			}
		}
	}
	return a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&rule{}).Error; err != nil {
			return fmt.Errorf("clear casbin policies: %w", err)
		}
		if len(rows) == 0 {
			return nil
		}
		if err := tx.Create(&rows).Error; err != nil {
			return fmt.Errorf("save casbin policies: %w", err)
		}
		return nil
	})
}

func (a *Adapter) AddPolicy(_ string, ptype string, values []string) error {
	row, err := newRule(ptype, values)
	if err != nil {
		return err
	}
	if err := a.db.Create(row).Error; err != nil {
		return fmt.Errorf("add casbin policy: %w", err)
	}
	return nil
}

func (a *Adapter) AddPolicies(_ string, ptype string, policies [][]string) error {
	if len(policies) == 0 {
		return nil
	}
	rows := make([]rule, 0, len(policies))
	for _, policy := range policies {
		row, err := newRule(ptype, policy)
		if err != nil {
			return err
		}
		rows = append(rows, *row)
	}
	return a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&rows).Error; err != nil {
			return fmt.Errorf("add casbin policies: %w", err)
		}
		return nil
	})
}

func exactPolicyQuery(db *gorm.DB, ptype string, values []string) (*gorm.DB, error) {
	if len(values) > 6 {
		return nil, fmt.Errorf("casbin policy has %d values, maximum is 6", len(values))
	}
	query := db.Where("ptype = ?", ptype)
	for index := 0; index < 6; index++ {
		column := fmt.Sprintf("v%d", index)
		if index < len(values) {
			query = query.Where(column+" = ?", values[index])
		} else {
			query = query.Where("("+column+" = ? OR "+column+" IS NULL)", "")
		}
	}
	return query, nil
}

func (a *Adapter) RemovePolicy(_ string, ptype string, values []string) error {
	query, err := exactPolicyQuery(a.db, ptype, values)
	if err != nil {
		return err
	}
	if err := query.Delete(&rule{}).Error; err != nil {
		return fmt.Errorf("remove casbin policy: %w", err)
	}
	return nil
}

func (a *Adapter) RemovePolicies(_ string, ptype string, policies [][]string) error {
	if len(policies) == 0 {
		return nil
	}
	return a.db.Transaction(func(tx *gorm.DB) error {
		for _, policy := range policies {
			query, err := exactPolicyQuery(tx, ptype, policy)
			if err != nil {
				return err
			}
			if err := query.Delete(&rule{}).Error; err != nil {
				return fmt.Errorf("remove casbin policies: %w", err)
			}
		}
		return nil
	})
}

// RemoveFilteredPolicy treats empty filter values as wildcards, matching the
// Casbin adapter contract.
func (a *Adapter) RemoveFilteredPolicy(_ string, ptype string, fieldIndex int, fieldValues ...string) error {
	if fieldIndex < 0 || fieldIndex > 5 || fieldIndex+len(fieldValues) > 6 {
		return fmt.Errorf("invalid casbin filter range: index=%d values=%d", fieldIndex, len(fieldValues))
	}
	query := a.db.Where("ptype = ?", ptype)
	for offset, value := range fieldValues {
		if value == "" {
			continue
		}
		query = query.Where(fmt.Sprintf("v%d = ?", fieldIndex+offset), value)
	}
	if err := query.Delete(&rule{}).Error; err != nil {
		return fmt.Errorf("remove filtered casbin policies: %w", err)
	}
	return nil
}
