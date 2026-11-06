package seeders

import (
	"os"
	"strings"
	"testing"
)

func TestAromasCSVHas50UniqueAromas(t *testing.T) {
	f, err := os.Open("../../../seed/aromas.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := LoadAromas(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 50 {
		t.Fatalf("want 50 aroma, got %d", len(rows))
	}
	if rows[0].Name != "Victoria" || rows[49].Name != "So Sexy" || rows[34].Name != "Sulthan" {
		t.Fatalf("urutan/nama tidak sesuai: %q %q %q", rows[0].Name, rows[34].Name, rows[49].Name)
	}
	if rows[0].Gender != "unisex" || rows[0].Grade != "regular" {
		t.Fatalf("default gender/grade salah: %+v", rows[0])
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

const header = "product_name,product_type,parent_sku,variation1,option1,variation2,option2,sku,price,original_price,stock,weight_gram,shopee_item_id,shopee_model_id\n"

func plan(t *testing.T, body string) ([]*productPlan, error) {
	t.Helper()
	rows, err := LoadCatalog(strings.NewReader(header + body))
	if err != nil {
		return nil, err
	}
	return planProducts(rows)
}

func TestPlanProductsGroupsLikeShopee(t *testing.T) {
	plans, err := plan(t, `Parfum Refill,perfume,PR,Aroma,Victoria,Ukuran,30ml,PR-VIC-30,Rp45.000,55000,10,100,111,1
Parfum Refill,perfume,,Aroma,Victoria,Ukuran,50ml,PR-VIC-50,65000,,10,,,2
Parfum Refill,perfume,,Aroma,Bvlgari,Ukuran,30ml,PR-BVL-30,45000,,0,,,3
# komentar dilewati
Botol Kosong,empty_bottle,,,,,,BTL-1,8000,,50,,,
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 2 {
		t.Fatalf("want 2 produk, got %d", len(plans))
	}
	p := plans[0]
	if strings.Join(p.Opts1, "|") != "Victoria|Bvlgari" || strings.Join(p.Opts2, "|") != "30ml|50ml" {
		t.Fatalf("urutan pilihan salah: %v %v", p.Opts1, p.Opts2)
	}
	if p.ParentSKU != "PR" || p.WeightGram != 100 || p.ShopeeItemID == nil || *p.ShopeeItemID != 111 || len(p.Rows) != 3 {
		t.Fatalf("field produk salah: %+v", p)
	}
	if p.Rows[0].Price != 45000 || *p.Rows[0].OriginalPrice != 55000 {
		t.Fatalf("harga salah: %+v", p.Rows[0])
	}
}

func TestPlanProductsRejectsBadData(t *testing.T) {
	cases := map[string]string{
		"kombinasi dobel":       "P,perfume,,Aroma,Victoria,,,,45000,,1,,,\nP,perfume,,Aroma,victoria,,,,50000,,1,,,\n",
		"sku dobel":             "P,perfume,,Aroma,A,,,X,45000,,1,,,\nP,perfume,,Aroma,B,,,X,45000,,1,,,\n",
		"nama variasi beda":     "P,perfume,,Aroma,A,,,,45000,,1,,,\nP,perfume,,Wangi,B,,,,45000,,1,,,\n",
		"harga kosong":          "P,perfume,,Aroma,A,,,,,,1,,,\n",
		"harga coret < harga":   "P,perfume,,Aroma,A,,,,45000,40000,1,,,\n",
		"tanpa variasi 2 baris": "P,other,,,,,,,1000,,1,,,\nP,other,,,,,,,2000,,1,,,\n",
		"variasi tanpa pilihan": "P,perfume,,Aroma,,,,,45000,,1,,,\n",
		"product_type salah":    "P,parfum,,Aroma,A,,,,45000,,1,,,\n",
		"slug bentrok":          "D & G,perfume,,Aroma,A,,,,45000,,1,,,\nD G,perfume,,Aroma,A,,,,45000,,1,,,\n",
		"harga desimal":         "P,perfume,,Aroma,A,,,,45000.00,,1,,,\n",
		"placeholder TODO":      "TODO judul,perfume,,Aroma,A,,,,TODO,,0,,,\n",
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
	plans, err := planProducts(rows)
	if err != nil {
		t.Fatal(err)
	}
	perfumes := 0
	for _, p := range plans {
		if p.Type == "perfume" {
			perfumes++
			if p.Var1 != "Aroma" || p.Var2 != "" {
				t.Fatalf("listing parfum harus 1 variasi \"Aroma\" tanpa variasi 2, got %q/%q", p.Var1, p.Var2)
			}
		}
	}
	if perfumes != 1 {
		t.Fatalf("parfum harus disatukan dalam 1 listing, got %d", perfumes)
	}
	if strings.Contains(string(b), "TODO") && len(plans[0].Opts1) != 50 {
		t.Fatalf("template harus berisi 50 aroma, got %d", len(plans[0].Opts1))
	}
}
