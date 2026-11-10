package seeders

import (
	"errors"
	"fmt"

	"backend/src/catalog"
	"backend/src/repository"

	"gorm.io/gorm"
)

// CatalogData adalah isi semua CSV katalog yang sudah diparse.
type CatalogData struct {
	Categories []CategoryRow
	Sizes      []SizeRow
	Scents     []ScentRow
	Variants   []PerfumeVariantRow
	General    []GeneralRow
}

type Counter struct{ Created, Updated int }

func (c Counter) String() string { return fmt.Sprintf("%d baru, %d diperbarui", c.Created, c.Updated) }

type CatalogReport struct {
	Categories, Sizes, Products, Scents, Variants Counter
	PricesApplied                                 int // baris varian_parfum.csv / produk_umum.csv yang mengubah harga/status
	StockChanged                                  int // varian yang stoknya berubah (tercatat di stock_movements)
	Archived                                      int // produk/wangian yang tidak ada lagi di CSV (dinonaktifkan)
	Unpriced                                      int // wangian aktif tanpa tarif + produk umum tanpa harga (tidak tampil)
}

// ErrDryRun dikembalikan SeedCatalog saat dryRun=true; semua perubahan di-rollback.
var ErrDryRun = errors.New("dry run: semua perubahan di-rollback")

// SeedCatalog meng-upsert semua data dalam SATU transaksi; kalau ada error,
// tidak ada yang tersimpan. Tidak ada yang dihapus: produk/wangian yang tidak
// ada lagi di CSV hanya dinonaktifkan. Varian yang tidak disebut di varian_parfum.csv
// dan kolom yang kosong tidak diubah (harga yang diisi admin lewat API tetap aman).
func SeedCatalog(db *gorm.DB, d CatalogData, dryRun bool) (*CatalogReport, error) {
	rep := &CatalogReport{}
	err := db.Transaction(func(tx *gorm.DB) error {
		catIDs, err := upsertCategories(tx, d.Categories, rep)
		if err != nil {
			return err
		}
		sizes, err := upsertSizes(tx, d.Sizes, rep)
		if err != nil {
			return err
		}
		sizeIDs := map[int]uint{}
		for _, sz := range sizes {
			sizeIDs[sz.VolumeML] = sz.ID
		}
		locationID, err := EnsureDefaultLocation(tx)
		if err != nil {
			return err
		}
		scentIDs, err := upsertPerfumes(tx, d.Scents, catIDs, sizeIDs, rep)
		if err != nil {
			return err
		}
		variants, err := generatePerfumeVariants(tx, d.Scents, scentIDs, rep)
		if err != nil {
			return err
		}
		if err := applyPerfumeVariants(tx, d.Variants, variants, locationID, rep); err != nil {
			return err
		}
		if err := upsertGeneral(tx, d.General, catIDs, locationID, rep); err != nil {
			return err
		}
		var noRate, noPrice int64
		if err := tx.Model(&repository.Scent{}).
			Joins("JOIN products p ON p.id = scents.product_id AND p.status = ?", repository.ProductStatusActive).
			Where("scents.is_active AND (scents.price_per_ml_murni IS NULL OR scents.price_per_ml_campur IS NULL)").Count(&noRate).Error; err != nil {
			return err
		}
		if err := tx.Model(&repository.Variant{}).Where("scent_id IS NULL AND price IS NULL").Count(&noPrice).Error; err != nil {
			return err
		}
		rep.Unpriced = int(noRate + noPrice)
		if dryRun {
			return ErrDryRun
		}
		return nil
	})
	return rep, err
}

