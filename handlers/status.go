// Copyright 2025 The OSAC Authors
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DBStatusUpdater implements workflow.StatusUpdater by directly updating
// the status JSONB column in a given table via GORM.
type DBStatusUpdater struct {
	db    *gorm.DB
	table string
}

// NewDBStatusUpdater creates a status updater for the given table name.
func NewDBStatusUpdater(db *gorm.DB, table string) *DBStatusUpdater {
	return &DBStatusUpdater{db: db, table: table}
}

// UpdateStatus updates the status fields of a resource identified by
// org_id and name.
func (u *DBStatusUpdater) UpdateStatus(ctx context.Context, orgID uuid.UUID, name string, fields map[string]interface{}) error {
	status, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("marshaling status: %w", err)
	}

	result := u.db.WithContext(ctx).
		Table(u.table).
		Where("org_id = ? AND name = ? AND deleted_at IS NULL", orgID, name).
		Updates(map[string]interface{}{
			"status":     string(status),
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		return fmt.Errorf("updating status in %s: %w", u.table, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("resource %s/%s not found in %s", orgID, name, u.table)
	}
	return nil
}
