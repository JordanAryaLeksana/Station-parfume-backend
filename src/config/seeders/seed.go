package seeders

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"backend/src/repository"

	"gorm.io/gorm"
)

// CatalogData adalah isi categories.csv, wangian.csv, dan catalog.csv yang sudah diparse.
type CatalogData struct {
	Categories []CategoryRow
	Scents     []ScentRow
	Catalog    []CatalogRow
}

type Counter struct{ Created, Updated int }

func (c Counter) String() string { return fmt.Sprintf("%d baru, %d diperbarui", c.Created, c.Updated) }

type CatalogReport struct {
	Categories, ScentGroups, Scents, Products, Variations, Options, Models Counter
	ScentsDeactivated                                                      int // wangian di DB yang tidak ada lagi di wangian.csv
	ModelsDeactivated                                                      int // model di DB yang tidak ada lagi di CSV
	Warnings                                                               []string
}

// ErrDryRun dikembalikan SeedCatalog saat DryRun=true; semua perubahan di-rollback.
var ErrDryRun = errors.New("dry run: semua perubahan di-rollback")

// SeedCatalog meng-upsert semua data dalam SATU transaksi. Kalau ada error, tidak ada
// yang tersimpan. Tidak ada yang dihapus: model milik produk di CSV yang tidak
// ada lagi di CSV hanya dinonaktifkan (is_active=false), supaya website tetap
// sama dengan Shopee.
func SeedCatalog(db *gorm.DB, d CatalogData, dryRun bool) (*CatalogReport, error) {
	plans, err := planProducts(d.Catalog, d.Categories, d.Scents)
	if err != nil {
		return nil, err
	}
	rep := &CatalogReport{}
	err = db.Transaction(func(tx *gorm.DB) error {
		catIDs, err := upsertCategories(tx, d.Categories, rep)
		if err != nil {
			return err
		}
		scentIDs, err := upsertScents(tx, d.Scents, rep)
		if err != nil {
			return err
		}
		used := map[string]bool{}
		for _, p := range plans {
			if err := upsertProduct(tx, p, catIDs[Slugify(p.Category)], scentIDs, rep); err != nil {
				return fmt.Errorf("produk %q: %w", p.Name, err)
			}
			used[Slugify(p.Category)] = true
		}
		if len(plans) > 0 {
			for _, c := range d.Categories {
				if !used[Slugify(c.Name)] {
					rep.Warnings = append(rep.Warnings, fmt.Sprintf("kategori %q belum punya produk di catalog.csv", c.Name))
				}
			}
		}
		if dryRun {
			return ErrDryRun
		}
		return nil
	})
	return rep, err
}

// ---------------------------------------------------------------------
// Rencana produk dari baris catalog.csv (validasi tanpa DB)
// ---------------------------------------------------------------------

type productPlan struct {
	Name, Slug, Category, ParentSKU string
	WeightGram                      int
	ShopeeItemID                    *int64
	Var1, Var2                      string
	ScentTier                       int               // tier bernama "Aroma" (0/1), -1 = wangian di level produk
	Scent                           string            // kode wangian produk (kalau ScentTier = -1)
	OptionScents                    map[string]string // label pilihan (normLabel) → kode wangian
	Opts1, Opts2                    []string          // urutan = urutan muncul pertama di CSV (= urutan di Shopee)
	Rows                            []CatalogRow
}

