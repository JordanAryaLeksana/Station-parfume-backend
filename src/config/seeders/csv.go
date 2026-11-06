// Package seeders mengisi database dari file CSV di folder seed/ (root repo).
//
//   - aromas.csv  : master aroma (nama persis seperti label variasi di Shopee)
//   - bottles.csv : master botol
//   - catalog.csv : satu baris = satu model/SKU, bentuknya sama dengan export
//     "Ubah Massal" Seller Center (produk, variasi 1/2, SKU, harga, stok)
//
// Semua parsing & validasi terjadi sebelum database disentuh.
package seeders

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"backend/src/repository"
	"backend/src/utils/slug"
)

type AromaRow struct {
	Line        int
	Name        string
	Gender      string
	Grade       string
	Brand       string
	Description string
}

type BottleRow struct {
	Line        int
	Name        string
	BottleType  string
	VolumeML    int
	Description string
}

// CatalogRow = satu kombinasi variasi (model) yang bisa dibeli.
type CatalogRow struct {
	Line          int
	ProductName   string
	ProductType   string
	ParentSKU     string
	Variation1    string
	Option1       string
	Variation2    string
	Option2       string
	SKU           string
	Price         int64
	OriginalPrice *int64
	Stock         int
	WeightGram    int
	ShopeeItemID  *int64
	ShopeeModelID *int64
}

// readCSV membaca CSV ber-header dan mengembalikan baris sebagai map kolom→nilai.
// Baris kosong dan baris yang diawali '#' dilewati.
func readCSV(r io.Reader, required []string) ([]map[string]string, []int, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.Comment = '#'
	cr.TrimLeadingSpace = true

	header, err := cr.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("baca header: %w", err)
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")))] = i
	}
	for _, col := range required {
		if _, ok := idx[col]; !ok {
			return nil, nil, fmt.Errorf("kolom wajib %q tidak ada di header", col)
		}
	}

	var rows []map[string]string
	var lines []int
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		line, _ := cr.FieldPos(0)
		m := map[string]string{}
		empty := true
		for col, i := range idx {
			if i < len(rec) {
				v := strings.TrimSpace(rec[i])
				m[col] = v
				if v != "" {
					empty = false
				}
			}
		}
		if empty {
			continue
		}
		rows = append(rows, m)
		lines = append(lines, line)
	}
	return rows, lines, nil
}

func LoadAromas(r io.Reader) ([]AromaRow, error) {
	rows, lines, err := readCSV(r, []string{"name"})
	if err != nil {
		return nil, fmt.Errorf("aromas.csv: %w", err)
	}
	var out []AromaRow
	seen := map[string]int{}
	for i, m := range rows {
		a := AromaRow{Line: lines[i], Name: m["name"], Gender: strings.ToLower(m["gender"]), Grade: strings.ToLower(m["grade"]), Brand: m["brand"], Description: m["description"]}
		if a.Name == "" {
			return nil, fmt.Errorf("aromas.csv baris %d: name kosong", a.Line)
		}
		if a.Gender == "" {
			a.Gender = repository.GenderUnisex
		}
		if a.Grade == "" {
			a.Grade = repository.GradeRegular
		}
		if !oneOf(a.Gender, repository.GenderMen, repository.GenderWomen, repository.GenderUnisex) {
			return nil, fmt.Errorf("aromas.csv baris %d: gender %q harus men/women/unisex", a.Line, a.Gender)
		}
		if !oneOf(a.Grade, repository.GradeRegular, repository.GradeExclusive, repository.GradeSuperExclusive) {
			return nil, fmt.Errorf("aromas.csv baris %d: grade %q harus regular/exclusive/super_exclusive", a.Line, a.Grade)
		}
		slug := Slugify(a.Name)
		if prev, ok := seen[slug]; ok {
			return nil, fmt.Errorf("aromas.csv baris %d: %q bentrok dengan baris %d (slug %q)", a.Line, a.Name, prev, slug)
		}
		seen[slug] = a.Line
		out = append(out, a)
	}
	return out, nil
}

func LoadBottles(r io.Reader) ([]BottleRow, error) {
	rows, lines, err := readCSV(r, []string{"name", "bottle_type", "volume_ml"})
	if err != nil {
		return nil, fmt.Errorf("bottles.csv: %w", err)
	}
	var out []BottleRow
	seen := map[string]int{}
	for i, m := range rows {
		b := BottleRow{Line: lines[i], Name: m["name"], BottleType: strings.ToLower(m["bottle_type"]), Description: m["description"]}
		if b.Name == "" || b.BottleType == "" {
			return nil, fmt.Errorf("bottles.csv baris %d: name dan bottle_type wajib", b.Line)
		}
		v, err := strconv.Atoi(m["volume_ml"])
		if err != nil || v <= 0 {
			return nil, fmt.Errorf("bottles.csv baris %d: volume_ml %q harus angka > 0", b.Line, m["volume_ml"])
		}
		b.VolumeML = v
		slug := Slugify(b.Name)
		if prev, ok := seen[slug]; ok {
			return nil, fmt.Errorf("bottles.csv baris %d: %q bentrok dengan baris %d", b.Line, b.Name, prev)
		}
		seen[slug] = b.Line
		out = append(out, b)
	}
	return out, nil
}

