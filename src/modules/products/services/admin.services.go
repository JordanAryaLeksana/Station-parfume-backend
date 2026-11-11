package services

// Endpoint admin katalog. Alur input admin:
//   1. Buat produk parfum (grup foto) → 2. tambah wangian + tarif per ml (kombinasi
//   ukuran × murni/campur dibuat otomatis; harga = tarif × ml + botol) →
//   3. isi stok produk umum (POST /admin/inventory/adjustments; tercatat di
//   stock_movements). Harga khusus per varian opsional.

import (
	"backend/src/catalog"
	"backend/src/config"
	"backend/src/repository"
	"backend/src/utils/slug"
	"errors"
	"fmt"
	"slices"

	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var validate = validator.New()

type ProductInput struct {
	Kind          string  `json:"kind" validate:"required,oneof=perfume general"`
	CategoryID    uint    `json:"category_id" validate:"required"`
	Name          string  `json:"name" validate:"required,max=255"`
	Code          string  `json:"code" validate:"omitempty,max=60"` // kosong = dari nama
	Description   string  `json:"description"`
	Grade         *string `json:"grade" validate:"omitempty,oneof=regular exclusive"`
	InspiredBy    *string `json:"inspired_by" validate:"omitempty,max=255"`
	NotePrimary   *string `json:"note_primary"`
	NoteSecondary *string `json:"note_secondary"`
	WeightGram    int     `json:"weight_gram" validate:"gte=0"`
	TrackStock    *bool   `json:"track_stock"` // kosong: parfum = false (selalu tersedia), produk umum = true
	Status        string  `json:"status" validate:"omitempty,oneof=draft active archived"`
}

type ProductPatch struct {
	CategoryID    *uint   `json:"category_id"`
	Name          *string `json:"name" validate:"omitempty,max=255"`
	Description   *string `json:"description"`
	Grade         *string `json:"grade" validate:"omitempty,oneof=regular exclusive"`
	InspiredBy    *string `json:"inspired_by" validate:"omitempty,max=255"`
	NotePrimary   *string `json:"note_primary"`
	NoteSecondary *string `json:"note_secondary"`
	WeightGram    *int    `json:"weight_gram" validate:"omitempty,gte=0"`
	TrackStock    *bool   `json:"track_stock"`
	Status        *string `json:"status" validate:"omitempty,oneof=draft active archived"`
	SortOrder     *int    `json:"sort_order"`
}

// Tarif per ml: murni = "bibit", campur = "parfum".
type ScentInput struct {
	Name             string `json:"name" validate:"required,max=100"`
	Code             string `json:"code" validate:"omitempty,max=100"` // kosong = <kode produk>-<nama>
	PricePerMLMurni  *int64 `json:"price_per_ml_murni" validate:"omitempty,gt=0"`
	PricePerMLCampur *int64 `json:"price_per_ml_campur" validate:"omitempty,gt=0"`
}

// ScentPatch: field nil = tidak diubah; tarif 0 = dikosongkan (wangian tidak dijual).
type ScentPatch struct {
	Name             *string `json:"name" validate:"omitempty,max=100"`
	IsActive         *bool   `json:"is_active"`
	SortOrder        *int    `json:"sort_order"`
	PricePerMLMurni  *int64  `json:"price_per_ml_murni" validate:"omitempty,gte=0"`
	PricePerMLCampur *int64  `json:"price_per_ml_campur" validate:"omitempty,gte=0"`
}

// RatesInput: ubah tarif semua wangian di 1 produk sekaligus (mis. semua varian Victoria).
type RatesInput struct {
	PricePerMLMurni  *int64 `json:"price_per_ml_murni" validate:"omitempty,gt=0"`
	PricePerMLCampur *int64 `json:"price_per_ml_campur" validate:"omitempty,gt=0"`
}

// ProductSizesInput: ukuran khusus produk. Kosong = kembali ke ukuran default.
type ProductSizesInput struct {
	SizeIDs []uint `json:"size_ids"`
}

type GeneralVariantInput struct {
	Label          *string `json:"label" validate:"omitempty,max=100"`
	Price          *int64  `json:"price" validate:"omitempty,gt=0"`
	CompareAtPrice *int64  `json:"compare_at_price" validate:"omitempty,gt=0"`
	IsActive       bool    `json:"is_active"`
}

// VariantPatch: field nil = tidak diubah. Price/CompareAtPrice = 0 → dikosongkan.
// Parfum: Price = harga khusus (0 = kembali ke rumus tarif × ml + botol).
type VariantPatch struct {
	ID             uint   `json:"id" validate:"required"`
	Price          *int64 `json:"price" validate:"omitempty,gte=0"`
	CompareAtPrice *int64 `json:"compare_at_price" validate:"omitempty,gte=0"`
	IsActive       *bool  `json:"is_active"`
	WeightGram     *int   `json:"weight_gram" validate:"omitempty,gte=0"`
}

type SizeInput struct {
	Label       string `json:"label" validate:"omitempty,max=20"`
	VolumeML    int    `json:"volume_ml" validate:"required,gt=0"`
	BottlePrice int64  `json:"bottle_price" validate:"gte=0"`
	WeightGram  int    `json:"weight_gram" validate:"gte=0"`
	IsDefault   bool   `json:"is_default"` // dijual untuk semua parfum
	IsActive    *bool  `json:"is_active"`
}

type SizePatch struct {
	Label       *string `json:"label" validate:"omitempty,max=20"`
	BottlePrice *int64  `json:"bottle_price" validate:"omitempty,gte=0"`
	WeightGram  *int    `json:"weight_gram" validate:"omitempty,gte=0"`
	IsDefault   *bool   `json:"is_default"`
	SortOrder   *int    `json:"sort_order"`
	IsActive    *bool   `json:"is_active"`
}

// InventoryAdjustment: mode "set" = stok fisik jadi Quantity (stok opname);
// mode "add" = tambah/kurangi Quantity (restock, retur, rusak).
type InventoryAdjustment struct {
	VariantID uint    `json:"variant_id" validate:"required"`
	Mode      string  `json:"mode" validate:"required,oneof=set add"`
	Quantity  int     `json:"quantity"`
	Reason    string  `json:"reason" validate:"omitempty,oneof=restock return adjustment opname"`
	Note      *string `json:"note"`
}

// AdminVariant = varian + harga hasil rumus + stok untuk layar admin
// (termasuk yang nonaktif / belum bisa dijual).
type AdminVariant struct {
	repository.Variant
	Name        string `json:"name"`
	ListPrice   *int64 `json:"list_price"`             // harga jual (rumus atau harga khusus); nil = belum bisa dihitung
	PricePerML  *int64 `json:"price_per_ml,omitempty"` // tarif yang dipakai rumus
	BottlePrice *int64 `json:"bottle_price,omitempty"`
	Sellable    bool   `json:"sellable"`  // tampil & bisa dibeli di website
	Available   int    `json:"available"` // stock − reserved (hanya berarti untuk produk track_stock)
}

// StockLevel: stok 1 varian setelah penyesuaian.
type StockLevel struct {
	VariantID uint `json:"variant_id"`
	Stock     int  `json:"stock"`
	Reserved  int  `json:"reserved"`
	Available int  `json:"available"`
}

type AdminProduct struct {
	repository.Product
	Variants []AdminVariant `json:"variants"`
}

func validationErr(err error) error { return fmt.Errorf("%w: %v", ErrValidation, err) }

func notFound(what string, id uint, err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("%s %d: %w", what, id, ErrNotFound)
	}
	return err
}