func planProducts(rows []CatalogRow, cats []CategoryRow, scents []ScentRow) ([]*productPlan, error) {
	scentCodes := map[string]bool{}
	for _, sc := range scents {
		scentCodes[sc.Code] = true
	}
	catNames := map[string]string{} // slug → nama persis di categories.csv
	for _, c := range cats {
		catNames[Slugify(c.Name)] = c.Name
	}
	var plans []*productPlan
	bySlug := map[string]*productPlan{}
	skus := map[string]int{}

	for _, r := range rows {
		slug := Slugify(r.ProductName)
		p, ok := bySlug[slug]
		if !ok {
			p = &productPlan{Name: r.ProductName, Slug: slug, Category: r.Category, Var1: r.Variation1, Var2: r.Variation2,
				ScentTier: aromaTier(r.Variation1, r.Variation2), OptionScents: map[string]string{}}
			bySlug[slug] = p
			plans = append(plans, p)
		}
		fail := func(format string, a ...any) error {
			return fmt.Errorf("catalog.csv baris %d (%s): %s", r.Line, r.ProductName, fmt.Sprintf(format, a...))
		}
		if p.Name != r.ProductName {
			return nil, fail("nama produk bentrok dengan %q (slug sama %q)", p.Name, slug)
		}
		if _, ok := catNames[Slugify(r.Category)]; !ok {
			return nil, fail("kategori %q tidak ada di categories.csv", r.Category)
		}
		if Slugify(p.Category) != Slugify(r.Category) {
			return nil, fail("kategori %q beda dengan baris sebelumnya (%q)", r.Category, p.Category)
		}
		if p.Var1 != r.Variation1 || p.Var2 != r.Variation2 {
			return nil, fail("nama variasi (%q, %q) harus sama di semua baris produk ini (%q, %q)", r.Variation1, r.Variation2, p.Var1, p.Var2)
		}
		if err := mergeField(&p.ParentSKU, r.ParentSKU); err != nil {
			return nil, fail("parent_sku: %v", err)
		}
		if r.WeightGram != 0 {
			if p.WeightGram != 0 && p.WeightGram != r.WeightGram {
				return nil, fail("weight_gram %d beda dengan baris sebelumnya (%d)", r.WeightGram, p.WeightGram)
			}
			p.WeightGram = r.WeightGram
		}
		if r.ShopeeItemID != nil {
			if p.ShopeeItemID != nil && *p.ShopeeItemID != *r.ShopeeItemID {
				return nil, fail("shopee_item_id beda dengan baris sebelumnya")
			}
			p.ShopeeItemID = r.ShopeeItemID
		}
		if r.SKU != "" {
			if prev, dup := skus[r.SKU]; dup {
				return nil, fail("SKU %q sudah dipakai di baris %d", r.SKU, prev)
			}
			skus[r.SKU] = r.Line
		}
		for _, prev := range p.Rows {
			if normLabel(prev.Option1) == normLabel(r.Option1) && normLabel(prev.Option2) == normLabel(r.Option2) {
				return nil, fail("kombinasi (%q, %q) sudah ada di baris %d", r.Option1, r.Option2, prev.Line)
			}
		}
		if r.Scent != "" {
			if !scentCodes[r.Scent] {
				return nil, fail("scent %q tidak ada di wangian.csv", r.Scent)
			}
			if p.ScentTier < 0 {
				if p.Scent != "" && p.Scent != r.Scent {
					return nil, fail("scent %q beda dengan baris sebelumnya (%q); produk tanpa variasi \"Aroma\" hanya boleh 1 wangian", r.Scent, p.Scent)
				}
				p.Scent = r.Scent
			} else {
				label := normLabel([]string{r.Option1, r.Option2}[p.ScentTier])
				if prev, ok := p.OptionScents[label]; ok && prev != r.Scent {
					return nil, fail("pilihan %q sudah ditautkan ke scent %q", label, prev)
				}
				p.OptionScents[label] = r.Scent
			}
		}
		if p.Var1 == "" && len(p.Rows) > 0 {
			return nil, fail("produk tanpa variasi hanya boleh 1 baris")
		}
		p.Opts1 = appendUnique(p.Opts1, r.Option1)
		p.Opts2 = appendUnique(p.Opts2, r.Option2)
		p.Rows = append(p.Rows, r)
	}
	return plans, nil
}

// aromaTier mengembalikan tier variasi yang bernama "Aroma" (0/1), atau -1.
func aromaTier(var1, var2 string) int {
	for i, v := range []string{var1, var2} {
		if strings.EqualFold(strings.TrimSpace(v), "aroma") {
			return i
		}
	}
	return -1
}

func mergeField(dst *string, v string) error {
	if v == "" {
		return nil
	}
	if *dst != "" && *dst != v {
		return fmt.Errorf("%q beda dengan baris sebelumnya (%q)", v, *dst)
	}
	*dst = v
	return nil
}

