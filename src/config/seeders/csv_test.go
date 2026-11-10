package seeders

import (
	"fmt"
	"io"
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
	for in, want := range map[string]string{"D & G": "d-g", "1000 Bunga": "1000-bunga", "J-Lo": "j-lo", " Sulthan ": "sulthan"} {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestSeedFilesAreConsistent(t *testing.T) {
	cats := loadFile(t, "categories.csv", LoadCategories)
	sizes := loadFile(t, "sizes.csv", LoadSizes)
	scents := loadFile(t, "wangian.csv", LoadScents)
	variants := loadFile(t, "varian_parfum.csv", LoadPerfumeVariants)
	general := loadFile(t, "produk_umum.csv", LoadGeneralProducts)

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

	groups := map[string]map[string]bool{"regular": {}, "exclusive": {}}
	byCode := map[string]ScentRow{}
	for _, s := range scents {
		groups[s.Grade][s.GroupCode] = true
		byCode[s.Code] = s
		if !catNames[Slugify(s.Category)] {
			t.Fatalf("wangian %s: kategori %q tidak ada di categories.csv", s.Code, s.Category)
		}
		if s.Active && (s.RateMurni == nil || s.RateCampur == nil) {
			t.Fatalf("wangian aktif %s wajib punya tarif murni & campur", s.Code)
		}
	}
	// Regular 53 grup (81 wangian); Exclusive = 4 foto baru (11 wangian).
	if len(groups["regular"]) != 53 || len(groups["exclusive"]) != 4 || len(scents) != 92 {
		t.Fatalf("want 53+4 grup, 92 wangian; got %d+%d, %d", len(groups["regular"]), len(groups["exclusive"]), len(scents))
	}
	rate := func(code string) (int64, int64) {
		s, ok := byCode[code]
		if !ok || s.RateMurni == nil || s.RateCampur == nil {
			t.Fatalf("tarif %s tidak ada", code)
		}
		return *s.RateMurni, *s.RateCampur
	}
	for code, want := range map[string][2]int64{
		"victoria-romanwish":     {3000, 1500}, // Regular standar
		"bacarat":                {4000, 1500}, // Regular bibit 4.000
		"sulthan":                {4000, 1500},
		"exclusive-2-bacarat":    {5000, 3000}, // Exclusive ≠ Regular walau nama sama
		"exclusive-3-moonlight":  {4000, 2000}, // Moonlight beda tarif dari grupnya
		"exclusive-3-opus-black": {5000, 3000},
	} {
		if m, c := rate(code); m != want[0] || c != want[1] {
			t.Errorf("tarif %s = %d/%d; want %d/%d", code, m, c, want[0], want[1])
		}
	}
	if s := byCode["exclusive-1-scandal-secret"]; s.Name != "Scandal Secret" || s.Note1 != "sweet" || s.Note2 != "fresh" {
		t.Fatalf("Scandal Secret salah: %+v", s)
	}
	for _, code := range []string{"dunhil-desire-blue", "greentea", "guci-flora", "hugo-boss-boss-energi"} {
		if byCode[code].Active {
			t.Errorf("%s harus nonaktif (belum ada di daftar harga)", code)
		}
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
}

func TestLoadScentsRejectsBadData(t *testing.T) {
	const h = "grade,group_code,group,category,inspired_by,note1,note2,tarif_murni,tarif_campur,ukuran,aktif,code,varian,image\n"
	ok := "regular,annasui,Annasui,Parfum,Anna sui,sweet,fruity,3000,1500,,,annasui-dolly-girl,Dolly Girl,a.jpeg\n"
	cases := map[string]string{
		"grade salah":        "premium,a,A,K,,sweet,fruity,3000,1500,,,a,A,x\n",
		"note tidak dikenal": "regular,a,A,K,,sweet,spicy,3000,1500,,,a,A,x\n",
		"note sama":          "regular,a,A,K,,sweet,sweet,3000,1500,,,a,A,x\n",
		"kategori kosong":    "regular,a,A,,,sweet,fruity,3000,1500,,,a,A,x\n",
		"tarif desimal":      "regular,a,A,K,,sweet,fruity,3000.5,1500,,,a,A,x\n",
		"ukuran salah":       "regular,a,A,K,,sweet,fruity,3000,1500,sepuluh,,a,A,x\n",
		"aktif salah":        "regular,a,A,K,,sweet,fruity,3000,1500,,mungkin,a,A,x\n",
		"code tidak valid":   "regular,a,A,K,,sweet,fruity,3000,1500,,,A B,A,x\n",
		"code dobel":         ok + "regular,annasui,Annasui,Parfum,Anna sui,sweet,fruity,3000,1500,,,annasui-dolly-girl,Sweet Dream,a.jpeg\n",
		"varian dobel":       ok + "regular,annasui,Annasui,Parfum,Anna sui,sweet,fruity,3000,1500,,,annasui-x,dolly girl,a.jpeg\n",
		"data grup beda":     ok + "regular,annasui,Annasui,Parfum,Anna sui,sweet,woody,3000,1500,,,annasui-sweet-dream,Sweet Dream,a.jpeg\n",
		"ukuran grup beda":   ok + "regular,annasui,Annasui,Parfum,Anna sui,sweet,fruity,3000,1500,10;15,,annasui-sweet-dream,Sweet Dream,a.jpeg\n",
	}
	for name, body := range cases {
		if _, err := LoadScents(strings.NewReader(h + body)); err == nil {
			t.Errorf("%s: harus error", name)
		}
	}
	rows, err := LoadScents(strings.NewReader(h + "regular,mini,Mini,Parfum,,sweet,fruity,Rp3.000,1500,30;10 15,tidak,mini,Mini,x\n"))
	if err != nil {
		t.Fatal(err)
	}
	if r := rows[0]; *r.RateMurni != 3000 || *r.RateCampur != 1500 || fmt.Sprint(r.Sizes) != "[10 15 30]" || r.Active {
		t.Fatalf("parse salah: %+v", r)
	}
}

func TestLoadPerfumeVariants(t *testing.T) {
	const h = "product,varian,scent_code,ml,tipe,stok,harga_khusus,harga_coret,aktif\n"
	rows, err := LoadPerfumeVariants(strings.NewReader(h +
		"Annasui,Dolly Girl,annasui-dolly-girl,30,bibit,12,,,\n" +
		"Annasui,Dolly Girl,annasui-dolly-girl,30 ml,Parfum,,40000,50000,\n" +
		"Annasui,Dolly Girl,annasui-dolly-girl,50,murni,,,,tidak\n"))
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