func checkNotes(primary, secondary *string) error {
	for _, n := range []*string{primary, secondary} {
		if n != nil && !slices.Contains(repository.ScentNotes, *n) {
			return fmt.Errorf("%w: label aroma %q tidak dikenal", ErrValidation, *n)
		}
	}
	return nil
}

// ---------------------------------------------------------------- produk

func AdminListProducts(status string) ([]repository.Product, error) {
	q := config.DB.Preload("Category").Preload("Scents", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order, id") })
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var out []repository.Product
	if err := q.Order("kind DESC, sort_order, id").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func AdminGetProduct(id uint) (*AdminProduct, error) {
	var p repository.Product
	err := config.DB.Preload("Category").
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order") }).
		Preload("Sizes").
		Preload("Scents", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order, id") }).
		Preload("Variants", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order, id") }).
		Preload("Variants.Scent").Preload("Variants.Size").
		First(&p, id).Error
	if err != nil {
		return nil, notFound("product", id, err)
	}
	out := &AdminProduct{Product: p, Variants: make([]AdminVariant, 0, len(p.Variants))}
	for _, v := range p.Variants {
		av := AdminVariant{Variant: v, Name: v.DisplayName(), Available: v.Available()}
		if price, ok := catalog.ListPrice(&v); ok {
			total := price.Total
			av.ListPrice, av.PricePerML, av.BottlePrice = &total, price.PerML, price.BottlePrice
		}
		v.Product = &p
		av.Sellable = catalog.Purchasable(&v)
		av.Variant.Product = nil
		out.Variants = append(out.Variants, av)
	}
	out.Product.Variants = nil
	return out, nil
}

func AdminCreateProduct(userID uint, in *ProductInput) (*AdminProduct, error) {
	if err := validate.Struct(in); err != nil {
		return nil, validationErr(err)
	}
	if err := checkNotes(in.NotePrimary, in.NoteSecondary); err != nil {
		return nil, err
	}
	code := in.Code
	if code == "" {
		code = slug.Make(in.Name)
	}
	if code == "" {
		return nil, fmt.Errorf("%w: nama harus mengandung huruf atau angka", ErrValidation)
	}
	status := in.Status
	if status == "" {
		status = repository.ProductStatusDraft
	}
	trackStock := in.Kind == repository.ProductKindGeneral // parfum diracik sendiri → selalu tersedia
	if in.TrackStock != nil {
		trackStock = *in.TrackStock
	}
	p := repository.Product{CategoryID: in.CategoryID, Kind: in.Kind, Code: code, Name: in.Name, Slug: code, Description: in.Description,
		WeightGram: in.WeightGram, TrackStock: trackStock, Status: status}
	if in.Kind == repository.ProductKindPerfume {
		if in.Grade == nil || in.NotePrimary == nil || in.NoteSecondary == nil {
			return nil, fmt.Errorf("%w: produk parfum wajib punya grade, note_primary, note_secondary", ErrValidation)
		}
		p.Grade, p.InspiredBy, p.NotePrimary, p.NoteSecondary = in.Grade, in.InspiredBy, in.NotePrimary, in.NoteSecondary
	}
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var n int64
		if err := tx.Model(&repository.Product{}).Where("code = ? OR slug = ?", code, code).Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			return fmt.Errorf("kode produk %q: %w", code, ErrConflict)
		}
		if err := tx.Create(&p).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return AdminGetProduct(p.ID)
}

func AdminUpdateProduct(userID, id uint, in *ProductPatch) (*AdminProduct, error) {
	if err := validate.Struct(in); err != nil {
		return nil, validationErr(err)
	}
	if err := checkNotes(in.NotePrimary, in.NoteSecondary); err != nil {
		return nil, err
	}
	upd := map[string]any{}
	set := func(col string, v any, ok bool) {
		if ok {
			upd[col] = v
		}
	}
	set("category_id", in.CategoryID, in.CategoryID != nil)
	set("name", in.Name, in.Name != nil)
	set("description", in.Description, in.Description != nil)
	set("grade", in.Grade, in.Grade != nil)
	set("inspired_by", in.InspiredBy, in.InspiredBy != nil)
	set("note_primary", in.NotePrimary, in.NotePrimary != nil)
	set("note_secondary", in.NoteSecondary, in.NoteSecondary != nil)
	set("weight_gram", in.WeightGram, in.WeightGram != nil)
	set("track_stock", in.TrackStock, in.TrackStock != nil)
	set("status", in.Status, in.Status != nil)
	set("sort_order", in.SortOrder, in.SortOrder != nil)
	if len(upd) == 0 {
		return AdminGetProduct(id)
	}
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var p repository.Product
		if err := tx.First(&p, id).Error; err != nil {
			return notFound("product", id, err)
		}
		if err := tx.Model(&p).Updates(upd).Error; err != nil {
			return fmt.Errorf("%w: %v", ErrValidation, err) // mis. CHECK constraint (label aroma sama)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return AdminGetProduct(id)
}

// ---------------------------------------------------------------- wangian

// AdminAddScent menambah wangian ke produk parfum dan langsung membuat semua
// kombinasi ukuran aktif × murni/campur (harga kosong, nonaktif).
func AdminAddScent(userID, productID uint, in *ScentInput) (*AdminProduct, error) {
	if err := validate.Struct(in); err != nil {
		return nil, validationErr(err)
	}
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var p repository.Product
		if err := tx.First(&p, productID).Error; err != nil {
			return notFound("product", productID, err)
		}
		if p.Kind != repository.ProductKindPerfume {
			return fmt.Errorf("%w: wangian hanya untuk produk parfum", ErrValidation)
		}
		code := in.Code
		if code == "" {
			code = p.Code + "-" + slug.Make(in.Name)
		}
		var n int64
		if err := tx.Model(&repository.Scent{}).Where("code = ? OR (product_id = ? AND lower(name) = lower(?))", code, p.ID, in.Name).Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			return fmt.Errorf("wangian %q: %w", in.Name, ErrConflict)
		}
		var maxOrder int
		tx.Model(&repository.Scent{}).Where("product_id = ?", p.ID).Select("COALESCE(MAX(sort_order), -1)").Scan(&maxOrder)
		s := repository.Scent{ProductID: p.ID, Code: code, Name: in.Name, SortOrder: maxOrder + 1, IsActive: true,
			PricePerMLMurni: in.PricePerMLMurni, PricePerMLCampur: in.PricePerMLCampur}
		if err := tx.Create(&s).Error; err != nil {
			return err
		}
		if _, err := ensureAllVariants(tx, p.ID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return AdminGetProduct(productID)
}

func AdminUpdateScent(userID, id uint, in *ScentPatch) (*repository.Scent, error) {
	if err := validate.Struct(in); err != nil {
		return nil, validationErr(err)
	}
	var s repository.Scent
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&s, id).Error; err != nil {
			return notFound("scent", id, err)
		}
		upd := map[string]any{}
		if in.Name != nil {
			upd["name"] = *in.Name
		}
		if in.IsActive != nil {
			upd["is_active"] = *in.IsActive
		}
		if in.SortOrder != nil {
			upd["sort_order"] = *in.SortOrder
		}
		if in.PricePerMLMurni != nil {
			upd["price_per_ml_murni"] = nilIfZero(*in.PricePerMLMurni)
		}
		if in.PricePerMLCampur != nil {
			upd["price_per_ml_campur"] = nilIfZero(*in.PricePerMLCampur)
		}
		if len(upd) == 0 {
			return nil
		}
		if err := tx.Model(&s).Updates(upd).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &s, config.DB.First(&s, id).Error
}

func nilIfZero(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

// AdminSetProductRates mengubah tarif per ml SEMUA wangian di 1 produk sekaligus.
func AdminSetProductRates(userID, productID uint, in *RatesInput) (*AdminProduct, error) {
	if err := validate.Struct(in); err != nil {
		return nil, validationErr(err)
	}
	upd := map[string]any{}
	if in.PricePerMLMurni != nil {
		upd["price_per_ml_murni"] = *in.PricePerMLMurni
	}
	if in.PricePerMLCampur != nil {
		upd["price_per_ml_campur"] = *in.PricePerMLCampur
	}
	if len(upd) == 0 {
		return nil, fmt.Errorf("%w: isi price_per_ml_murni dan/atau price_per_ml_campur", ErrValidation)
	}
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var p repository.Product
		if err := tx.First(&p, productID).Error; err != nil {
			return notFound("product", productID, err)
		}
		if p.Kind != repository.ProductKindPerfume {
			return fmt.Errorf("%w: tarif per ml hanya untuk produk parfum", ErrValidation)
		}
		if err := tx.Model(&repository.Scent{}).Where("product_id = ?", p.ID).Updates(upd).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return AdminGetProduct(productID)
}

// AdminSetProductSizes menyetel ukuran khusus produk (mis. hanya 10/15/30 ml) lalu
// membuat kombinasi yang belum ada. Daftar kosong = kembali ke ukuran default.
func AdminSetProductSizes(userID, productID uint, in *ProductSizesInput) (*AdminProduct, error) {
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var p repository.Product
		if err := tx.First(&p, productID).Error; err != nil {
			return notFound("product", productID, err)
		}
		if p.Kind != repository.ProductKindPerfume {
			return fmt.Errorf("%w: ukuran hanya untuk produk parfum", ErrValidation)
		}
		var n int64
		if len(in.SizeIDs) > 0 {
			if err := tx.Model(&repository.Size{}).Where("id IN ?", in.SizeIDs).Count(&n).Error; err != nil {
				return err
			}
			if int(n) != len(slices.Compact(slices.Sorted(slices.Values(in.SizeIDs)))) {
				return fmt.Errorf("%w: ada size_id yang tidak dikenal", ErrValidation)
			}
		}
		if err := tx.Where("product_id = ?", p.ID).Delete(&repository.ProductSize{}).Error; err != nil {
			return err
		}
		for _, id := range slices.Compact(slices.Sorted(slices.Values(in.SizeIDs))) {
			if err := tx.Create(&repository.ProductSize{ProductID: p.ID, SizeID: id}).Error; err != nil {
				return err
			}
		}
		if _, err := ensureAllVariants(tx, p.ID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return AdminGetProduct(productID)
}

// ---------------------------------------------------------------- foto

// ImageInput: url = link CDN lengkap (file di-upload ke storage/CDN, bukan ke backend).
type ImageInput struct {
	URL       string `json:"url" validate:"required,url,max=2048"`
	ScentID   *uint  `json:"scent_id"`   // opsional: foto khusus 1 wangian
	SortOrder *int16 `json:"sort_order"` // kosong = paling belakang; 0 = foto sampul
}

type ImagePatch struct {
	URL       *string `json:"url" validate:"omitempty,url,max=2048"`
	ScentID   *uint   `json:"scent_id"` // 0 = lepas dari wangian
	SortOrder *int16  `json:"sort_order"`
}

func checkImageScent(tx *gorm.DB, productID uint, scentID *uint) error {
	if scentID == nil || *scentID == 0 {
		return nil
	}
	var n int64
	if err := tx.Model(&repository.Scent{}).Where("id = ? AND product_id = ?", *scentID, productID).Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("%w: scent_id %d bukan milik produk ini", ErrValidation, *scentID)
	}
	return nil
}

func AdminAddImage(userID, productID uint, in *ImageInput) (*AdminProduct, error) {
	if err := validate.Struct(in); err != nil {
		return nil, validationErr(err)
	}
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var p repository.Product
		if err := tx.First(&p, productID).Error; err != nil {
			return notFound("product", productID, err)
		}
		if err := checkImageScent(tx, p.ID, in.ScentID); err != nil {
			return err
		}
		img := repository.ProductImage{ProductID: p.ID, ScentID: in.ScentID, URL: in.URL}
		if in.SortOrder != nil {
			img.SortOrder = *in.SortOrder
		} else {
			var maxOrder int16
			tx.Model(&repository.ProductImage{}).Where("product_id = ?", p.ID).Select("COALESCE(MAX(sort_order), -1)").Scan(&maxOrder)
			img.SortOrder = maxOrder + 1
		}
		if err := tx.Create(&img).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return AdminGetProduct(productID)
}

func AdminUpdateImage(userID, id uint, in *ImagePatch) (*AdminProduct, error) {
	if err := validate.Struct(in); err != nil {
		return nil, validationErr(err)
	}
	var img repository.ProductImage
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&img, id).Error; err != nil {
			return notFound("image", id, err)
		}
		upd := map[string]any{}
		if in.URL != nil {
			upd["url"] = *in.URL
		}
		if in.SortOrder != nil {
			upd["sort_order"] = *in.SortOrder
		}
		if in.ScentID != nil {
			if err := checkImageScent(tx, img.ProductID, in.ScentID); err != nil {
				return err
			}
			if *in.ScentID == 0 {
				upd["scent_id"] = nil
			} else {
				upd["scent_id"] = *in.ScentID
			}
		}
		if len(upd) == 0 {
			return nil
		}
		if err := tx.Model(&img).Updates(upd).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return AdminGetProduct(img.ProductID)
}

// AdminDeleteImage menghapus baris foto (file di CDN tidak ikut terhapus).
func AdminDeleteImage(userID, id uint) (*AdminProduct, error) {
	var img repository.ProductImage
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&img, id).Error; err != nil {
			return notFound("image", id, err)
		}
		if err := tx.Delete(&img).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return AdminGetProduct(img.ProductID)
}

// ---------------------------------------------------------------- varian

// AdminGenerateVariants = tombol "Buat kombinasi" (mis. setelah menambah ukuran 100 ml).
func AdminGenerateVariants(userID, productID uint) (int, error) {
	created := 0
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var p repository.Product
		if err := tx.First(&p, productID).Error; err != nil {
			return notFound("product", productID, err)
		}
		if p.Kind != repository.ProductKindPerfume {
			return fmt.Errorf("%w: kombinasi otomatis hanya untuk produk parfum", ErrValidation)
		}
		var err error
		if created, err = ensureAllVariants(tx, p.ID); err != nil {
			return err
		}
		return nil
	})
	return created, err
}