var catalogCols = []string{"product_name", "product_type", "variation1", "option1", "variation2", "option2", "sku", "price", "stock"}

// allCatalogCols = semua kolom catalog.csv, urut seperti header (untuk cek placeholder).
var allCatalogCols = []string{"product_name", "product_type", "parent_sku", "variation1", "option1", "variation2", "option2",
	"sku", "price", "original_price", "stock", "weight_gram", "shopee_item_id", "shopee_model_id"}

func LoadCatalog(r io.Reader) ([]CatalogRow, error) {
	rows, lines, err := readCSV(r, catalogCols)
	if err != nil {
		return nil, fmt.Errorf("catalog.csv: %w", err)
	}
	var out []CatalogRow
	for i, m := range rows {
		c := CatalogRow{
			Line: lines[i], ProductName: m["product_name"], ProductType: strings.ToLower(m["product_type"]), ParentSKU: m["parent_sku"],
			Variation1: m["variation1"], Option1: m["option1"], Variation2: m["variation2"], Option2: m["option2"], SKU: m["sku"],
		}
		fail := func(format string, a ...any) error {
			return fmt.Errorf("catalog.csv baris %d: %s", c.Line, fmt.Sprintf(format, a...))
		}
		for _, col := range allCatalogCols {
			if strings.Contains(strings.ToUpper(m[col]), "TODO") {
				return nil, fail("kolom %s masih placeholder (%q): isi dengan data asli dari Shopee", col, m[col])
			}
		}
		if c.ProductName == "" {
			return nil, fail("product_name kosong")
		}
		if c.ProductType == "" {
			c.ProductType = repository.ProductTypePerfume
		}
		if !oneOf(c.ProductType, repository.ProductTypePerfume, repository.ProductTypeEmptyBottle, repository.ProductTypeOther) {
			return nil, fail("product_type %q harus perfume/empty_bottle/other", c.ProductType)
		}
		if (c.Variation1 == "") != (c.Option1 == "") || (c.Variation2 == "") != (c.Option2 == "") {
			return nil, fail("nama variasi dan pilihannya harus diisi berpasangan")
		}
		if c.Variation2 != "" && c.Variation1 == "" {
			return nil, fail("variation2 diisi tapi variation1 kosong")
		}
		if c.Price, err = parseRupiah(m["price"]); err != nil || c.Price <= 0 {
			return nil, fail("price %q harus rupiah > 0 (contoh 45000 atau Rp45.000)", m["price"])
		}
		if s := m["original_price"]; s != "" {
			op, err := parseRupiah(s)
			if err != nil || op < c.Price {
				return nil, fail("original_price %q harus rupiah >= price", s)
			}
			c.OriginalPrice = &op
		}
		if c.Stock, err = atoiDefault(m["stock"]); err != nil || c.Stock < 0 {
			return nil, fail("stock %q harus angka >= 0", m["stock"])
		}
		if c.WeightGram, err = atoiDefault(m["weight_gram"]); err != nil || c.WeightGram < 0 {
			return nil, fail("weight_gram %q harus angka >= 0", m["weight_gram"])
		}
		if c.ShopeeItemID, err = optInt64(m["shopee_item_id"]); err != nil {
			return nil, fail("shopee_item_id %q bukan angka", m["shopee_item_id"])
		}
		if c.ShopeeModelID, err = optInt64(m["shopee_model_id"]); err != nil {
			return nil, fail("shopee_model_id %q bukan angka", m["shopee_model_id"])
		}
		out = append(out, c)
	}
	return out, nil
}

var rupiahRe = regexp.MustCompile(`^(\d+|\d{1,3}(\.\d{3})+)$`)

// parseRupiah menerima "45000", "45.000", "Rp45.000", "Rp 45.000".
// Titik hanya boleh sebagai pemisah ribuan; "45000.00" atau "45,5" ditolak
// supaya tidak ada salah tafsir desimal.
func parseRupiah(s string) (int64, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(s, "Rp"), "rp"))
	if !rupiahRe.MatchString(s) {
		return 0, errors.New("format rupiah tidak valid")
	}
	return strconv.ParseInt(strings.ReplaceAll(s, ".", ""), 10, 64)
}

func atoiDefault(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.Atoi(strings.ReplaceAll(s, ".", ""))
}

func optInt64(s string) (*int64, error) {
	if s == "" {
		return nil, nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func oneOf(v string, opts ...string) bool {
	for _, o := range opts {
		if v == o {
			return true
		}
	}
	return false
}

// Slugify: "D & G" → "d-g", "1000 Bunga" → "1000-bunga".
func Slugify(s string) string { return slug.Make(s) }

// normLabel dipakai untuk mencocokkan label variasi dengan nama aroma/botol.
func normLabel(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}
