package seeders

import (
	"os"
	"strings"
	"testing"
)

func loadCategoriesFile(t *testing.T) []CategoryRow {
	t.Helper()
	f, err := os.Open("../../../seed/categories.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := LoadCategories(f)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

// Kategori harus sama persis dengan halaman toko Shopee (nama, urutan, ID).
func TestCategoriesCSVMatchesShopee(t *testing.T) {
	rows := loadCategoriesFile(t)
	want := []string{"Parfum & Wewangian", "Souvenir & Hadiah", "Kecantikan Lainnya", "Aksesoris Interior Mobil",
		"Perlengkapan Rumah Lainnya", "Alat Pemeliharaan Rumah", "Pengharum Ruangan & Aromaterapi"}
	if len(rows) != len(want) {
		t.Fatalf("want %d kategori, got %d", len(want), len(rows))
	}
	for i, w := range want {
		if rows[i].Name != w || rows[i].ShopeeCategoryID == nil {
			t.Fatalf("kategori %d: got %q (shopee id %v), want %q", i, rows[i].Name, rows[i].ShopeeCategoryID, w)
		}
	}
	if *rows[0].ShopeeCategoryID != 11043198 {
		t.Fatalf("shopee_category_id Parfum & Wewangian salah: %d", *rows[0].ShopeeCategoryID)
	}
}

func TestLoadCategoriesRejectsBadData(t *testing.T) {
	cases := map[string]string{
		"nama kosong":     ",1\n",
		"id bukan angka":  "A,abc\n",
		"slug dobel":      "A & B,1\nA B,2\n",
		"shopee id dobel": "A,1\nB,1\n",
	}
	for name, body := range cases {
		if _, err := LoadCategories(strings.NewReader("name,shopee_category_id\n" + body)); err == nil {
			t.Errorf("%s: harus error", name)
		}
	}
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

const header = "product_name,category,parent_sku,variation1,option1,variation2,option2,sku,price,original_price,stock,weight_gram,shopee_item_id,shopee_model_id\n"

func plan(t *testing.T, body string) ([]*productPlan, error) {
	t.Helper()
	rows, err := LoadCatalog(strings.NewReader(header + body))
	if err != nil {
		return nil, err
	}
	return planProducts(rows, testCats)
}

var testCats = []CategoryRow{{Name: "Parfum & Wewangian"}, {Name: "Kecantikan Lainnya"}}

func TestPlanProductsGroupsLikeShopee(t *testing.T) {
	plans, err := plan(t, `MAHERZEIN,Parfum & Wewangian,PR,Warna,Campur,Ukuran,15 ML,PR-C-15,Rp45.000,55000,10,100,111,1
MAHERZEIN,parfum & wewangian,,Warna,Campur,Ukuran,25 ML,PR-C-25,65000,,10,,,2
MAHERZEIN,Parfum & Wewangian,,Warna,Murni,Ukuran,15 ML,PR-M-15,45000,,0,,,3
# komentar dilewati
Botol spray,Kecantikan Lainnya,,,,,,BTL-1,5000,,50,,,
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 2 {
		t.Fatalf("want 2 produk, got %d", len(plans))
	}
	p := plans[0]
	if strings.Join(p.Opts1, "|") != "Campur|Murni" || strings.Join(p.Opts2, "|") != "15 ML|25 ML" {
		t.Fatalf("urutan pilihan salah: %v %v", p.Opts1, p.Opts2)
	}
	if p.ParentSKU != "PR" || p.WeightGram != 100 || p.ShopeeItemID == nil || *p.ShopeeItemID != 111 || len(p.Rows) != 3 {
		t.Fatalf("field produk salah: %+v", p)
	}
	if plans[1].Category != "Kecantikan Lainnya" || plans[1].Var1 != "" || len(plans[1].Rows) != 1 {
		t.Fatalf("botol spray harus listing biasa tanpa variasi: %+v", plans[1])
	}
	if p.Rows[0].Price != 45000 || *p.Rows[0].OriginalPrice != 55000 {
		t.Fatalf("harga salah: %+v", p.Rows[0])
	}
}

func TestPlanProductsRejectsBadData(t *testing.T) {
	cases := map[string]string{
		"kombinasi dobel":       "P,Parfum & Wewangian,,Aroma,Victoria,,,,45000,,1,,,\nP,Parfum & Wewangian,,Aroma,victoria,,,,50000,,1,,,\n",
		"sku dobel":             "P,Parfum & Wewangian,,Aroma,A,,,X,45000,,1,,,\nP,Parfum & Wewangian,,Aroma,B,,,X,45000,,1,,,\n",
		"nama variasi beda":     "P,Parfum & Wewangian,,Aroma,A,,,,45000,,1,,,\nP,Parfum & Wewangian,,Wangi,B,,,,45000,,1,,,\n",
		"harga kosong":          "P,Parfum & Wewangian,,Aroma,A,,,,,,1,,,\n",
		"harga coret < harga":   "P,Parfum & Wewangian,,Aroma,A,,,,45000,40000,1,,,\n",
		"tanpa variasi 2 baris": "P,Kecantikan Lainnya,,,,,,,1000,,1,,,\nP,Kecantikan Lainnya,,,,,,,2000,,1,,,\n",
		"variasi tanpa pilihan": "P,Parfum & Wewangian,,Aroma,,,,,45000,,1,,,\n",
		"kategori tidak ada":    "P,Parfum,,Aroma,A,,,,45000,,1,,,\n",
		"kategori kosong":       "P,,,Aroma,A,,,,45000,,1,,,\n",
		"kategori beda":         "P,Parfum & Wewangian,,Aroma,A,,,,45000,,1,,,\nP,Kecantikan Lainnya,,Aroma,B,,,,45000,,1,,,\n",
		"slug bentrok":          "D & G,Parfum & Wewangian,,Aroma,A,,,,45000,,1,,,\nD G,Parfum & Wewangian,,Aroma,A,,,,45000,,1,,,\n",
		"harga desimal":         "P,Parfum & Wewangian,,Aroma,A,,,,45000.00,,1,,,\n",
		"placeholder TODO":      "TODO judul,Parfum & Wewangian,,Aroma,A,,,,TODO,,0,,,\n",
	}
	for name, body := range cases {
		if _, err := plan(t, body); err == nil {
			t.Errorf("%s: harus error", name)
		}
	}
}

func TestCatalogCSVTemplate(t *testing.T) {
	b, err := os.ReadFile("../../../seed/catalog.csv")
	if err != nil {
		t.Fatal(err)
	}
	content := string(b)
	if strings.Contains(content, "TODO") {
		// Template belum diisi: seeder wajib menolak, supaya placeholder tidak masuk DB.
		if _, err := LoadCatalog(strings.NewReader(content)); err == nil || !strings.Contains(err.Error(), "placeholder") {
			t.Fatalf("template dengan TODO harus ditolak, got err=%v", err)
		}
		// Kalau TODO diganti nilai contoh, bentuknya harus 1 listing parfum × 50 aroma.
		content = strings.ReplaceAll(content, "TODO judul listing Shopee", "Parfum Contoh")
		content = strings.ReplaceAll(content, "TODO", "45000")
	}
	rows, err := LoadCatalog(strings.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	plans, err := planProducts(rows, loadCategoriesFile(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || plans[0].Category != "Parfum & Wewangian" || plans[0].Var1 != "Aroma" || plans[0].Var2 != "" {
		t.Fatalf("template harus 1 listing parfum dengan 1 variasi \"Aroma\", got %d listing", len(plans))
	}
	if strings.Contains(string(b), "TODO") && len(plans[0].Opts1) != 50 {
		t.Fatalf("template harus berisi 50 aroma, got %d", len(plans[0].Opts1))
	}
}
