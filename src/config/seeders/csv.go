// Package seeders mengisi database dari file CSV di folder seed/ (root repo).
// Semua file bisa dibuka & diisi lewat spreadsheet (Excel/Google Sheets).
//
//   - categories.csv   : kategori toko
//   - sizes.csv        : ukuran botol parfum + harga botol (20/30/50 ml default)
//   - wangian.csv      : produk parfum (1 grup foto) + wangian + tarif per ml
//   - varian_parfum.csv: pengecualian per varian parfum (harga khusus, nonaktif); opsional
//   - produk_umum.csv  : produk non-parfum (botol spray, souvenir, Al Rehab, ...)
//
// Harga parfum TIDAK diketik per varian: tarif per ml × volume + harga botol.
//
// Semua parsing & validasi terjadi sebelum database disentuh.
package seeders

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"backend/src/repository"
	"backend/src/utils/slug"
)

type CategoryRow struct {
	Line int
	Name string
}

type SizeRow struct {
	Line        int
	Label       string
	VolumeML    int
	BottlePrice int64
	WeightGram  int
	IsDefault   bool
}

// ScentRow = satu varian wangian (satu bullet di foto produk). Baris dengan
// group_code sama = 1 produk parfum.
type ScentRow struct {
	Line       int
	Grade      string
	GroupCode  string // = products.code
	Group      string // = products.name
	Category   string
	InspiredBy string
	Note1      string
	Note2      string
	Code       string // = scents.code
	Name       string
	Image      string // nama file foto (belum di-upload)
	RateMurni  *int64 // tarif "bibit" Rp/ml (nil = belum ada → tidak dijual)
	RateCampur *int64 // tarif "parfum" Rp/ml
	Sizes      []int  // ukuran khusus produk (ml); kosong = ukuran default
	Active     bool   // false = produk disimpan sebagai draft (tidak tampil)
}

// PerfumeVariantRow = pengecualian 1 varian parfum: harga khusus, nonaktif, (stok kalau
// produknya track_stock).
type PerfumeVariantRow struct {
	Line           int
	ScentCode      string
	VolumeML       int
	Concentration  string
	SpecialPrice   *int64 // harga khusus, mengalahkan rumus (nil = tidak diubah)
	CompareAtPrice *int64
	Stock          *int  // nil = jangan ubah stok
	Active         *bool // nil = tidak diubah
}

