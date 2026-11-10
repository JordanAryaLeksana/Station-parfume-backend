package services

import (
	"backend/src/catalog"
	"backend/src/config"
	"backend/src/modules/products/models"
	"backend/src/repository"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrConflict   = errors.New("already exists")
	ErrValidation = errors.New("validation error")
)

// ProductFilter: semua opsional.
type ProductFilter struct {
	Category string // slug kategori (termasuk sub-kategorinya)
	Kind     string // perfume | general
	Grade    string // regular | exclusive
	Note     string // label aroma, cocok dengan label pertama atau kedua
	Query    string // cari di nama produk, nama wangian, inspired by
}

func (f ProductFilter) validate() error {
	if f.Kind != "" && f.Kind != repository.ProductKindPerfume && f.Kind != repository.ProductKindGeneral {
		return fmt.Errorf("%w: kind harus perfume atau general", ErrValidation)
	}
	if f.Grade != "" && f.Grade != repository.GradeRegular && f.Grade != repository.GradeExclusive {
		return fmt.Errorf("%w: grade harus regular atau exclusive", ErrValidation)
	}
	if f.Note != "" && !slices.Contains(repository.ScentNotes, f.Note) {
		return fmt.Errorf("%w: note %q tidak dikenal", ErrValidation, f.Note)
	}
	return nil
}

// productQuery memuat produk aktif + yang dibutuhkan kartu/halaman produk.
// Varian aktif ikut; yang harganya belum bisa dihitung disaring oleh sellable().
func productQuery() *gorm.DB {
	return config.DB.
		Preload("Category").
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order") }).
		Preload("Sizes").
		Preload("Scents", func(db *gorm.DB) *gorm.DB { return db.Where("is_active").Order("sort_order, id") }).
		Preload("Variants", func(db *gorm.DB) *gorm.DB { return db.Where("is_active").Order("sort_order, id") }).
		Preload("Variants.Scent").
		Preload("Variants.Size").
		Where("products.status = ?", repository.ProductStatusActive)
}

func GetAllProducts(f ProductFilter) ([]models.ProductCard, error) {
	if err := f.validate(); err != nil {
		return nil, err
	}
	q := productQuery()
	if f.Category != "" {
		cats := config.DB.Model(&repository.Category{}).Select("id").
			Where("is_active AND (slug = ? OR parent_id IN (?))", f.Category,
				config.DB.Model(&repository.Category{}).Select("id").Where("slug = ?", f.Category))
		q = q.Where("products.category_id IN (?)", cats)
	}
	if f.Kind != "" {
		q = q.Where("products.kind = ?", f.Kind)
	}
	if f.Grade != "" {
		q = q.Where("products.grade = ?", f.Grade)
	}
	if f.Note != "" {
		q = q.Where("? IN (products.note_primary, products.note_secondary)", f.Note)
	}
	if s := strings.TrimSpace(f.Query); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		q = q.Where("lower(products.name) LIKE ? OR lower(products.inspired_by) LIKE ? OR EXISTS ("+
			"SELECT 1 FROM scents s WHERE s.product_id = products.id AND s.is_active AND s.deleted_at IS NULL AND lower(s.name) LIKE ?)",
			like, like, like)
	}
	var products []repository.Product
	if err := q.Order("products.sort_order, products.id").Find(&products).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve products: %w", err)
	}
	avail, err := availability(products)
	if err != nil {
		return nil, err
	}
	out := make([]models.ProductCard, 0, len(products))
	for _, p := range products {
		// Produk yang belum punya varian berharga (admin belum isi harga) tidak tampil.
		if len(sellable(p)) == 0 {
			continue
		}
		out = append(out, toCard(p, avail))
	}
	return out, nil
}

