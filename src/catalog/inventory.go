package catalog

import (
	"errors"
	"fmt"
	"sort"

	"backend/src/repository"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func findOne[T any](tx *gorm.DB, out *T, query string, args ...any) (bool, error) {
	err := tx.Where(query, args...).First(out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return err == nil, err
}

// EnsureScentVariants membuat varian yang belum ada untuk 1 wangian × ukuran yang diberikan
// × murni/campur, dan memanggil each untuk setiap varian (lama maupun baru).
// Dipakai seeder dan endpoint admin "buat kombinasi". Mengembalikan jumlah varian baru.
func EnsureScentVariants(tx *gorm.DB, s repository.Scent, sizes []repository.Size, each func(repository.Variant, repository.Size, string)) (int, error) {
	sorted := append([]repository.Size(nil), sizes...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].VolumeML < sorted[j].VolumeML })
	created := 0
	for si, size := range sorted {
		for ci, conc := range repository.Concentrations {
			var v repository.Variant
			found, err := findOne(tx, &v, "product_id = ? AND scent_id = ? AND size_id = ? AND concentration = ?", s.ProductID, s.ID, size.ID, conc)
			if err != nil {
				return created, err
			}
			if !found {
				sid, szid, c := s.ID, size.ID, conc
				// Aktif langsung: harga dihitung dari tarif wangian × ml + botol.
				v = repository.Variant{ProductID: s.ProductID, ScentID: &sid, SizeID: &szid, Concentration: &c,
					Code: repository.VariantCode("", s.Code, size.VolumeML, conc, ""), SortOrder: s.SortOrder*100 + si*10 + ci, IsActive: true}
				if err := tx.Create(&v).Error; err != nil {
					return created, fmt.Errorf("%d ml %s: %w", size.VolumeML, conc, err)
				}
				created++
			}
			if each != nil {
				each(v, size, conc)
			}
		}
	}
	return created, nil
}

// SetStock menyetel stok fisik varian menjadi target dan mencatat selisihnya
// di stock_movements. Baris varian dikunci (FOR UPDATE) supaya tidak balapan
// dengan checkout. changed=false kalau stok sudah sama.
func SetStock(tx *gorm.DB, variantID uint, target int, reason string, userID *uint, note *string) (bool, error) {
	if target < 0 {
		return false, fmt.Errorf("stok tidak boleh negatif")
	}
	var v repository.Variant
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "stock").First(&v, variantID).Error; err != nil {
		return false, err
	}
	delta := target - v.Stock
	if delta == 0 {
		return false, nil
	}
	if err := tx.Model(&repository.Variant{}).Where("id = ?", variantID).
		Updates(map[string]any{"stock": target, "updated_at": gorm.Expr("now()")}).Error; err != nil {
		return false, err
	}
	m := repository.StockMovement{VariantID: variantID, Delta: delta, StockAfter: target, Reason: reason, UserID: userID, Note: note}
	return true, tx.Create(&m).Error
}
