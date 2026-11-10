package catalog

import (
	"testing"

	"backend/src/repository"
)

func ptr[T any](v T) *T { return &v }

func TestApplyDiscount(t *testing.T) {
	cases := []struct {
		price   int64
		percent int
		want    int64
	}{{50000, 0, 50000}, {50000, 10, 45000}, {45999, 15, 39099}, {1, 50, 1}, {50000, 100, 1}}
	for _, c := range cases {
		if got := ApplyDiscount(c.price, c.percent); got != c.want {
			t.Errorf("ApplyDiscount(%d, %d) = %d; want %d", c.price, c.percent, got, c.want)
		}
	}
}

func perfume(rateMurni, rateCampur int64, ml int, bottle int64, conc string) *repository.Variant {
	return &repository.Variant{ID: 7, IsActive: true, Concentration: &conc,
		Product: &repository.Product{Status: repository.ProductStatusActive},
		Scent:   &repository.Scent{IsActive: true, PricePerMLMurni: &rateMurni, PricePerMLCampur: &rateCampur},
		Size:    &repository.Size{ID: 2, VolumeML: ml, BottlePrice: bottle, IsActive: true, IsDefault: true}}
}

func TestListPrice(t *testing.T) {
	// Victoria 30 ml: murni 3.000 × 30 + 5.000 = 95.000; campur 1.500 × 30 + 5.000 = 50.000.
	p, ok := ListPrice(perfume(3000, 1500, 30, 5000, "murni"))
	if !ok || p.Total != 95000 || *p.PerML != 3000 || *p.BottlePrice != 5000 {
		t.Fatalf("murni: %+v %v", p, ok)
	}
	if p, _ := ListPrice(perfume(3000, 1500, 30, 5000, "campur")); p.Total != 50000 {
		t.Fatalf("campur: %d", p.Total)
	}
	// Exclusive Moonlight 50 ml murni: 4.000 × 50 + 5.000.
	if p, _ := ListPrice(perfume(4000, 2000, 50, 5000, "murni")); p.Total != 205000 {
		t.Fatalf("moonlight: %d", p.Total)
	}
	// Harga khusus mengalahkan rumus.
	v := perfume(3000, 1500, 30, 5000, "murni")
	v.Price = ptr(int64(80000))
	if p, _ := ListPrice(v); p.Total != 80000 || p.PerML != nil {
		t.Fatalf("harga khusus: %+v", p)
	}
	// Tarif kosong → tidak bisa dihitung.
	v = perfume(3000, 1500, 30, 5000, "campur")
	v.Scent.PricePerMLCampur = nil
	if _, ok := ListPrice(v); ok {
		t.Fatal("tarif kosong harus ok=false")
	}
}

func TestUnitPrice(t *testing.T) {
	v := perfume(3000, 1500, 30, 5000, "murni") // 95.000
	if got := (&Pricing{}).UnitPrice(v); got != 95000 {
		t.Fatalf("pembeli biasa: %d", got)
	}
	level := &repository.ResellerLevel{ID: 1, Name: "Reseller", DiscountPercent: 10, IsActive: true}
	p := &Pricing{Level: level, overrides: map[uint]int64{}}
	if got := p.UnitPrice(v); got != 85500 {
		t.Fatalf("reseller 10%%: %d", got)
	}
	p.overrides[7] = 80000
	if got := p.UnitPrice(v); got != 80000 {
		t.Fatalf("harga khusus reseller harus menang: %d", got)
	}
	if got := p.UnitPrice(&repository.Variant{ID: 8}); got != 0 {
		t.Fatalf("varian tanpa harga harus 0: %d", got)
	}
}

func TestPurchasable(t *testing.T) {
	ok := perfume(3000, 1500, 30, 5000, "murni")
	if !Purchasable(ok) {
		t.Fatal("varian lengkap harus bisa dibeli")
	}
	cases := map[string]func(v repository.Variant) repository.Variant{
		"nonaktif": func(v repository.Variant) repository.Variant { v.IsActive = false; return v },
		"tanpa tarif": func(v repository.Variant) repository.Variant {
			s := *v.Scent
			s.PricePerMLMurni = nil
			v.Scent = &s
			return v
		},
		"produk draft": func(v repository.Variant) repository.Variant {
			v.Product = &repository.Product{Status: "draft"}
			return v
		},
		"wangian mati": func(v repository.Variant) repository.Variant {
			s := *v.Scent
			s.IsActive = false
			v.Scent = &s
			return v
		},
		"ukuran nonaktif": func(v repository.Variant) repository.Variant { z := *v.Size; z.IsActive = false; v.Size = &z; return v },
		"bukan default": func(v repository.Variant) repository.Variant {
			z := *v.Size
			z.IsDefault = false
			v.Size = &z
			return v
		},
		"di luar ukuran khusus": func(v repository.Variant) repository.Variant {
			v.Product = &repository.Product{Status: repository.ProductStatusActive, Sizes: []repository.Size{{ID: 99}}}
			return v
		},
	}
	for name, mod := range cases {
		v := mod(*ok)
		if Purchasable(&v) {
			t.Errorf("%s: tidak boleh bisa dibeli", name)
		}
	}
	// Ukuran khusus (mis. 10 ml, bukan default) boleh kalau tercantum di product_sizes.
	v := *ok
	z := *v.Size
	z.IsDefault = false
	v.Size = &z
	v.Product = &repository.Product{Status: repository.ProductStatusActive, Sizes: []repository.Size{{ID: 2}}}
	if !Purchasable(&v) {
		t.Fatal("ukuran khusus yang tercantum harus bisa dibeli")
	}
	// Produk umum: wajib harga tetap.
	g := &repository.Variant{IsActive: true, Product: &repository.Product{Status: repository.ProductStatusActive}}
	if Purchasable(g) {
		t.Fatal("produk umum tanpa harga tidak boleh bisa dibeli")
	}
	g.Price = ptr(int64(5000))
	if !Purchasable(g) {
		t.Fatal("produk umum berharga harus bisa dibeli")
	}
}

func TestStock(t *testing.T) {
	avail := map[uint]int{7: 2}
	perf := perfume(3000, 1500, 30, 5000, "murni") // Product.TrackStock = false (parfum diracik)
	if StockOf(perf, avail) != nil || !HasStock(perf, avail, 999) {
		t.Fatal("parfum tanpa track_stock harus selalu tersedia")
	}
	general := &repository.Variant{ID: 7, Product: &repository.Product{TrackStock: true}}
	if s := StockOf(general, avail); s == nil || *s != 2 {
		t.Fatalf("produk dengan track_stock: stok %v", s)
	}
	if !HasStock(general, avail, 2) || HasStock(general, avail, 3) {
		t.Fatal("HasStock harus membandingkan dengan stok tersedia")
	}
	if HasStock(&repository.Variant{ID: 8, Product: &repository.Product{TrackStock: true}}, avail, 1) {
		t.Fatal("tanpa baris stok = 0")
	}
}
