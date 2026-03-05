// Package model contains all database model structs.
// Each struct maps directly to one database table.
//
// GORM CONVENTIONS we follow:
//   - Field `ID` maps to primary key column `id`
//   - `gorm:"column:xyz"` overrides the column name
//   - `gorm:"type:..."` sets the exact SQL type
//   - `json:"..."` controls JSON serialization (for API responses)
//   - `json:"-"` hides sensitive fields like passwords from JSON output
package model

import "time"

// Base contains common fields shared by most tables.
// Embedding this struct in your model automatically adds these fields.
//
// WHY EMBED INSTEAD OF GORM'S gorm.Model?
// GORM's built-in gorm.Model uses uint for ID and assumes auto-increment.
// We use CHAR(36) UUIDs, so we define our own base.
// Also, gorm.Model has deleted_at built-in but we want to control it explicitly.
type Base struct {
	CreatedAt time.Time  `gorm:"column:created_at;autoCreateTime"  json:"created_at"`
	UpdatedAt time.Time  `gorm:"column:updated_at;autoUpdateTime"  json:"updated_at"`
	DeletedAt *time.Time `gorm:"column:deleted_at;index"           json:"deleted_at,omitempty"`
}

// SoftDelete implements GORM's soft delete interface.
// When you call db.Delete(&record), GORM sets deleted_at instead of issuing a SQL DELETE.
// When you query, GORM automatically adds WHERE deleted_at IS NULL.