func ensureAllVariants(tx *gorm.DB, productID uint) (int, error) {
	sizes, err := catalog.SizesForProduct(tx, productID)
	if err != nil {
		return 0, err
	}
	var scents []repository.Scent
	if err := tx.Where("product_id = ? AND is_active", productID).Find(&scents).Error; err != nil {
		return 0, err
	}
	total := 0
	for _, s := range scents {
		n, err := catalog.EnsureScentVariants(tx, s, sizes, nil)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

// AdminAddGeneralVariant menambah pilihan ke produk umum ("Pakai foto").
func AdminAddGeneralVariant(userID, productID uint, in *GeneralVariantInput) (*AdminProduct, error) {
	if err := validate.Struct(in); err != nil {
		return nil, validationErr(err)
	}
	if in.IsActive && in.Price == nil {
		return nil, fmt.Errorf("%w: varian aktif wajib punya harga", ErrValidation)
	}
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var p repository.Product
		if err := tx.First(&p, productID).Error; err != nil {
			return notFound("product", productID, err)
		}
		if p.Kind != repository.ProductKindGeneral {
			return fmt.Errorf("%w: produk parfum memakai wangian × ukuran × tipe", ErrValidation)
		}
		labelSlug := ""
		if in.Label != nil {
			labelSlug = slug.Make(*in.Label)
		}
		v := repository.Variant{ProductID: p.ID, Label: in.Label, Code: repository.VariantCode(p.Code, "", 0, "", labelSlug),
			Price: in.Price, CompareAtPrice: in.CompareAtPrice, IsActive: in.IsActive}
		if err := tx.Create(&v).Error; err != nil {
			return fmt.Errorf("%w: %v", ErrConflict, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return AdminGetProduct(productID)
}

// AdminUpdateVariants mengubah harga/status banyak varian sekaligus (grid harga).
// Semua atau tidak sama sekali: kalau 1 baris salah, tidak ada yang tersimpan.
func AdminUpdateVariants(userID uint, rows []VariantPatch) (int, error) {
	if len(rows) == 0 {
		return 0, fmt.Errorf("%w: tidak ada varian", ErrValidation)
	}
	for i := range rows {
		if err := validate.Struct(&rows[i]); err != nil {
			return 0, validationErr(err)
		}
	}
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		for _, r := range rows {
			var v repository.Variant
			if err := tx.First(&v, r.ID).Error; err != nil {
				return notFound("variant", r.ID, err)
			}
			upd := map[string]any{}
			if r.Price != nil {
				var np *int64
				if *r.Price > 0 {
					np = r.Price
				}
				upd["price"] = np
				v.Price = np
			}
			if r.CompareAtPrice != nil {
				var nc *int64
				if *r.CompareAtPrice > 0 {
					nc = r.CompareAtPrice
				}
				upd["compare_at_price"] = nc
			}
			if r.IsActive != nil {
				upd["is_active"] = *r.IsActive
				v.IsActive = *r.IsActive
			}
			if r.WeightGram != nil {
				upd["weight_gram"] = *r.WeightGram
			}
			if len(upd) == 0 {
				continue
			}
			if v.IsActive && v.Price == nil && v.ScentID == nil {
				return fmt.Errorf("%w: varian %d (produk umum) aktif wajib punya harga", ErrValidation, r.ID)
			}
			if err := tx.Model(&repository.Variant{}).Where("id = ?", r.ID).Updates(upd).Error; err != nil {
				return fmt.Errorf("%w: varian %d: %v", ErrValidation, r.ID, err)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return len(rows), nil
}

// ---------------------------------------------------------------- ukuran

func AdminListSizes() ([]repository.Size, error) {
	var out []repository.Size
	return out, config.DB.Order("volume_ml").Find(&out).Error
}

func AdminCreateSize(userID uint, in *SizeInput) (*repository.Size, error) {
	if err := validate.Struct(in); err != nil {
		return nil, validationErr(err)
	}
	label := in.Label
	if label == "" {
		label = fmt.Sprintf("%d ml", in.VolumeML)
	}
	active := in.IsActive == nil || *in.IsActive
	s := repository.Size{Label: label, VolumeML: in.VolumeML, BottlePrice: in.BottlePrice, WeightGram: in.WeightGram,
		IsDefault: in.IsDefault, SortOrder: in.VolumeML, IsActive: active}
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var n int64
		if err := tx.Model(&repository.Size{}).Where("volume_ml = ?", in.VolumeML).Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			return fmt.Errorf("ukuran %d ml: %w", in.VolumeML, ErrConflict)
		}
		if err := tx.Create(&s).Error; err != nil {
			return err
		}
		return nil
	})
	return &s, err
}

func AdminUpdateSize(userID, id uint, in *SizePatch) (*repository.Size, error) {
	if err := validate.Struct(in); err != nil {
		return nil, validationErr(err)
	}
	var s repository.Size
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&s, id).Error; err != nil {
			return notFound("size", id, err)
		}
		upd := map[string]any{}
		if in.Label != nil {
			upd["label"] = *in.Label
		}
		if in.WeightGram != nil {
			upd["weight_gram"] = *in.WeightGram
		}
		if in.BottlePrice != nil {
			upd["bottle_price"] = *in.BottlePrice
		}
		if in.IsDefault != nil {
			upd["is_default"] = *in.IsDefault
		}
		if in.SortOrder != nil {
			upd["sort_order"] = *in.SortOrder
		}
		if in.IsActive != nil {
			upd["is_active"] = *in.IsActive
		}
		if len(upd) == 0 {
			return nil
		}
		if err := tx.Model(&s).Updates(upd).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &s, config.DB.First(&s, id).Error
}

// ---------------------------------------------------------------- stok

// AdminAdjustInventory menerapkan banyak penyesuaian stok sekaligus.
// Semua atau tidak sama sekali. Setiap perubahan dicatat di stock_movements.
func AdminAdjustInventory(userID uint, rows []InventoryAdjustment) ([]StockLevel, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("%w: tidak ada penyesuaian", ErrValidation)
	}
	for i := range rows {
		if err := validate.Struct(&rows[i]); err != nil {
			return nil, validationErr(err)
		}
		if rows[i].Mode == "add" && rows[i].Quantity == 0 {
			return nil, fmt.Errorf("%w: quantity tidak boleh 0 untuk mode add", ErrValidation)
		}
	}
	var out []StockLevel
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		uid := userID
		for _, r := range rows {
			// Kunci baris varian dulu supaya "add" dihitung dari stok terbaru.
			var v repository.Variant
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&v, r.VariantID).Error; err != nil {
				return notFound("variant", r.VariantID, err)
			}
			reason := r.Reason
			target := r.Quantity
			if r.Mode == "add" {
				target = v.Stock + r.Quantity
				if reason == "" {
					reason = repository.MovementRestock
					if r.Quantity < 0 {
						reason = repository.MovementAdjustment
					}
				}
			} else if reason == "" {
				reason = repository.MovementOpname
			}
			if target < 0 {
				return fmt.Errorf("%w: stok varian %d jadi negatif", ErrValidation, v.ID)
			}
			if _, err := catalog.SetStock(tx, v.ID, target, reason, &uid, r.Note); err != nil {
				return err
			}
			v.Stock = target
			out = append(out, StockLevel{VariantID: v.ID, Stock: v.Stock, Reserved: v.Reserved, Available: v.Available()})
		}
		return nil
	})
	return out, err
}

func AdminListMovements(variantID uint, limit int) ([]repository.StockMovement, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := config.DB.Order("id DESC").Limit(limit)
	if variantID != 0 {
		q = q.Where("variant_id = ?", variantID)
	}
	var out []repository.StockMovement
	return out, q.Find(&out).Error
}