func appendUnique(list []string, v string) []string {
	if v == "" {
		return list
	}
	for _, x := range list {
		if normLabel(x) == normLabel(v) {
			return list
		}
	}
	return append(list, v)
}

// ---------------------------------------------------------------------
// Upsert ke DB
// ---------------------------------------------------------------------

func findActive[T any](tx *gorm.DB, out *T, query string, args ...any) (bool, error) {
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

// upsertCategories menyimpan kategori dengan sort_order = urutan di categories.csv.
// Dicocokkan lewat shopee_category_id kalau ada, kalau tidak lewat slug nama.
func upsertCategories(tx *gorm.DB, rows []CategoryRow, rep *CatalogReport) (map[string]uint, error) {
	ids := map[string]uint{}
	for i, r := range rows {
		slug := Slugify(r.Name)
		var c repository.Category
		found := false
		var err error
		if r.ShopeeCategoryID != nil {
			if found, err = findActive(tx, &c, "shopee_category_id = ?", *r.ShopeeCategoryID); err != nil {
				return nil, err
			}
		}
		if !found {
			if found, err = findActive(tx, &c, "slug = ?", slug); err != nil {
				return nil, err
			}
		}
		if found {
			upd := map[string]any{"name": r.Name, "slug": slug, "sort_order": i, "is_active": true}
			if r.ShopeeCategoryID != nil {
				upd["shopee_category_id"] = *r.ShopeeCategoryID
			}
			if err := tx.Model(&c).Updates(upd).Error; err != nil {
				return nil, fmt.Errorf("kategori %q: %w", r.Name, err)
			}
			rep.Categories.Updated++
		} else {
			c = repository.Category{Name: r.Name, Slug: slug, ShopeeCategoryID: r.ShopeeCategoryID, SortOrder: i, IsActive: true}
			if err := tx.Create(&c).Error; err != nil {
				return nil, fmt.Errorf("kategori %q: %w", r.Name, err)
			}
			rep.Categories.Created++
		}
		ids[slug] = c.ID
	}
	return ids, nil
}

// upsertScents menyimpan grup + varian wangian (dicocokkan lewat code). Urutan = urutan di CSV.
// Wangian yang tidak ada lagi di CSV dinonaktifkan, tidak dihapus.
func upsertScents(tx *gorm.DB, rows []ScentRow, rep *CatalogReport) (map[string]uint, error) {
	ids := map[string]uint{}
	if len(rows) == 0 {
		return ids, nil
	}
	groupIDs := map[string]uint{}
	inGroup := map[string]int{}
	for _, r := range rows {
		gid, ok := groupIDs[r.GroupCode]
		if !ok {
			var g repository.ScentGroup
			found, err := findActive(tx, &g, "code = ?", r.GroupCode)
			if err != nil {
				return nil, err
			}
			if found {
				err = tx.Model(&g).Updates(map[string]any{"grade": r.Grade, "name": r.Group, "inspired_by": strPtr(r.InspiredBy),
					"note_primary": r.Note1, "note_secondary": r.Note2, "sort_order": len(groupIDs), "is_active": true}).Error
				rep.ScentGroups.Updated++
			} else {
				g = repository.ScentGroup{Code: r.GroupCode, Grade: r.Grade, Name: r.Group, InspiredBy: strPtr(r.InspiredBy),
					NotePrimary: r.Note1, NoteSecondary: r.Note2, SortOrder: len(groupIDs), IsActive: true}
				err = tx.Create(&g).Error
				rep.ScentGroups.Created++
			}
			if err != nil {
				return nil, fmt.Errorf("grup wangian %s: %w", r.GroupCode, err)
			}
			gid = g.ID
			groupIDs[r.GroupCode] = gid
		}

		var sc repository.Scent
		found, err := findActive(tx, &sc, "code = ?", r.Code)
		if err != nil {
			return nil, err
		}
		order := inGroup[r.GroupCode]
		inGroup[r.GroupCode]++
		if found {
			err = tx.Model(&sc).Updates(map[string]any{"group_id": gid, "name": r.Name, "sort_order": order, "is_active": true}).Error
			rep.Scents.Updated++
		} else {
			sc = repository.Scent{GroupID: gid, Code: r.Code, Name: r.Name, SortOrder: order, IsActive: true}
			err = tx.Create(&sc).Error
			rep.Scents.Created++
		}
		if err != nil {
			return nil, fmt.Errorf("wangian %s (wangian.csv baris %d): %w", r.Code, r.Line, err)
		}
		ids[r.Code] = sc.ID
	}

	codes := make([]string, 0, len(ids))
	for c := range ids {
		codes = append(codes, c)
	}
	res := tx.Model(&repository.Scent{}).Where("is_active AND code NOT IN ?", codes).Update("is_active", false)
	if res.Error != nil {
		return nil, res.Error
	}
	rep.ScentsDeactivated = int(res.RowsAffected)
	gcodes := make([]string, 0, len(groupIDs))
	for c := range groupIDs {
		gcodes = append(gcodes, c)
	}
	if err := tx.Model(&repository.ScentGroup{}).Where("is_active AND code NOT IN ?", gcodes).Update("is_active", false).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func scentIDPtr(ids map[string]uint, code string) *uint {
	if code == "" {
		return nil
	}
	id := ids[code]
	return &id
}

func upsertProduct(tx *gorm.DB, p *productPlan, categoryID uint, scentIDs map[string]uint, rep *CatalogReport) error {
	var prod repository.Product
	found, err := findActive(tx, &prod, "slug = ?", p.Slug)
	if err != nil {
		return err
	}
	if found {
		upd := map[string]any{"name": p.Name, "category_id": categoryID, "scent_id": scentIDPtr(scentIDs, p.Scent),
			"parent_sku": strPtr(p.ParentSKU), "weight_gram": p.WeightGram}
		if p.ShopeeItemID != nil {
			upd["shopee_item_id"] = *p.ShopeeItemID
		}
		if err := tx.Model(&prod).Updates(upd).Error; err != nil {
			return err
		}
		rep.Products.Updated++
	} else {
		prod = repository.Product{Name: p.Name, Slug: p.Slug, CategoryID: categoryID, ScentID: scentIDPtr(scentIDs, p.Scent), Status: repository.ProductStatusActive,
			ParentSKU: strPtr(p.ParentSKU), WeightGram: p.WeightGram, ShopeeItemID: p.ShopeeItemID}
		if err := tx.Create(&prod).Error; err != nil {
			return err
		}
		rep.Products.Created++
	}

	// variasi + pilihan
	optIDs := [2]map[string]uint{{}, {}}
	for tier, v := range []struct {
		name string
		opts []string
	}{{p.Var1, p.Opts1}, {p.Var2, p.Opts2}} {
		if v.name == "" {
			continue
		}
		var pv repository.ProductVariation
		found, err := findActive(tx, &pv, "product_id = ? AND tier_index = ?", prod.ID, tier)
		if err != nil {
			return err
		}
		if found {
			if err := tx.Model(&pv).Update("name", v.name).Error; err != nil {
				return err
			}
			rep.Variations.Updated++
		} else {
			pv = repository.ProductVariation{ProductID: prod.ID, TierIndex: int16(tier), Name: v.name}
			if err := tx.Create(&pv).Error; err != nil {
				return err
			}
			rep.Variations.Created++
		}
		var optScents map[string]*uint
		if tier == p.ScentTier {
			optScents = map[string]*uint{}
			for label, code := range p.OptionScents {
				optScents[label] = scentIDPtr(scentIDs, code)
			}
		}
		ids, err := upsertOptions(tx, prod.ID, pv.ID, v.opts, optScents, rep)
		if err != nil {
			return fmt.Errorf("variasi %q: %w", v.name, err)
		}
		optIDs[tier] = ids
	}

	// model / SKU
	touched := make([]uint, 0, len(p.Rows))
	for _, r := range p.Rows {
		var o1, o2 *uint
		if r.Option1 != "" {
			id := optIDs[0][normLabel(r.Option1)]
			o1 = &id
		}
		if r.Option2 != "" {
			id := optIDs[1][normLabel(r.Option2)]
			o2 = &id
		}
		q := tx.Where("product_id = ?", prod.ID)
		q = whereOpt(q, "option1_id", o1)
		q = whereOpt(q, "option2_id", o2)
		var m repository.ProductModel
		err := q.First(&m).Error
		switch {
		case err == nil:
			upd := map[string]any{"sku": strPtr(r.SKU), "price": r.Price, "original_price": r.OriginalPrice, "stock": r.Stock, "is_active": true}
			if r.ShopeeModelID != nil {
				upd["shopee_model_id"] = *r.ShopeeModelID
			}
			if err := tx.Model(&m).Updates(upd).Error; err != nil {
				return fmt.Errorf("catalog.csv baris %d: %w", r.Line, err)
			}
			rep.Models.Updated++
		case errors.Is(err, gorm.ErrRecordNotFound):
			m = repository.ProductModel{ProductID: prod.ID, Option1ID: o1, Option2ID: o2, SKU: strPtr(r.SKU), Price: r.Price,
				OriginalPrice: r.OriginalPrice, Stock: r.Stock, ShopeeModelID: r.ShopeeModelID, IsActive: true}
			if err := tx.Create(&m).Error; err != nil {
				return fmt.Errorf("catalog.csv baris %d: %w", r.Line, err)
			}
			rep.Models.Created++
		default:
			return err
		}
		touched = append(touched, m.ID)
	}

	res := tx.Model(&repository.ProductModel{}).
		Where("product_id = ? AND is_active AND id NOT IN ?", prod.ID, touched).
		Update("is_active", false)
	if res.Error != nil {
		return res.Error
	}
	rep.ModelsDeactivated += int(res.RowsAffected)
	return nil
}

func whereOpt(q *gorm.DB, col string, id *uint) *gorm.DB {
	if id == nil {
		return q.Where(col + " IS NULL")
	}
	return q.Where(col+" = ?", *id)
}

// upsertOptions menyimpan pilihan dengan option_index = urutan di CSV.
// Pilihan lama yang tidak ada di CSV tidak dihapus, tapi dipindah ke urutan paling belakang.
func upsertOptions(tx *gorm.DB, productID, variationID uint, labels []string, scents map[string]*uint, rep *CatalogReport) (map[string]uint, error) {
	var existing []repository.ProductVariationOption
	if err := tx.Where("variation_id = ?", variationID).Find(&existing).Error; err != nil {
		return nil, err
	}
	byLabel := map[string]*repository.ProductVariationOption{}
	for i := range existing {
		byLabel[normLabel(existing[i].Label)] = &existing[i]
	}
	// geser dulu semua index supaya penomoran ulang tidak bentrok dengan unique (variation_id, option_index)
	if len(existing) > 0 {
		if err := tx.Model(&repository.ProductVariationOption{}).Where("variation_id = ?", variationID).
			Update("option_index", gorm.Expr("option_index + 10000")).Error; err != nil {
			return nil, err
		}
	}

	ids := map[string]uint{}
	inCSV := map[uint]bool{}
	for i, label := range labels {
		key := normLabel(label)
		if o, ok := byLabel[key]; ok {
			err := tx.Model(o).Updates(map[string]any{"option_index": i, "label": label, "scent_id": scents[key]}).Error
			if err != nil {
				return nil, fmt.Errorf("pilihan %q: %w", label, err)
			}
			ids[key] = o.ID
			inCSV[o.ID] = true
			rep.Options.Updated++
			continue
		}
		o := repository.ProductVariationOption{VariationID: variationID, ProductID: productID, OptionIndex: int16(i), Label: label, ScentID: scents[key]}
		if err := tx.Create(&o).Error; err != nil {
			return nil, fmt.Errorf("pilihan %q: %w", label, err)
		}
		ids[key] = o.ID
		rep.Options.Created++
	}

	var leftovers []repository.ProductVariationOption
	for _, o := range existing {
		if !inCSV[o.ID] {
			leftovers = append(leftovers, o)
		}
	}
	sort.Slice(leftovers, func(i, j int) bool { return leftovers[i].OptionIndex < leftovers[j].OptionIndex })
	for k, o := range leftovers {
		if err := tx.Model(&repository.ProductVariationOption{}).Where("id = ?", o.ID).Update("option_index", len(labels)+k).Error; err != nil {
			return nil, err
		}
	}
	return ids, nil
}