// EnsureDefaultLocation memastikan ada 1 lokasi stok default ("Toko Utama").
func EnsureDefaultLocation(tx *gorm.DB) (uint, error) {
	var loc repository.Location
	err := tx.Where("is_default").First(&loc).Error
	if err == nil {
		return loc.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	loc = repository.Location{Code: "utama", Name: "Toko Utama", IsDefault: true, IsActive: true}
	if err := tx.Create(&loc).Error; err != nil {
		return 0, fmt.Errorf("lokasi default: %w", err)
	}
	return loc.ID, nil
}

func findOne[T any](tx *gorm.DB, out *T, query string, args ...any) (bool, error) {
	err := tx.Where(query, args...).First(out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return err == nil, err
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func upsertCategories(tx *gorm.DB, rows []CategoryRow, rep *CatalogReport) (map[string]uint, error) {
	ids := map[string]uint{}
	var existing []repository.Category
	if err := tx.Find(&existing).Error; err != nil {
		return nil, err
	}
	for _, c := range existing {
		ids[c.Slug] = c.ID
	}
	for i, r := range rows {
		slug := Slugify(r.Name)
		var c repository.Category
		found, err := findOne(tx, &c, "slug = ?", slug)
		if err != nil {
			return nil, err
		}
		if found {
			err = tx.Model(&c).Updates(map[string]any{"name": r.Name, "sort_order": i, "is_active": true}).Error
			rep.Categories.Updated++
		} else {
			c = repository.Category{Name: r.Name, Slug: slug, SortOrder: i, IsActive: true}
			err = tx.Create(&c).Error
			rep.Categories.Created++
		}
		if err != nil {
			return nil, fmt.Errorf("kategori %q: %w", r.Name, err)
		}
		ids[slug] = c.ID
	}
	return ids, nil
}

func categoryID(ids map[string]uint, name string) (uint, error) {
	id, ok := ids[Slugify(name)]
	if !ok {
		return 0, fmt.Errorf("kategori %q tidak ada (tambahkan ke categories.csv)", name)
	}
	return id, nil
}

// upsertSizes mengembalikan semua ukuran aktif, urut dari yang terkecil.
func upsertSizes(tx *gorm.DB, rows []SizeRow, rep *CatalogReport) ([]repository.Size, error) {
	for i, r := range rows {
		var s repository.Size
		found, err := findOne(tx, &s, "volume_ml = ?", r.VolumeML)
		if err != nil {
			return nil, err
		}
		if found {
			err = tx.Model(&s).Updates(map[string]any{"label": r.Label, "bottle_price": r.BottlePrice, "weight_gram": r.WeightGram,
				"is_default": r.IsDefault, "sort_order": i, "is_active": true}).Error
			rep.Sizes.Updated++
		} else {
			s = repository.Size{Label: r.Label, VolumeML: r.VolumeML, BottlePrice: r.BottlePrice, WeightGram: r.WeightGram,
				IsDefault: r.IsDefault, SortOrder: i, IsActive: true}
			err = tx.Create(&s).Error
			rep.Sizes.Created++
		}
		if err != nil {
			return nil, fmt.Errorf("ukuran %d ml: %w", r.VolumeML, err)
		}
	}
	var all []repository.Size
	if err := tx.Where("is_active").Order("volume_ml").Find(&all).Error; err != nil {
		return nil, err
	}
	return all, nil
}

// upsertPerfumes: 1 group_code = 1 produk parfum, 1 baris = 1 wangian.
// Mengembalikan kode wangian → id.
func upsertPerfumes(tx *gorm.DB, rows []ScentRow, catIDs map[string]uint, sizeIDs map[int]uint, rep *CatalogReport) (map[string]uint, error) {
	scentIDs := map[string]uint{}
	if len(rows) == 0 {
		return scentIDs, nil
	}
	productIDs := map[string]uint{}
	inProduct := map[string]int{}
	for _, r := range rows {
		pid, ok := productIDs[r.GroupCode]
		if !ok {
			catID, err := categoryID(catIDs, r.Category)
			if err != nil {
				return nil, fmt.Errorf("wangian.csv baris %d: %w", r.Line, err)
			}
			grade, n1, n2 := r.Grade, r.Note1, r.Note2
			status := repository.ProductStatusActive
			if !r.Active {
				status = repository.ProductStatusDraft
			}
			var p repository.Product
			found, err := findOne(tx, &p, "code = ?", r.GroupCode)
			if err != nil {
				return nil, err
			}
			if found {
				if p.Kind != repository.ProductKindPerfume {
					return nil, fmt.Errorf("wangian.csv baris %d: group_code %s sudah dipakai produk umum", r.Line, r.GroupCode)
				}
				err = tx.Model(&p).Updates(map[string]any{"category_id": catID, "name": r.Group, "slug": r.GroupCode,
					"grade": grade, "inspired_by": strPtr(r.InspiredBy), "note_primary": n1, "note_secondary": n2,
					"status": status, "sort_order": len(productIDs)}).Error
				rep.Products.Updated++
			} else {
				// Parfum diracik sendiri → stok tidak dihitung (TrackStock false), selalu tersedia.
				p = repository.Product{CategoryID: catID, Kind: repository.ProductKindPerfume, Code: r.GroupCode, Name: r.Group, Slug: r.GroupCode,
					Grade: &grade, InspiredBy: strPtr(r.InspiredBy), NotePrimary: &n1, NoteSecondary: &n2,
					Status: status, SortOrder: len(productIDs)}
				err = tx.Create(&p).Error
				rep.Products.Created++
			}
			if err != nil {
				return nil, fmt.Errorf("produk %s (wangian.csv baris %d): %w", r.GroupCode, r.Line, err)
			}
			pid = p.ID
			productIDs[r.GroupCode] = pid
			if err := syncProductSizes(tx, pid, r.Sizes, sizeIDs); err != nil {
				return nil, fmt.Errorf("wangian.csv baris %d: %w", r.Line, err)
			}
		}

		var s repository.Scent
		found, err := findOne(tx, &s, "code = ?", r.Code)
		if err != nil {
			return nil, err
		}
		order := inProduct[r.GroupCode]
		inProduct[r.GroupCode]++
		if found {
			if s.ProductID != pid {
				return nil, fmt.Errorf("wangian.csv baris %d: code %s sudah milik produk lain; buat code baru", r.Line, r.Code)
			}
			// tarif kosong di CSV = tidak diubah (tarif yang diisi admin tetap aman)
			upd := map[string]any{"name": r.Name, "sort_order": order, "is_active": true}
			if r.RateMurni != nil {
				upd["price_per_ml_murni"] = *r.RateMurni
			}
			if r.RateCampur != nil {
				upd["price_per_ml_campur"] = *r.RateCampur
			}
			err = tx.Model(&s).Updates(upd).Error
			rep.Scents.Updated++
		} else {
			s = repository.Scent{ProductID: pid, Code: r.Code, Name: r.Name, SortOrder: order, IsActive: true,
				PricePerMLMurni: r.RateMurni, PricePerMLCampur: r.RateCampur}
			err = tx.Create(&s).Error
			rep.Scents.Created++
		}
		if err != nil {
			return nil, fmt.Errorf("wangian %s (wangian.csv baris %d): %w", r.Code, r.Line, err)
		}
		scentIDs[r.Code] = s.ID
	}

	// Yang tidak ada lagi di wangian.csv dinonaktifkan (tidak dihapus).
	res := tx.Model(&repository.Scent{}).Where("is_active AND code NOT IN ?", keys(scentIDs)).Update("is_active", false)
	if res.Error != nil {
		return nil, res.Error
	}
	rep.Archived += int(res.RowsAffected)
	res = tx.Model(&repository.Product{}).
		Where("kind = ? AND status <> ? AND code NOT IN ?", repository.ProductKindPerfume, repository.ProductStatusArchived, keys(productIDs)).
		Update("status", repository.ProductStatusArchived)
	if res.Error != nil {
		return nil, res.Error
	}
	rep.Archived += int(res.RowsAffected)
	return scentIDs, nil
}

func variantKey(scentCode string, ml int, conc string) string {
	return fmt.Sprintf("%s|%d|%s", scentCode, ml, conc)
}

// syncProductSizes menyetel ukuran khusus produk (product_sizes). Daftar kosong =
// produk memakai ukuran default.
func syncProductSizes(tx *gorm.DB, productID uint, mls []int, sizeIDs map[int]uint) error {
	if err := tx.Where("product_id = ?", productID).Delete(&repository.ProductSize{}).Error; err != nil {
		return err
	}
	for _, ml := range mls {
		id, ok := sizeIDs[ml]
		if !ok {
			return fmt.Errorf("ukuran %d ml tidak ada di sizes.csv", ml)
		}
		if err := tx.Create(&repository.ProductSize{ProductID: productID, SizeID: id}).Error; err != nil {
			return err
		}
	}
	return nil
}

// generatePerfumeVariants = tombol "Buat kombinasi": setiap wangian × ukuran produk
// (khusus atau default) × murni/campur dibuat kalau belum ada. Harga mengikuti rumus.
func generatePerfumeVariants(tx *gorm.DB, rows []ScentRow, scentIDs map[string]uint, rep *CatalogReport) (map[string]uint, error) {
	out := map[string]uint{}
	sizesOf := map[uint][]repository.Size{}
	for _, r := range rows {
		var s repository.Scent
		if err := tx.First(&s, scentIDs[r.Code]).Error; err != nil {
			return nil, err
		}
		sizes, ok := sizesOf[s.ProductID]
		if !ok {
			var err error
			if sizes, err = catalog.SizesForProduct(tx, s.ProductID); err != nil {
				return nil, err
			}
			sizesOf[s.ProductID] = sizes
		}
		n, err := catalog.EnsureScentVariants(tx, s, sizes, func(v repository.Variant, size repository.Size, conc string) {
			out[variantKey(r.Code, size.VolumeML, conc)] = v.ID
		})
		if err != nil {
			return nil, fmt.Errorf("varian %s: %w", r.Code, err)
		}
		rep.Variants.Created += n
	}
	return out, nil
}

func applyPerfumeVariants(tx *gorm.DB, rows []PerfumeVariantRow, variants map[string]uint, locationID uint, rep *CatalogReport) error {
	for _, r := range rows {
		id, ok := variants[variantKey(r.ScentCode, r.VolumeML, r.Concentration)]
		if !ok {
			return fmt.Errorf("varian_parfum.csv baris %d: %s %d ml %s tidak dikenal (scent_code harus ada di wangian.csv dan ukurannya dijual untuk produk itu)",
				r.Line, r.ScentCode, r.VolumeML, r.Concentration)
		}
		if err := applyVariant(tx, id, r.SpecialPrice, r.CompareAtPrice, r.Active, r.Stock, false, locationID, rep); err != nil {
			return fmt.Errorf("varian_parfum.csv baris %d: %w", r.Line, err)
		}
	}
	return nil
}

// applyVariant menerapkan 1 baris CSV ke varian. Kolom kosong = TIDAK diubah,
// jadi harga/stok yang sudah diisi admin lewat API tidak terhapus saat seeder
// dijalankan ulang. Produk umum: harga diisi → varian aktif (activateOnPrice).
func applyVariant(tx *gorm.DB, id uint, price, compare *int64, active *bool, stock *int, activateOnPrice bool, locationID uint, rep *CatalogReport) error {
	upd := map[string]any{}
	if price != nil {
		upd["price"] = *price
		if activateOnPrice {
			upd["is_active"] = true
		}
	}
	if compare != nil {
		upd["compare_at_price"] = *compare
	}
	if active != nil {
		upd["is_active"] = *active
	}
	if len(upd) > 0 {
		if err := tx.Model(&repository.Variant{}).Where("id = ?", id).Updates(upd).Error; err != nil {
			return err
		}
		rep.PricesApplied++
	}
	if stock != nil {
		changed, err := catalog.SetStock(tx, id, locationID, *stock, repository.MovementOpname, nil, nil)
		if err != nil {
			return err
		}
		if changed {
			rep.StockChanged++
		}
	}
	return nil
}

func upsertGeneral(tx *gorm.DB, rows []GeneralRow, catIDs map[string]uint, locationID uint, rep *CatalogReport) error {
	productIDs := map[string]uint{}
	for i, r := range rows {
		pid, ok := productIDs[r.ProductCode]
		if !ok {
			catID, err := categoryID(catIDs, r.Category)
			if err != nil {
				return fmt.Errorf("produk_umum.csv baris %d: %w", r.Line, err)
			}
			var p repository.Product
			found, err := findOne(tx, &p, "code = ?", r.ProductCode)
			if err != nil {
				return err
			}
			if found {
				if p.Kind != repository.ProductKindGeneral {
					return fmt.Errorf("produk_umum.csv baris %d: product_code %s sudah dipakai produk parfum", r.Line, r.ProductCode)
				}
				err = tx.Model(&p).Updates(map[string]any{"category_id": catID, "name": r.ProductName, "slug": r.ProductCode,
					"weight_gram": r.WeightGram, "status": repository.ProductStatusActive, "sort_order": 1000 + len(productIDs)}).Error
				rep.Products.Updated++
			} else {
				// Produk umum = barang jadi → stok dihitung (bisa dimatikan admin per produk).
				p = repository.Product{CategoryID: catID, Kind: repository.ProductKindGeneral, Code: r.ProductCode, Name: r.ProductName,
					Slug: r.ProductCode, WeightGram: r.WeightGram, TrackStock: true, Status: repository.ProductStatusActive, SortOrder: 1000 + len(productIDs)}
				err = tx.Create(&p).Error
				rep.Products.Created++
			}
			if err != nil {
				return fmt.Errorf("produk %s (produk_umum.csv baris %d): %w", r.ProductCode, r.Line, err)
			}
			pid = p.ID
			productIDs[r.ProductCode] = pid
		}

		var v repository.Variant
		q := tx.Where("product_id = ? AND scent_id IS NULL", pid)
		if r.Label == "" {
			q = q.Where("label IS NULL")
		} else {
			q = q.Where("lower(label) = lower(?)", r.Label)
		}
		err := q.First(&v).Error
		switch {
		case err == nil:
			if err := tx.Model(&v).Updates(map[string]any{"label": strPtr(r.Label), "sort_order": i}).Error; err != nil {
				return err
			}
			rep.Variants.Updated++
		case errors.Is(err, gorm.ErrRecordNotFound):
			v = repository.Variant{ProductID: pid, Label: strPtr(r.Label), SortOrder: i,
				Code: repository.VariantCode(r.ProductCode, "", 0, "", Slugify(r.Label))}
			if err := tx.Create(&v).Error; err != nil {
				return fmt.Errorf("produk_umum.csv baris %d: %w", r.Line, err)
			}
			rep.Variants.Created++
		default:
			return err
		}
		if err := applyVariant(tx, v.ID, r.Price, nil, r.Active, r.Stock, true, locationID, rep); err != nil {
			return fmt.Errorf("produk_umum.csv baris %d: %w", r.Line, err)
		}
	}
	return nil
}
