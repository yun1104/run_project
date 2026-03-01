package database

import (
	"errors"
	"gorm.io/gorm"
)

type VersionedModel struct {
	ID      int64 `gorm:"primaryKey"`
	Version int64 `gorm:"default:0"`
}

func UpdateWithOptimisticLock(db *gorm.DB, table string, id int64, updates map[string]interface{}) error {
	var current struct {
		Version int64
	}
	
	err := db.Table(table).Select("version").Where("id = ?", id).First(&current).Error
	if err != nil {
		return err
	}
	
	updates["version"] = current.Version + 1
	
	result := db.Table(table).
		Where("id = ? AND version = ?", id, current.Version).
		Updates(updates)
	
	if result.Error != nil {
		return result.Error
	}
	
	if result.RowsAffected == 0 {
		return errors.New("update conflict: version mismatch")
	}
	
	return nil
}

type Inventory struct {
	ID       int64 `gorm:"primaryKey"`
	Stock    int   `gorm:"not null"`
	Version  int64 `gorm:"default:0"`
}

func DecrementStock(db *gorm.DB, inventoryID int64, quantity int) error {
	for retries := 0; retries < 3; retries++ {
		var inventory Inventory
		err := db.Where("id = ?", inventoryID).First(&inventory).Error
		if err != nil {
			return err
		}
		
		if inventory.Stock < quantity {
			return errors.New("insufficient stock")
		}
		
		result := db.Model(&Inventory{}).
			Where("id = ? AND version = ?", inventoryID, inventory.Version).
			Updates(map[string]interface{}{
				"stock":   inventory.Stock - quantity,
				"version": inventory.Version + 1,
			})
		
		if result.Error != nil {
			return result.Error
		}
		
		if result.RowsAffected > 0 {
			return nil
		}
	}
	
	return errors.New("failed after retries")
}
