package seeders

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"
)

func loadFile[T any](t *testing.T, name string, parse func(io.Reader) ([]T, error)) []T {
	t.Helper()
	f, err := os.Open("../../../seed/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := parse(f)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestParseRupiah(t *testing.T) {
	ok := map[string]int64{"45000": 45000, "45.000": 45000, "Rp45.000": 45000, "Rp 1.250.000": 1250000}
	for in, want := range ok {
		got, err := parseRupiah(in)
		if err != nil || got != want {
			t.Errorf("parseRupiah(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"45000.00", "45,5", "4.50", "-1", "", "abc"} {
		if _, err := parseRupiah(in); err == nil {
			t.Errorf("parseRupiah(%q) harus error", in)
		}
	}
}

func TestSlugify(t *testing.T) {
	for in, want := range map[string]string{"Teh & Madu": "teh-madu", "1000 Bunga": "1000-bunga", "Al-Fatih": "al-fatih", " Sulthan ": "sulthan"} {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestSeedFilesAreConsistent(t *testing.T) {
	cats := loadFile(t, "categories.csv", LoadCategories)
	sizes := loadFile(t, "sizes.csv", LoadSizes)
	scents := loadFile(t, "wangian.csv", LoadScents)
	if _, err := os.Stat("../../../seed/" + InspiredByFile); err == nil {
		if err := ApplyInspiredBy(scents, loadFile(t, InspiredByFile, LoadInspiredBy)); err != nil {
			t.Fatal(err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	variants := loadFile(t, "varian_parfum.csv", LoadPerfumeVariants)
	general := loadFile(t, "produk_umum.csv", LoadGeneralProducts)
	if _, err := os.Stat("../../../seed/" + RenameFile); err == nil {
		if err := CheckRenames(loadFile(t, RenameFile, LoadRenames), scents, general); err != nil {
			t.Fatal(err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}

	catNames := map[string]bool{}
	for _, c := range cats {
		catNames[Slugify(c.Name)] = true
	}
	if len(cats) != 7 {
		t.Fatalf("want 7 kategori, got %d", len(cats))
	}
	// 20/30/50 ml default, botol Rp5.000 semua ukuran.
	if len(sizes) != 3 {
		t.Fatalf("want 3 ukuran, got %d", len(sizes))
	}
	for i, ml := range []int{20, 30, 50} {
		if sizes[i].VolumeML != ml || !sizes[i].IsDefault || sizes[i].BottlePrice != 5000 {
			t.Fatalf("ukuran %d salah: %+v", i, sizes[i])
		}
	}

	// 1 baris = 1 parfum = 1 produk: group_code (alamat halaman) tidak boleh dipakai 2 baris.
	perfumes := map[string]ScentRow{} // group_code → baris
	grades := map[string]int{}
	byCode := map[string]ScentRow{}
	for _, s := range scents {
		if prev, ok := perfumes[s.GroupCode]; ok {
			t.Fatalf("group_code %s dipakai baris %d dan %d (1 baris = 1 produk)", s.GroupCode, prev.Line, s.Line)
		}
		perfumes[s.GroupCode] = s
		grades[s.Grade]++
		byCode[s.Code] = s
		if !catNames[Slugify(s.Category)] {
			t.Fatalf("wangian %s: kategori %q tidak ada di categories.csv", s.Code, s.Category)
		}
		if s.Active && (s.RateMurni == nil || s.RateCampur == nil) {
			t.Fatalf("wangian aktif %s wajib punya tarif murni & campur", s.Code)
		}
	}
	if grades["regular"] != 81 || grades["exclusive"] != 11 || len(scents) != 92 {
		t.Fatalf("want 81 Regular + 11 Exclusive = 92 parfum; got %d + %d = %d", grades["regular"], grades["exclusive"], len(scents))
	}
	if p := perfumes["kasturi-kijang"]; p.Grade != "regular" || p.Group != "Kasturi Kijang" || p.Code != "kasturi-kasturi-kijang" {
		t.Errorf("kasturi-kijang salah: %+v", p)
	}
	rate := func(code string) (int64, int64) {
		s, ok := byCode[code]
		if !ok || s.RateMurni == nil || s.RateCampur == nil {
			t.Fatalf("tarif %s tidak ada", code)
		}
		return *s.RateMurni, *s.RateCampur
	}
	for code, want := range map[string][2]int64{
		"lavender": {3000, 1500},
		"sulthan":  {4000, 1500},
	} {
		if m, c := rate(code); m != want[0] || c != want[1] {
			t.Errorf("tarif %s = %d/%d; want %d/%d", code, m, c, want[0], want[1])
		}
	}
	if s := byCode["malaikat-subuh"]; s.Name != "Malaikat Subuh" || s.Note1 != "musk" || s.Note2 != "oud" {
		t.Fatalf("Malaikat Subuh salah: %+v", s)
	}
	inactive := 0
	for _, s := range scents {
		if !s.Active {
			inactive++
		}
	}
	if inactive != 8 || byCode["greentea"].Active {
		t.Errorf("want 8 parfum nonaktif (belum ada di daftar harga, termasuk greentea); got %d", inactive)
	}

	// varian_parfum.csv = pengecualian saja; template kosong (harga dari rumus, stok parfum tidak dihitung).
	if len(variants) != 0 {
		t.Fatalf("varian_parfum.csv harus kosong di template, got %d baris", len(variants))
	}

	products := map[string]bool{}
	for _, g := range general {
		products[g.ProductCode] = true
		if !catNames[Slugify(g.Category)] {
			t.Fatalf("produk_umum.csv baris %d: kategori %q tidak ada", g.Line, g.Category)
		}
	}
	if len(products) != 16 {
		t.Fatalf("want 16 produk umum, got %d", len(products))
	}
	// products.code dipakai bersama parfum dan produk umum, jadi tidak boleh sama.
	for code := range perfumes {
		if products[code] {
			t.Errorf("kode %s dipakai parfum (wangian.csv) dan produk umum (produk_umum.csv)", code)
		}
	}
}

func TestLoadScentsRejectsBadData(t *testing.T) {
	const h = "grade,group_code,group,category,note1,note2,tarif_murni,tarif_campur,ukuran,aktif,code,varian,image\n"
	ok := "regular,contoh,Contoh,Parfum,sweet,fruity,3000,1500,,,contoh-satu,Satu,a.jpeg\n"
	cases := map[string]string{
		"grade salah":        "premium,a,A,K,sweet,fruity,3000,1500,,,a,A,x\n",
		"note tidak dikenal": "regular,a,A,K,sweet,spicy,3000,1500,,,a,A,x\n",
		"note sama":          "regular,a,A,K,sweet,sweet,3000,1500,,,a,A,x\n",
		"kategori kosong":    "regular,a,A,,sweet,fruity,3000,1500,,,a,A,x\n",
		"tarif desimal":      "regular,a,A,K,sweet,fruity,3000.5,1500,,,a,A,x\n",
		"ukuran salah":       "regular,a,A,K,sweet,fruity,3000,1500,sepuluh,,a,A,x\n",
		"aktif salah":        "regular,a,A,K,sweet,fruity,3000,1500,,mungkin,a,A,x\n",
		"code tidak valid":   "regular,a,A,K,sweet,fruity,3000,1500,,,A B,A,x\n",
		"code dobel":         ok + "regular,contoh,Contoh,Parfum,sweet,fruity,3000,1500,,,contoh-satu,Dua,a.jpeg\n",
		"varian dobel":       ok + "regular,contoh,Contoh,Parfum,sweet,fruity,3000,1500,,,contoh-x,satu,a.jpeg\n",
		"data grup beda":     ok + "regular,contoh,Contoh,Parfum,sweet,woody,3000,1500,,,contoh-dua,Dua,a.jpeg\n",
		"ukuran grup beda":   ok + "regular,contoh,Contoh,Parfum,sweet,fruity,3000,1500,10;15,,contoh-dua,Dua,a.jpeg\n",
	}
	for name, body := range cases {
		if _, err := LoadScents(strings.NewReader(h + body)); err == nil {
			t.Errorf("%s: harus error", name)
		}
	}
	rows, err := LoadScents(strings.NewReader(h + "regular,mini,Mini,Parfum,sweet,fruity,Rp3.000,1500,30;10 15,tidak,mini,Mini,x\n"))
	if err != nil {
		t.Fatal(err)
	}
	if r := rows[0]; *r.RateMurni != 3000 || *r.RateCampur != 1500 || fmt.Sprint(r.Sizes) != "[10 15 30]" || r.Active {
		t.Fatalf("parse salah: %+v", r)
	}
}

func TestInspiredByStaysInternal(t *testing.T) {
	const h = "grade,group_code,group,category,note1,note2,tarif_murni,tarif_campur,ukuran,aktif,code,varian,image\n"
	const legacy = "grade,group_code,group,category,inspired_by,note1,note2,tarif_murni,tarif_campur,ukuran,aktif,code,varian,image\n"
	if _, err := LoadScents(strings.NewReader(legacy + "regular,a,A,K,,sweet,fruity,3000,1500,,,a,A,x\n")); err == nil ||
		!strings.Contains(err.Error(), InspiredByFile) {
		t.Fatalf("kolom inspired_by di wangian.csv harus ditolak: %v", err)
	}

	scents, err := LoadScents(strings.NewReader(h +
		"regular,alpha,Alpha,K,sweet,fruity,3000,1500,,,alpha,Alpha,x\n" +
		"regular,beta,Beta,K,sweet,fruity,3000,1500,,,beta,Beta,x\n"))
	if err != nil {
		t.Fatal(err)
	}
	brands, err := LoadInspiredBy(strings.NewReader("# internal\ncode,inspired_by\nalpha,Merek Uji\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyInspiredBy(scents, brands); err != nil {
		t.Fatal(err)
	}
	if scents[0].InspiredBy == nil || *scents[0].InspiredBy != "Merek Uji" || scents[1].InspiredBy != nil {
		t.Fatalf("inspired_by salah dipasang: %v, %v", scents[0].InspiredBy, scents[1].InspiredBy)
	}
	if err := ApplyInspiredBy(scents, []InspiredByRow{{Line: 2, Code: "gamma", InspiredBy: "X"}}); err == nil {
		t.Fatal("code yang tidak ada di wangian.csv harus error")
	}

	for name, body := range map[string]string{
		"code tidak valid": "A B,Merek\n",
		"kosong":           "alpha,\n",
		"dobel":            "alpha,Merek\nalpha,Lain\n",
		"terlalu panjang":  "alpha," + strings.Repeat("x", 256) + "\n",
	} {
		if _, err := LoadInspiredBy(strings.NewReader("code,inspired_by\n" + body)); err == nil {
			t.Errorf("%s: harus error", name)
		}
	}
}

func TestLoadScentsAudienceOccasions(t *testing.T) {
	const h = "grade,group_code,group,category,note1,note2,tarif_murni,tarif_campur,ukuran,aktif,code,varian,image,untuk,cocok_untuk,deskripsi\n"
	rows, err := LoadScents(strings.NewReader(h +
		"regular,alpha,Alpha,K,sweet,fruity,3000,1500,,,alpha,Alpha,x,Wanita,Ibadah; harian;harian,Lembut dan manis.\n" +
		"regular,beta,Beta,K,sweet,fruity,3000,1500,,,beta,Beta,x,,,\n"))
	if err != nil {
		t.Fatal(err)
	}
	a, b := rows[0], rows[1]
	if a.Audience == nil || *a.Audience != "wanita" || fmt.Sprint(a.Occasions) != "[harian ibadah]" || a.Description == nil || *a.Description != "Lembut dan manis." {
		t.Fatalf("parse salah: %v %v %v", a.Audience, a.Occasions, a.Description)
	}
	if b.Audience != nil || b.Occasions != nil || b.Description != nil {
		t.Fatalf("kolom kosong harus berarti tidak diubah: %+v", b)
	}
	for name, body := range map[string]string{
		"untuk tidak dikenal": "regular,a,A,K,sweet,fruity,3000,1500,,,a,A,x,anak,,\n",
		"cocok tidak dikenal": "regular,a,A,K,sweet,fruity,3000,1500,,,a,A,x,,kencan,\n",
		"grup beda untuk":     "regular,a,A,K,sweet,fruity,3000,1500,,,a,A,x,pria,,\nregular,a,A,K,sweet,fruity,3000,1500,,,a2,B,x,wanita,,\n",
	} {
		if _, err := LoadScents(strings.NewReader(h + body)); err == nil {
			t.Errorf("%s: harus error", name)
		}
	}
}

func TestRenames(t *testing.T) {
	rows, err := LoadRenames(strings.NewReader("# internal\njenis,code_lama,code_baru\nproduk,lama,baru\nwangian,lama,baru\nproduk,baru,akhir\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0].Kind != RenameProduct || rows[1].Kind != RenameScent {
		t.Fatalf("parse salah: %+v", rows)
	}
	for name, body := range map[string]string{
		"jenis salah":      "varian,a,b\n",
		"code sama":        "produk,a,a\n",
		"code tidak valid": "produk,A B,b\n",
		"lama dobel":       "produk,a,b\nproduk,a,c\n",
		"baru dobel":       "produk,a,c\nproduk,b,c\n",
	} {
		if _, err := LoadRenames(strings.NewReader("jenis,code_lama,code_baru\n" + body)); err == nil {
			t.Errorf("%s: harus error", name)
		}
	}

	scents := []ScentRow{{GroupCode: "akhir", Code: "baru"}}
	if err := CheckRenames(rows, scents, nil); err != nil {
		t.Fatalf("rantai lama → baru → akhir harus lolos: %v", err)
	}
	if err := CheckRenames(rows[:1], []ScentRow{{GroupCode: "lain", Code: "lain"}}, nil); err == nil {
		t.Fatal("code_baru yang tidak ada di CSV harus error")
	}
	if err := CheckRenames([]RenameRow{{Line: 2, Kind: RenameProduct, From: "x", To: "botol"}}, nil, []GeneralRow{{ProductCode: "botol"}}); err != nil {
		t.Fatalf("ganti nama produk umum harus lolos: %v", err)
	}
}

func TestLoadPerfumeVariants(t *testing.T) {
	const h = "product,varian,scent_code,ml,tipe,stok,harga_khusus,harga_coret,aktif\n"
	rows, err := LoadPerfumeVariants(strings.NewReader(h +
		"Contoh,Satu,contoh-satu,30,bibit,12,,,\n" +
		"Contoh,Satu,contoh-satu,30 ml,Parfum,,40000,50000,\n" +
		"Contoh,Satu,contoh-satu,50,murni,,,,tidak\n"))
	if err != nil {
		t.Fatal(err)
	}
	if r := rows[0]; r.Concentration != "murni" || *r.Stock != 12 || r.SpecialPrice != nil || r.Active != nil {
		t.Fatalf("baris 1 salah (bibit = murni): %+v", r)
	}
	if r := rows[1]; r.Concentration != "campur" || r.Stock != nil || *r.SpecialPrice != 40000 || *r.CompareAtPrice != 50000 || r.VolumeML != 30 {
		t.Fatalf("baris 2 salah (parfum = campur): %+v", r)
	}
	if r := rows[2]; r.Active == nil || *r.Active {
		t.Fatalf("aktif=tidak harus false: %+v", r)
	}
	bad := map[string]string{
		"tipe salah":      "P,V,a,30,extrait,1,,,\n",
		"ml salah":        "P,V,a,tiga puluh,murni,1,,,\n",
		"stok negatif":    "P,V,a,30,murni,-1,,,\n",
		"harga desimal":   "P,V,a,30,murni,,45000.00,,\n",
		"harga coret <=":  "P,V,a,30,murni,,50000,50000,\n",
		"kombinasi dobel": "P,V,a,30,murni,1,,,\nP,V,a,30,bibit,2,,,\n",
	}
	for name, body := range bad {
		if _, err := LoadPerfumeVariants(strings.NewReader(h + body)); err == nil {
			t.Errorf("%s: harus error", name)
		}
	}
}

func TestLoadGeneralProductsRejectsBadData(t *testing.T) {
	const h = "product_code,product_name,category,label,harga,stok,weight_gram,aktif\n"
	bad := map[string]string{
		"code tidak valid":        "Botol Spray,Botol,K,,5000,,,\n",
		"multi baris tanpa label": "b,Botol,K,,5000,,,\nb,Botol,K,Besar,7000,,,\n",
		"label dobel":             "b,Botol,K,Kecil,5000,,,\nb,Botol,K,kecil,7000,,,\n",
		"nama beda":               "b,Botol,K,Kecil,5000,,,\nb,Botol 2,K,Besar,7000,,,\n",
		"aktif tanpa harga":       "b,Botol,K,,,,,ya\n",
	}
	for name, body := range bad {
		if _, err := LoadGeneralProducts(strings.NewReader(h + body)); err == nil {
			t.Errorf("%s: harus error", name)
		}
	}
}
