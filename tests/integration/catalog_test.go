//go:build integration

package integration

import (
	"fmt"
	"strings"
	"testing"
)

func findVariant(list []any, pred func(map[string]any) bool) map[string]any {
	for _, v := range list {
		if m, ok := v.(map[string]any); ok && pred(m) {
			return m
		}
	}
	return nil
}

func adminVariant(t *testing.T, product map[string]any, volume int, conc string) map[string]any {
	t.Helper()
	v := findVariant(product["variants"].([]any), func(m map[string]any) bool {
		size, _ := m["size"].(map[string]any)
		return size != nil && int(num(size["volume_ml"])) == volume && m["concentration"] == conc
	})
	if v == nil {
		t.Fatalf("varian %d ml %s tidak ada: %s", volume, conc, j(product["variants"]))
	}
	return v
}

func TestCatalogAndCheckout(t *testing.T) {
	resetData(t)
	owner := ownerToken(t)

	// ---------------- katalog publik
	r := call(t, "GET", "/api/v1/categories", "", nil)
	check(t, "kategori publik", r.Status == 200 && len(r.List()) > 0, r.Status)
	catID := num(r.List()[0].(map[string]any)["id"])
	r = call(t, "GET", "/api/v1/sizes", "", nil)
	check(t, "ukuran default 20/30/50 ml, botol Rp5.000", r.Status == 200 && len(r.List()) == 3 &&
		num(r.List()[1].(map[string]any)["bottle_price"]) == 5000, j(r.List()))
	r = call(t, "GET", "/api/v1/products?kind=perfume&q=annasui", "", nil)
	card := findVariant(r.List(), func(m map[string]any) bool { return m["slug"] == "annasui" })
	check(t, "kartu Annasui: harga 20 ml campur s/d 50 ml murni (rumus tarif × ml + botol)", card != nil &&
		num(card["price_min"]) == 1500*20+5000 && num(card["price_max"]) == 3000*50+5000 && card["in_stock"] == true, j(card))
	check(t, "filter note tidak dikenal → 400", call(t, "GET", "/api/v1/products?note=tidak-ada", "", nil).Status == 400)
	r = call(t, "GET", "/api/v1/products/annasui", "", nil)
	dg := findVariant(r.Data()["variants"].([]any), func(m map[string]any) bool { return m["code"] == "annasui-dolly-girl-30-murni" })
	check(t, "detail Annasui: 12 varian, Dolly Girl 30 ml murni = Rp95.000, stok tidak dihitung",
		r.Status == 200 && len(r.Data()["variants"].([]any)) == 12 && dg != nil && num(dg["price"]) == 95000 &&
			dg["available"] == nil && dg["in_stock"] == true, r.Status)
	check(t, "produk tidak ada → 404", call(t, "GET", "/api/v1/products/tidak-ada", "", nil).Status == 404)

	// ---------------- admin: produk parfum baru
	check(t, "input produk tidak valid → 400", call(t, "POST", "/api/v1/admin/products", owner, map[string]any{"kind": "x", "category_id": catID, "name": "X"}).Status == 400)
	r = call(t, "POST", "/api/v1/admin/products", owner, map[string]any{"kind": "perfume", "category_id": catID, "name": "Uji Wangi",
		"grade": "regular", "note_primary": "fresh", "note_secondary": "woody"})
	pid := num(r.Data()["id"])
	check(t, "buat produk parfum → 201 draft, track_stock false", r.Status == 201 && r.Data()["status"] == "draft" && r.Data()["track_stock"] == false, r.Status, string(r.Raw))
	r = call(t, "POST", fmt.Sprintf("/api/v1/admin/products/%v/scents", pid), owner, map[string]any{"name": "Pagi", "price_per_ml_murni": 2000, "price_per_ml_campur": 1000})
	check(t, "tambah wangian → 6 kombinasi otomatis", r.Status == 201 && len(r.Data()["variants"].([]any)) == 6, r.Status)
	v30 := adminVariant(t, r.Data(), 30, "murni")
	check(t, "harga rumus terlihat admin (2.000 × 30 + 5.000), belum dijual karena draft",
		num(v30["list_price"]) == 65000 && num(v30["price_per_ml"]) == 2000 && num(v30["bottle_price"]) == 5000 && v30["sellable"] == false, j(v30))
	check(t, "produk draft tidak tampil publik", call(t, "GET", "/api/v1/products/uji-wangi", "", nil).Status == 404)
	r = call(t, "PATCH", fmt.Sprintf("/api/v1/admin/products/%v", pid), owner, map[string]any{"status": "active"})
	check(t, "aktifkan produk", r.Status == 200 && r.Data()["status"] == "active", r.Status)
	r = call(t, "GET", "/api/v1/products/uji-wangi", "", nil)
	check(t, "produk aktif tampil publik dengan 6 varian", r.Status == 200 && len(r.Data()["variants"].([]any)) == 6, r.Status)

	r = call(t, "PATCH", "/api/v1/admin/variants", owner, map[string]any{"variants": []map[string]any{{"id": v30["id"], "price": 60000}}})
	check(t, "harga khusus massal → updated 1", r.Status == 200 && num(r.Data()["updated"]) == 1, r.Status, string(r.Raw))
	r = call(t, "GET", "/api/v1/products/uji-wangi", "", nil)
	pv := findVariant(r.Data()["variants"].([]any), func(m map[string]any) bool { return num(m["id"]) == num(v30["id"]) })
	check(t, "harga khusus mengalahkan rumus", pv != nil && num(pv["price"]) == 60000, j(pv))

	r = call(t, "PATCH", fmt.Sprintf("/api/v1/admin/products/%v/rates", pid), owner, map[string]any{"price_per_ml_campur": 1200})
	check(t, "ubah tarif campur semua wangian (20 ml = 1.200 × 20 + 5.000)", r.Status == 200 && num(adminVariant(t, r.Data(), 20, "campur")["list_price"]) == 29000, r.Status)

	r = call(t, "POST", "/api/v1/admin/sizes", owner, map[string]any{"volume_ml": 10, "bottle_price": 4000})
	size10 := num(r.Data()["id"])
	check(t, "tambah ukuran 10 ml (bukan default)", r.Status == 201 && r.Data()["is_default"] == false && r.Data()["label"] == "10 ml", r.Status, string(r.Raw))
	r = call(t, "PATCH", fmt.Sprintf("/api/v1/admin/sizes/%v", size10), owner, map[string]any{"weight_gram": 40})
	check(t, "ubah ukuran", r.Status == 200 && num(r.Data()["weight_gram"]) == 40, r.Status)
	check(t, "daftar ukuran admin (4)", len(call(t, "GET", "/api/v1/admin/sizes", owner, nil).List()) == 4)
	r = call(t, "PUT", fmt.Sprintf("/api/v1/admin/products/%v/sizes", pid), owner, map[string]any{"size_ids": []any{size10}})
	check(t, "ukuran khusus produk = 10 ml saja; kombinasinya langsung dibuat", r.Status == 200 && len(r.Data()["variants"].([]any)) == 8, r.Status)
	r = call(t, "POST", fmt.Sprintf("/api/v1/admin/products/%v/variants/generate", pid), owner, nil)
	check(t, "buat kombinasi lagi → 0 (sudah lengkap, idempoten)", r.Status == 200 && num(r.Data()["created"]) == 0, r.Status, string(r.Raw))
	r = call(t, "GET", "/api/v1/products/uji-wangi", "", nil)
	v10 := findVariant(r.Data()["variants"].([]any), func(m map[string]any) bool {
		return strings.HasSuffix(m["code"].(string), "-10-murni")
	})
	check(t, "publik hanya ukuran 10 ml (2.000 × 10 + 4.000 = 24.000)", len(r.Data()["size_options"].([]any)) == 1 &&
		len(r.Data()["variants"].([]any)) == 2 && v10 != nil && num(v10["price"]) == 24000, j(r.Data()["variants"]))

	r = call(t, "POST", fmt.Sprintf("/api/v1/admin/products/%v/images", pid), owner, map[string]any{"url": "https://cdn.test/uji.jpg"})
	check(t, "tambah foto (link CDN)", r.Status == 201 && len(r.Data()["images"].([]any)) == 1, r.Status, string(r.Raw))
	img := num(r.Data()["images"].([]any)[0].(map[string]any)["id"])
	check(t, "ubah foto", call(t, "PATCH", fmt.Sprintf("/api/v1/admin/images/%v", img), owner, map[string]any{"sort_order": 0}).Status == 200)
	r = call(t, "DELETE", fmt.Sprintf("/api/v1/admin/images/%v", img), owner, nil)
	check(t, "hapus foto", r.Status == 200 && r.Data()["images"] == nil, r.Status)
	sid := scalar[uint](t, "select id from scents where product_id = ?", pid)
	r = call(t, "PATCH", fmt.Sprintf("/api/v1/admin/scents/%d", sid), owner, map[string]any{"is_active": false})
	check(t, "nonaktifkan wangian → produk hilang dari publik", r.Status == 200 && r.Data()["is_active"] == false &&
		call(t, "GET", "/api/v1/products/uji-wangi", "", nil).Status == 404, r.Status)
	check(t, "daftar produk admin termasuk semua status", len(call(t, "GET", "/api/v1/admin/products", owner, nil).List()) > 50)

	// ---------------- produk umum, keranjang, checkout
	bsID := scalar[uint](t, "select v.id from variants v join products p on p.id = v.product_id where p.code = 'botol-spray' order by v.id limit 1")
	bsProd := scalar[uint](t, "select id from products where code = 'botol-spray'")
	dgID := num(dg["id"])
	r = call(t, "GET", "/api/v1/products/botol-spray", "", nil)
	bs := r.Data()["variants"].([]any)[0].(map[string]any)
	check(t, "produk umum: stok dihitung (0), in_stock false", num(bs["available"]) == 0 && bs["in_stock"] == false, j(bs))

	buyer := registerBuyer(t, "pembeli@test.local")
	check(t, "checkout keranjang kosong → 400", call(t, "POST", "/api/v1/orders", buyer, map[string]any{"recipient_name": "A",
		"recipient_phone": "08", "province": "P", "city": "C", "district": "D", "postal_code": "1", "address": "J"}).Status == 400)
	r = call(t, "POST", "/api/v1/cart/items", buyer, map[string]any{"variant_id": dgID, "quantity": 50})
	items := r.Data()["items"].([]any)
	check(t, "parfum: tambah 50 pcs tanpa batas stok (stock null)", r.Status == 200 && items[0].(map[string]any)["stock"] == nil, r.Status)
	itemID := num(items[0].(map[string]any)["id"])
	check(t, "ubah jumlah item", call(t, "PUT", fmt.Sprintf("/api/v1/cart/items/%v", itemID), buyer, map[string]any{"quantity": 2}).Status == 200)
	check(t, "produk umum stok 0 ditolak", call(t, "POST", "/api/v1/cart/items", buyer, map[string]any{"variant_id": bsID, "quantity": 1}).Status == 400)
	r = call(t, "POST", "/api/v1/admin/inventory/adjustments", owner, map[string]any{"adjustments": []map[string]any{{"variant_id": bsID, "mode": "add", "quantity": 4}}})
	lvl := r.List()[0].(map[string]any)
	check(t, "restock +4", r.Status == 200 && num(lvl["stock"]) == 4 && num(lvl["available"]) == 4, r.Status, string(r.Raw))
	check(t, "tambah 3 botol spray", call(t, "POST", "/api/v1/cart/items", buyer, map[string]any{"variant_id": bsID, "quantity": 3}).Status == 200)
	r = call(t, "GET", "/api/v1/cart/", buyer, nil) // garis miring di akhir tetap dilayani
	check(t, "keranjang: 2 × 95.000 + 3 × 5.000 = 205.000", r.Status == 200 && num(r.Data()["total_price"]) == 205000, r.Status, string(r.Raw))
	addr := map[string]any{"recipient_name": "Pembeli", "recipient_phone": "081234567890", "province": "Jawa Barat", "city": "Karawang",
		"district": "Telukjambe", "postal_code": "41361", "address": "Jl. Contoh 1"}
	r = call(t, "POST", "/api/v1/orders", buyer, addr)
	b := string(r.Raw)
	check(t, "checkout → 201 snapshot (tarif 3.000, botol 5.000, track_stock)", r.Status == 201 && strings.Contains(b, `"unit_price":95000`) &&
		strings.Contains(b, `"price_per_ml":3000`) && strings.Contains(b, `"bottle_price":5000`) && strings.Contains(b, `"track_stock":false`) &&
		strings.Contains(b, `"track_stock":true`) && num(r.Data()["total"]) == 205000, r.Status, b)
	oid := num(r.Data()["id"])
	check(t, "hanya botol spray yang di-reserve (3)", scalar[int](t, "select reserved from variants where id = ?", bsID) == 3 &&
		scalar[int](t, "select stock + reserved from variants where id = ?", dgID) == 0)
	check(t, "keranjang kosong setelah checkout", len(call(t, "GET", "/api/v1/cart", buyer, nil).Data()["items"].([]any)) == 0)
	r = call(t, "PUT", fmt.Sprintf("/api/v1/orders/%v/status", oid), owner, map[string]any{"status": "PAID"})
	check(t, "staf tidak bisa menandai lunas manual → 400", r.Status == 400, r.Status)
	// lunas lewat Midtrans (palsu): bayar → transaksi lunas → notifikasi
	call(t, "POST", fmt.Sprintf("/api/v1/orders/%v/pay", oid), buyer, nil)
	payRef := scalar[string](t, "select reference_id::text from payments where order_id = ?", oid)
	fake.complete(payRef, 0)
	r = notify(t, payRef, "settlement", serverKey)
	check(t, "PAID: stok 4 → 1, reserved 0, 1 riwayat sale", r.Status == 200 && scalar[int](t, "select stock from variants where id = ?", bsID) == 1 &&
		scalar[int](t, "select reserved from variants where id = ?", bsID) == 0 &&
		scalar[int](t, "select count(*) from stock_movements where order_id = ?", oid) == 1, r.Status)
	check(t, "status tidak boleh lompat (PAID → COMPLETED) → 400", call(t, "PUT", fmt.Sprintf("/api/v1/orders/%v/status", oid), owner, map[string]any{"status": "COMPLETED"}).Status == 400)
	call(t, "PUT", fmt.Sprintf("/api/v1/orders/%v/status", oid), owner, map[string]any{"status": "PROCESSING"})
	call(t, "PUT", fmt.Sprintf("/api/v1/orders/%v/status", oid), owner, map[string]any{"status": "SHIPPED"})
	r = call(t, "GET", fmt.Sprintf("/api/v1/orders/%v", oid), buyer, nil)
	check(t, "pembeli melihat pesanan SHIPPED + shipped_at", r.Status == 200 && r.Data()["status"] == "SHIPPED" && r.Data()["shipped_at"] != nil, r.Status)
	check(t, "daftar pesanan pembeli", len(call(t, "GET", "/api/v1/orders", buyer, nil).List()) == 1)
	check(t, "pembeli tidak bisa mengubah status → 403", call(t, "PUT", fmt.Sprintf("/api/v1/orders/%v/status", oid), buyer, map[string]any{"status": "COMPLETED"}).Status == 403)

	r = call(t, "GET", fmt.Sprintf("/api/v1/admin/products/%d", bsProd), owner, nil)
	av := findVariant(r.Data()["variants"].([]any), func(m map[string]any) bool { return uint(num(m["id"])) == bsID })
	check(t, "admin: stock 1, reserved 0, available 1", av != nil && num(av["stock"]) == 1 && num(av["reserved"]) == 0 && num(av["available"]) == 1, j(av))
	r = call(t, "GET", fmt.Sprintf("/api/v1/admin/inventory/movements?variant_id=%d", bsID), owner, nil)
	check(t, "riwayat stok: restock +4 lalu sale −3", len(r.List()) == 2 && num(r.List()[0].(map[string]any)["delta"]) == -3, string(r.Raw))
	call(t, "PATCH", fmt.Sprintf("/api/v1/admin/products/%d", bsProd), owner, map[string]any{"track_stock": false})
	r = call(t, "GET", "/api/v1/products/botol-spray", "", nil)
	bs = r.Data()["variants"].([]any)[0].(map[string]any)
	check(t, "track_stock dimatikan → selalu tersedia", bs["available"] == nil && bs["in_stock"] == true, j(bs))
	r = call(t, "DELETE", fmt.Sprintf("/api/v1/cart/items/%v", itemID), buyer, nil)
	check(t, "hapus item yang sudah tidak ada → 404", r.Status == 404, r.Status)
	check(t, "kosongkan keranjang", call(t, "DELETE", "/api/v1/cart", buyer, nil).Status == 200)
}