// GeneralRow = 1 varian produk umum. Baris dengan product_code sama = 1 produk.
type GeneralRow struct {
	Line        int
	ProductCode string
	ProductName string
	Category    string
	Label       string // kosong = produk tanpa pilihan
	Price       *int64
	Stock       *int
	WeightGram  int
	Active      *bool
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

func LoadCategories(r io.Reader) ([]CategoryRow, error) {
	rows, lines, err := readCSV(r, []string{"name"})
	if err != nil {
		return nil, fmt.Errorf("categories.csv: %w", err)
	}
	var out []CategoryRow
	seen := map[string]int{}
	for i, m := range rows {
		c := CategoryRow{Line: lines[i], Name: m["name"]}
		if c.Name == "" {
			return nil, fmt.Errorf("categories.csv baris %d: name kosong", c.Line)
		}
		slug := Slugify(c.Name)
		if prev, ok := seen[slug]; ok {
			return nil, fmt.Errorf("categories.csv baris %d: %q bentrok dengan baris %d (slug %q)", c.Line, c.Name, prev, slug)
		}
		seen[slug] = c.Line
		out = append(out, c)
	}
	return out, nil
}

func LoadSizes(r io.Reader) ([]SizeRow, error) {
	rows, lines, err := readCSV(r, []string{"label", "volume_ml"})
	if err != nil {
		return nil, fmt.Errorf("sizes.csv: %w", err)
	}
	var out []SizeRow
	seen := map[int]int{}
	for i, m := range rows {
		s := SizeRow{Line: lines[i], Label: m["label"]}
		if s.VolumeML, err = strconv.Atoi(m["volume_ml"]); err != nil || s.VolumeML <= 0 {
			return nil, fmt.Errorf("sizes.csv baris %d: volume_ml %q harus angka > 0", s.Line, m["volume_ml"])
		}
		if s.WeightGram, err = atoiDefault(m["weight_gram"]); err != nil || s.WeightGram < 0 {
			return nil, fmt.Errorf("sizes.csv baris %d: weight_gram %q harus angka >= 0", s.Line, m["weight_gram"])
		}
		if strings.TrimSpace(m["bottle_price"]) != "" {
			if s.BottlePrice, err = parseRupiah(m["bottle_price"]); err != nil {
				return nil, fmt.Errorf("sizes.csv baris %d: bottle_price %q: %v", s.Line, m["bottle_price"], err)
			}
		}
		def, err := optBool(m["default"])
		if err != nil {
			return nil, fmt.Errorf("sizes.csv baris %d: default %q harus ya/tidak", s.Line, m["default"])
		}
		s.IsDefault = def != nil && *def
		if s.Label == "" {
			s.Label = fmt.Sprintf("%d ml", s.VolumeML)
		}
		if prev, ok := seen[s.VolumeML]; ok {
			return nil, fmt.Errorf("sizes.csv baris %d: %d ml sudah ada di baris %d", s.Line, s.VolumeML, prev)
		}
		seen[s.VolumeML] = s.Line
		out = append(out, s)
	}
	return out, nil
}

var codeRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func LoadScents(r io.Reader) ([]ScentRow, error) {
	rows, lines, err := readCSV(r, []string{"grade", "group_code", "group", "category", "note1", "note2", "code", "varian"})
	if err != nil {
		return nil, fmt.Errorf("wangian.csv: %w", err)
	}
	notes := map[string]bool{}
	for _, n := range repository.ScentNotes {
		notes[n] = true
	}
	var out []ScentRow
	codes := map[string]int{}
	names := map[string]int{}       // group_code + nama varian → baris
	groups := map[string]ScentRow{} // group_code → baris pertama grup itu
	for i, m := range rows {
		s := ScentRow{Line: lines[i], Grade: strings.ToLower(m["grade"]), GroupCode: m["group_code"], Group: m["group"],
			Category: m["category"], InspiredBy: m["inspired_by"], Note1: strings.ToLower(m["note1"]), Note2: strings.ToLower(m["note2"]),
			Code: m["code"], Name: m["varian"], Image: m["image"], Active: true}
		fail := func(format string, a ...any) error {
			return fmt.Errorf("wangian.csv baris %d: %s", s.Line, fmt.Sprintf(format, a...))
		}
		if s.RateMurni, err = optRupiah(m["tarif_murni"]); err != nil {
			return nil, fail("tarif_murni %q: %v", m["tarif_murni"], err)
		}
		if s.RateCampur, err = optRupiah(m["tarif_campur"]); err != nil {
			return nil, fail("tarif_campur %q: %v", m["tarif_campur"], err)
		}
		if s.Sizes, err = parseSizeList(m["ukuran"]); err != nil {
			return nil, fail("ukuran %q: %v", m["ukuran"], err)
		}
		active, err := optBool(m["aktif"])
		if err != nil {
			return nil, fail("aktif %q harus ya/tidak", m["aktif"])
		}
		if active != nil {
			s.Active = *active
		}
		if s.Grade != repository.GradeRegular && s.Grade != repository.GradeExclusive {
			return nil, fail("grade %q harus regular/exclusive", s.Grade)
		}
		if s.Group == "" || s.Name == "" || s.Category == "" {
			return nil, fail("group, category, dan varian wajib diisi")
		}
		if !codeRe.MatchString(s.GroupCode) || !codeRe.MatchString(s.Code) {
			return nil, fail("group_code/code harus huruf kecil, angka, dan '-' (got %q, %q)", s.GroupCode, s.Code)
		}
		if !notes[s.Note1] || !notes[s.Note2] || s.Note1 == s.Note2 {
			return nil, fail("note1/note2 (%q, %q) harus 2 label berbeda dari: %s", s.Note1, s.Note2, strings.Join(repository.ScentNotes, ", "))
		}
		if prev, ok := codes[s.Code]; ok {
			return nil, fail("code %q sudah dipakai di baris %d", s.Code, prev)
		}
		codes[s.Code] = s.Line
		key := s.GroupCode + "\x00" + normLabel(s.Name)
		if prev, ok := names[key]; ok {
			return nil, fail("varian %q dobel di grup %s (baris %d)", s.Name, s.GroupCode, prev)
		}
		names[key] = s.Line
		if g, ok := groups[s.GroupCode]; ok {
			if g.Grade != s.Grade || g.Group != s.Group || g.Category != s.Category || g.InspiredBy != s.InspiredBy ||
				g.Note1 != s.Note1 || g.Note2 != s.Note2 || g.Image != s.Image || g.Active != s.Active || !slices.Equal(g.Sizes, s.Sizes) {
				return nil, fail("data grup %s beda dengan baris %d (grade, group, category, inspired_by, note, image, ukuran, aktif harus sama)", s.GroupCode, g.Line)
			}
		} else {
			groups[s.GroupCode] = s
		}
		out = append(out, s)
	}
	return out, nil
}

// LoadPerfumeVariants membaca varian_parfum.csv. Kolom product/varian hanya untuk
// dibaca admin; kuncinya scent_code + ml + tipe. Kolom kosong = tidak diubah.
func LoadPerfumeVariants(r io.Reader) ([]PerfumeVariantRow, error) {
	rows, lines, err := readCSV(r, []string{"scent_code", "ml", "tipe"})
	if err != nil {
		return nil, fmt.Errorf("varian_parfum.csv: %w", err)
	}
	var out []PerfumeVariantRow
	seen := map[string]int{}
	for i, m := range rows {
		p := PerfumeVariantRow{Line: lines[i], ScentCode: m["scent_code"], Concentration: normConcentration(m["tipe"])}
		fail := func(format string, a ...any) error {
			return fmt.Errorf("varian_parfum.csv baris %d: %s", p.Line, fmt.Sprintf(format, a...))
		}
		if !codeRe.MatchString(p.ScentCode) {
			return nil, fail("scent_code %q tidak valid", p.ScentCode)
		}
		if p.VolumeML, err = parseML(m["ml"]); err != nil {
			return nil, fail("ml %q harus angka > 0", m["ml"])
		}
		if p.Concentration == "" {
			return nil, fail("tipe %q harus murni/bibit atau campur/parfum", m["tipe"])
		}
		if p.SpecialPrice, err = optRupiah(m["harga_khusus"]); err != nil {
			return nil, fail("harga_khusus %q: %v", m["harga_khusus"], err)
		}
		if p.CompareAtPrice, err = optRupiah(m["harga_coret"]); err != nil {
			return nil, fail("harga_coret %q: %v", m["harga_coret"], err)
		}
		if p.CompareAtPrice != nil && p.SpecialPrice != nil && *p.CompareAtPrice <= *p.SpecialPrice {
			return nil, fail("harga_coret harus lebih besar dari harga_khusus")
		}
		if p.Stock, err = optStock(m["stok"]); err != nil {
			return nil, fail("stok %q harus angka >= 0", m["stok"])
		}
		if p.Active, err = optBool(m["aktif"]); err != nil {
			return nil, fail("aktif %q harus ya/tidak", m["aktif"])
		}
		key := fmt.Sprintf("%s|%d|%s", p.ScentCode, p.VolumeML, p.Concentration)
		if prev, ok := seen[key]; ok {
			return nil, fail("kombinasi %s %d ml %s sudah ada di baris %d", p.ScentCode, p.VolumeML, p.Concentration, prev)
		}
		seen[key] = p.Line
		out = append(out, p)
	}
	return out, nil
}

// normConcentration: "murni"/"bibit" → murni, "campur"/"parfum" → campur, selain itu "".
func normConcentration(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "murni", "bibit":
		return repository.ConcentrationMurni
	case "campur", "parfum":
		return repository.ConcentrationCampur
	}
	return ""
}