// GetProduct: idOrSlug boleh angka (id) atau slug ("annasui").
func GetProduct(idOrSlug string) (*models.ProductDetail, error) {
	q := productQuery()
	if id, err := strconv.ParseUint(idOrSlug, 10, 64); err == nil {
		q = q.Where("products.id = ?", id)
	} else {
		q = q.Where("products.slug = ?", idOrSlug)
	}
	var p repository.Product
	err := q.First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && len(sellable(p)) == 0) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve product: %w", err)
	}
	avail, err := availability([]repository.Product{p})
	if err != nil {
		return nil, err
	}
	d := &models.ProductDetail{ProductCard: toCard(p, avail), Description: p.Description, Images: p.Images,
		ScentOptions: []repository.Scent{}, SizeOptions: []repository.Size{}, Concentrations: []string{}, Variants: []models.VariantView{}}
	usedScent, usedSize, usedConc := map[uint]bool{}, map[uint]bool{}, map[string]bool{}
	for _, v := range sellable(p) {
		price, _ := catalog.ListPrice(&v)
		d.Variants = append(d.Variants, models.VariantView{ID: v.ID, ScentID: v.ScentID, SizeID: v.SizeID, Concentration: v.Concentration,
			Label: v.Label, Name: v.DisplayName(), Code: v.Code, Price: price.Total, CompareAtPrice: v.CompareAtPrice,
			InStock: catalog.HasStock(&v, avail, 1), Available: catalog.StockOf(&v, avail)})
		if v.ScentID != nil {
			usedScent[*v.ScentID] = true
		}
		if v.Size != nil && !usedSize[v.Size.ID] {
			usedSize[v.Size.ID] = true
			d.SizeOptions = append(d.SizeOptions, *v.Size)
		}
		if v.Concentration != nil {
			usedConc[*v.Concentration] = true
		}
	}
	// Pilihan hanya yang punya varian yang bisa dibeli; urutan tetap.
	for _, s := range p.Scents {
		if usedScent[s.ID] {
			d.ScentOptions = append(d.ScentOptions, s)
		}
	}
	slices.SortFunc(d.SizeOptions, func(a, b repository.Size) int { return a.VolumeML - b.VolumeML })
	for _, c := range repository.Concentrations {
		if usedConc[c] {
			d.Concentrations = append(d.Concentrations, c)
		}
	}
	return d, nil
}

func GetSizes() ([]repository.Size, error) {
	var sizes []repository.Size
	if err := config.DB.Where("is_active").Order("volume_ml").Find(&sizes).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve sizes: %w", err)
	}
	return sizes, nil
}

// sellable: varian yang tampil di website (lihat catalog.Purchasable).
func sellable(p repository.Product) []repository.Variant {
	out := make([]repository.Variant, 0, len(p.Variants))
	for _, v := range p.Variants {
		v.Product = &p
		if catalog.Purchasable(&v) {
			out = append(out, v)
		}
	}
	return out
}

func availability(products []repository.Product) (map[uint]int, error) {
	var ids []uint
	for _, p := range products {
		for _, v := range p.Variants {
			ids = append(ids, v.ID)
		}
	}
	if len(ids) == 0 {
		return map[uint]int{}, nil
	}
	loc, err := catalog.DefaultLocationID(config.DB)
	if err != nil {
		return nil, err
	}
	return catalog.Available(config.DB, loc, ids)
}

func toCard(p repository.Product, avail map[uint]int) models.ProductCard {
	c := models.ProductCard{ID: p.ID, Kind: p.Kind, Name: p.Name, Slug: p.Slug, Grade: p.Grade, InspiredBy: p.InspiredBy,
		NotePrimary: p.NotePrimary, NoteSecondary: p.NoteSecondary, Category: p.Category}
	if len(p.Images) > 0 {
		c.ImageURL = &p.Images[0].URL
	}
	for _, s := range p.Scents {
		c.Scents = append(c.Scents, s.Name)
	}
	for i, v := range sellable(p) {
		price, _ := catalog.ListPrice(&v)
		if i == 0 || price.Total < c.PriceMin {
			c.PriceMin = price.Total
		}
		if price.Total > c.PriceMax {
			c.PriceMax = price.Total
		}
		if catalog.HasStock(&v, avail, 1) {
			c.InStock = true
		}
	}
	return c
}