func TestHTTPHardening(t *testing.T) {
	resetData(t)
	r := call(t, "GET", "/healthz", "", nil)
	check(t, "/healthz 200", r.Status == 200 && r.Body["status"] == "ok", r.Status)
	r = call(t, "GET", "/readyz", "", nil)
	check(t, "/readyz 200 (DB & Redis ok)", r.Status == 200 && r.Body["checks"].(map[string]any)["database"] == "ok", r.Status, string(r.Raw))
	r = call(t, "OPTIONS", "/api/v1/cart", "", nil, "Origin", frontendOrigin, "Access-Control-Request-Method", "GET", "Access-Control-Request-Headers", "authorization")
	check(t, "preflight CORS dari frontend → 204", r.Status == 204 && r.Header.Get("Access-Control-Allow-Origin") == frontendOrigin, r.Status, r.Header)
	r = call(t, "GET", "/api/v1/categories", "", nil, "Origin", "https://evil.test")
	check(t, "origin asing → 403 JSON", r.Status == 403 && r.Body["message"] != nil, r.Status)
	r = call(t, "GET", "/api/v1/categories", "", nil, "Origin", frontendOrigin)
	check(t, "GET lintas origin terdaftar → 200 + header CORS", r.Status == 200 && r.Header.Get("Access-Control-Allow-Origin") == frontendOrigin, r.Status)
	check(t, "header keamanan ada", r.Header.Get("X-Content-Type-Options") == "nosniff" && r.Header.Get("Cache-Control") == "no-store" && r.Header.Get("X-Request-ID") != "")
	check(t, "404 JSON", call(t, "GET", "/api/v1/tidak-ada", "", nil).Body["message"] != nil)
	check(t, "/docs (Swagger UI) aktif", call(t, "GET", "/docs", "", nil).Status == 200)
	check(t, "/docs/openapi.yaml", call(t, "GET", "/docs/openapi.yaml", "", nil).Status == 200)
}