func parseML(s string) (int, error) {
	v, err := strconv.Atoi(strings.TrimSuffix(strings.ToLower(strings.ReplaceAll(s, " ", "")), "ml"))
	if err != nil || v <= 0 {
		return 0, errors.New("harus angka > 0")
	}
	return v, nil
}

// parseSizeList: "10;15;30" atau "10 15 30" → [10 15 30] (urut, tanpa dobel). Kosong = nil.
func parseSizeList(s string) ([]int, error) {
	var out []int
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r == ';' || r == ' ' || r == '|' || r == '/' }) {
		v, err := parseML(f)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	slices.Sort(out)
	return out, nil
}

func LoadGeneralProducts(r io.Reader) ([]GeneralRow, error) {
	rows, lines, err := readCSV(r, []string{"product_code", "product_name", "category", "label", "harga"})
	if err != nil {
		return nil, fmt.Errorf("produk_umum.csv: %w", err)
	}
	var out []GeneralRow
	seen := map[string]int{}
	products := map[string]GeneralRow{}
	for i, m := range rows {
		g := GeneralRow{Line: lines[i], ProductCode: m["product_code"], ProductName: m["product_name"], Category: m["category"], Label: m["label"]}
		fail := func(format string, a ...any) error {
			return fmt.Errorf("produk_umum.csv baris %d: %s", g.Line, fmt.Sprintf(format, a...))
		}
		if !codeRe.MatchString(g.ProductCode) {
			return nil, fail("product_code %q harus huruf kecil, angka, dan '-'", g.ProductCode)
		}
		if g.ProductName == "" || g.Category == "" {
			return nil, fail("product_name dan category wajib diisi")
		}
		if g.Price, err = optRupiah(m["harga"]); err != nil {
			return nil, fail("harga %q: %v", m["harga"], err)
		}
		if g.Stock, err = optStock(m["stok"]); err != nil {
			return nil, fail("stok %q harus angka >= 0", m["stok"])
		}
		if g.WeightGram, err = atoiDefault(m["weight_gram"]); err != nil || g.WeightGram < 0 {
			return nil, fail("weight_gram %q harus angka >= 0", m["weight_gram"])
		}
		if g.Active, err = optBool(m["aktif"]); err != nil {
			return nil, fail("aktif %q harus ya/tidak", m["aktif"])
		}
		if g.Active != nil && *g.Active && g.Price == nil {
			return nil, fail("varian aktif wajib punya harga")
		}
		if p, ok := products[g.ProductCode]; ok {
			if p.ProductName != g.ProductName || p.Category != g.Category {
				return nil, fail("product_name/category beda dengan baris %d untuk product_code %s", p.Line, g.ProductCode)
			}
			if p.Label == "" || g.Label == "" {
				return nil, fail("produk dengan lebih dari 1 baris wajib mengisi label di semua baris")
			}
		} else {
			products[g.ProductCode] = g
		}
		key := g.ProductCode + "\x00" + normLabel(g.Label)
		if prev, ok := seen[key]; ok {
			return nil, fail("label %q dobel untuk %s (baris %d)", g.Label, g.ProductCode, prev)
		}
		seen[key] = g.Line
		out = append(out, g)
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

// optRupiah: kosong = nil; selain itu harus rupiah > 0.
func optRupiah(s string) (*int64, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	v, err := parseRupiah(s)
	if err != nil {
		return nil, err
	}
	if v <= 0 {
		return nil, errors.New("harus > 0")
	}
	return &v, nil
}

func optStock(s string) (*int, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	v, err := atoiDefault(s)
	if err != nil || v < 0 {
		return nil, errors.New("stok tidak valid")
	}
	return &v, nil
}

func optBool(s string) (*bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return nil, nil
	case "ya", "y", "true", "1", "aktif":
		t := true
		return &t, nil
	case "tidak", "t", "n", "false", "0", "nonaktif":
		f := false
		return &f, nil
	}
	return nil, errors.New("nilai tidak dikenal")
}

func atoiDefault(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.Atoi(strings.ReplaceAll(s, ".", ""))
}

// Slugify: "D & G" → "d-g", "1000 Bunga" → "1000-bunga".
func Slugify(s string) string { return slug.Make(s) }

// normLabel: perbandingan label tanpa beda huruf besar/kecil dan spasi ganda.
func normLabel(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}
