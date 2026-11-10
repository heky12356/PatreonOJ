package models

import (
	"errors"
	"gorm.io/gorm"
)

// EnsureBuiltinRoles creates missing defaults without resetting existing access
// policies or assigning a role to any account.
func EnsureBuiltinRoles(db *gorm.DB) error {
	definitions := []struct{ Name, Description string }{
		{"admin", "管理员"}, {"moderator", "版主"}, {"teacher", "教师"}, {"student", "学生"}, {"user", "普通用户"},
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for _, def := range definitions {
			var permission Permission
			err := tx.Unscoped().Where("code = ?", def.Name).First(&permission).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				permission = Permission{Code: def.Name, Description: def.Description}
				if err := tx.Create(&permission).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
			var role Role
			err = tx.Unscoped().Where("name = ?", def.Name).First(&role).Error
			if err == nil {
				continue
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			role = Role{Name: def.Name, Description: def.Description}
			if err := tx.Create(&role).Error; err != nil {
				return err
			}
			if !permission.DeletedAt.Valid {
				if err := tx.Model(&role).Association("Permissions").Append(&permission); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
