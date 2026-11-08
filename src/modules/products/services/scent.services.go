package services

import (
	"backend/src/config"
	"backend/src/repository"
	"fmt"
	"slices"

	"gorm.io/gorm"
)

// GetScentGroups mengembalikan wangian per foto produk (grup + varian), urut seperti
// di seed/wangian.csv. grade ("regular"/"exclusive") dan note (label aroma,
// mis. "woody") opsional; note cocok dengan label pertama maupun kedua.
func GetScentGroups(grade, note string) ([]repository.ScentGroup, error) {
	if grade != "" && grade != repository.GradeRegular && grade != repository.GradeExclusive {
		return nil, fmt.Errorf("%w: grade harus regular atau exclusive", ErrValidation)
	}
	if note != "" && !slices.Contains(repository.ScentNotes, note) {
		return nil, fmt.Errorf("%w: note %q tidak dikenal", ErrValidation, note)
	}
	q := config.DB.
		Preload("Scents", func(db *gorm.DB) *gorm.DB { return db.Where("is_active").Order("sort_order, id") }).
		Where("is_active")
	if grade != "" {
		q = q.Where("grade = ?", grade)
	}
	if note != "" {
		q = q.Where("? IN (note_primary, note_secondary)", note)
	}
	var groups []repository.ScentGroup
	if err := q.Order("sort_order, id").Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve scents: %w", err)
	}
	return groups, nil
}
