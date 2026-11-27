//go:build integration

package integration

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"backend/migrations"
	"backend/src/config"
	"backend/src/config/seeders"
)

func scentRow(t *testing.T, data seeders.CatalogData, code string) *seeders.ScentRow {
	t.Helper()
	for i := range data.Scents {
		if data.Scents[i].GroupCode == code {
			return &data.Scents[i]
		}
	}
	t.Fatalf("wangian.csv tanpa %s", code)
	return nil
}

func strs(v any) []string {
	var out []string
	for _, x := range asList(v) {
		s, _ := x.(string)
		out = append(out, s)
	}
	return out
}

func asList(v any) []any { l, _ := v.([]any); return l }

func TestAudienceOccasions(t *testing.T) {
	resetData(t)
	owner := ownerToken(t)
	pid := scalar[uint](t, "select id from products where code = 'lavender'")
	path := fmt.Sprintf("/api/v1/admin/products/%d", pid)

	r := call(t, "PATCH", path, owner, map[string]any{"audience": "wanita", "occasions": []string{"Ibadah", "harian", "harian"}})
	check(t, "admin isi untuk + cocok untuk (dinormalkan, urut, tanpa duplikat)", r.Status == 200 && r.Data()["audience"] == "wanita" &&
		slices.Equal(strs(r.Data()["occasions"]), []string{"harian", "ibadah"}), r.Status, string(r.Raw))
	r = call(t, "GET", "/api/v1/products/lavender", "", nil)
	check(t, "detail publik menampilkan untuk + cocok untuk", r.Status == 200 && r.Data()["audience"] == "wanita" &&
		slices.Equal(strs(r.Data()["occasions"]), []string{"harian", "ibadah"}), string(r.Raw))
	r = call(t, "GET", "/api/v1/products?q=lavender", "", nil)
	card := findVariant(r.List(), func(m map[string]any) bool { return m["slug"] == "lavender" })
	check(t, "kartu publik menampilkan untuk + cocok untuk", card != nil && card["audience"] == "wanita" && len(strs(card["occasions"])) == 2, j(card))

	check(t, "untuk tidak dikenal → 400", call(t, "PATCH", path, owner, map[string]any{"audience": "anak"}).Status == 400)
	check(t, "cocok untuk tidak dikenal → 400", call(t, "PATCH", path, owner, map[string]any{"occasions": []string{"kencan"}}).Status == 400)
	r = call(t, "PATCH", path, owner, map[string]any{"audience": "", "occasions": []string{}})
	_, has := r.Data()["audience"]
	check(t, "string kosong mengosongkan untuk, [] mengosongkan cocok untuk", r.Status == 200 && !has && len(strs(r.Data()["occasions"])) == 0, string(r.Raw))
	r = call(t, "GET", "/api/v1/products/lavender", "", nil)
	_, hasAud := r.Data()["audience"]
	_, hasOcc := r.Data()["occasions"]
	check(t, "publik tanpa untuk/cocok untuk kalau kosong", !hasAud && !hasOcc, string(r.Raw))

	gid := scalar[uint](t, "select id from products where code = 'botol-spray'")
	gpath := fmt.Sprintf("/api/v1/admin/products/%d", gid)
	check(t, "produk umum tidak boleh punya untuk → 400", call(t, "PATCH", gpath, owner, map[string]any{"audience": "pria"}).Status == 400)
	check(t, "produk umum tidak boleh punya cocok untuk → 400", call(t, "PATCH", gpath, owner, map[string]any{"occasions": []string{"harian"}}).Status == 400)

	catID := scalar[uint](t, "select id from categories order by id limit 1")
	r = call(t, "POST", "/api/v1/admin/products", owner, map[string]any{"kind": "perfume", "category_id": catID, "name": "Uji Untuk",
		"grade": "regular", "note_primary": "fresh", "note_secondary": "woody", "audience": "unisex", "occasions": []string{"santai"}})
	check(t, "buat parfum dengan untuk + cocok untuk", r.Status == 201 && r.Data()["audience"] == "unisex" &&
		slices.Equal(strs(r.Data()["occasions"]), []string{"santai"}), r.Status, string(r.Raw))
	check(t, "buat produk umum dengan untuk → 400", call(t, "POST", "/api/v1/admin/products", owner, map[string]any{"kind": "general",
		"category_id": catID, "name": "Uji Umum", "audience": "pria"}).Status == 400)

	data := seedData(t)
	row := scentRow(t, data, "lavender")
	aud, desc := "unisex", "Deskripsi uji dari CSV."
	row.Audience, row.Occasions, row.Description = &aud, []string{"santai", "ibadah"}, &desc
	if _, err := seeders.SeedCatalog(config.DB, data, false); err != nil {
		t.Fatal(err)
	}
	r = call(t, "GET", "/api/v1/products/lavender", "", nil)
	check(t, "seed mengisi untuk, cocok untuk, deskripsi dari CSV", r.Data()["audience"] == "unisex" &&
		slices.Equal(strs(r.Data()["occasions"]), []string{"santai", "ibadah"}) && r.Data()["description"] == desc, string(r.Raw))
	if _, err := seeders.SeedCatalog(config.DB, seedData(t), false); err != nil {
		t.Fatal(err)
	}
	r = call(t, "GET", "/api/v1/products/lavender", "", nil)
	check(t, "kolom CSV kosong = tidak diubah", r.Data()["audience"] == "unisex" && r.Data()["description"] == desc, string(r.Raw))
}

func TestRenameInPlace(t *testing.T) {
	resetData(t)
	buyer := registerBuyer(t, "ganti-nama@test.local")
	pid := scalar[uint](t, "select id from products where code = 'lavender'")
	vid := scalar[uint](t, "select id from variants where code = 'lavender-30-murni'")
	gvid := scalar[uint](t, "select id from variants where code = 'botol-spray'")
	nVariants := scalar[int](t, "select count(*) from variants")
	check(t, "isi tas sebelum ganti nama", call(t, "POST", "/api/v1/cart/items", buyer, map[string]any{"variant_id": vid, "quantity": 2}).Status == 200)

	renamed := func() seeders.CatalogData {
		data := seedData(t)
		row := scentRow(t, data, "lavender")
		row.GroupCode, row.Group, row.Code, row.Name = "uji-ganti", "Uji Ganti", "uji-ganti", "Uji Ganti"
		for i := range data.General {
			if data.General[i].ProductCode == "botol-spray" {
				data.General[i].ProductCode, data.General[i].ProductName = "uji-botol", "Uji Botol"
			}
		}
		data.Renames = []seeders.RenameRow{
			{Line: 2, Kind: seeders.RenameProduct, From: "lavender", To: "uji-ganti"},
			{Line: 3, Kind: seeders.RenameScent, From: "lavender", To: "uji-ganti"},
			{Line: 4, Kind: seeders.RenameProduct, From: "botol-spray", To: "uji-botol"},
		}
		if err := seeders.CheckRenames(data.Renames, data.Scents, data.General); err != nil {
			t.Fatal(err)
		}
		return data
	}

	rep, err := seeders.SeedCatalog(config.DB, renamed(), false)
	if err != nil {
		t.Fatal(err)
	}
	check(t, "3 ganti nama, tanpa produk/varian baru, tanpa arsip", rep.Renamed == 3 && rep.Products.Created == 0 &&
		rep.Variants.Created == 0 && rep.Archived == 0, fmt.Sprintf("%+v", rep))
	check(t, "ID produk tetap, code/slug/nama baru", scalar[string](t, "select code || '|' || slug || '|' || name from products where id = ?", pid) == "uji-ganti|uji-ganti|Uji Ganti")
	check(t, "ID varian tetap, code varian ikut diganti", scalar[string](t, "select code from variants where id = ?", vid) == "uji-ganti-30-murni")
	check(t, "varian produk umum ikut diganti", scalar[string](t, "select code from variants where id = ?", gvid) == "uji-botol")
	check(t, "jumlah varian tidak bertambah", scalar[int](t, "select count(*) from variants") == nVariants)
	r := call(t, "GET", "/api/v1/products/uji-ganti", "", nil)
	check(t, "alamat baru jalan dengan ID yang sama", r.Status == 200 && num(r.Data()["id"]) == float64(pid), r.Status)
	check(t, "alamat lama → 404", call(t, "GET", "/api/v1/products/lavender", "", nil).Status == 404)
	r = call(t, "GET", "/api/v1/cart", buyer, nil)
	items := asList(r.Data()["items"])
	check(t, "isi tas tetap (varian sama, nama baru)", r.Status == 200 && len(items) == 1 &&
		num(items[0].(map[string]any)["variant_id"]) == float64(vid), string(r.Raw))

	rep, err = seeders.SeedCatalog(config.DB, renamed(), false)
	check(t, "seed ulang tidak mengubah apa-apa", err == nil && rep.Renamed == 0 && rep.Products.Created == 0 && rep.Archived == 0, err, fmt.Sprintf("%+v", rep))

	resetData(t)
	data := seedData(t)
	data.Renames = []seeders.RenameRow{{Line: 2, Kind: seeders.RenameProduct, From: "lavender", To: "cuddle"}}
	_, err = seeders.SeedCatalog(config.DB, data, false)
	check(t, "code baru yang sudah dipakai → error, semua di-rollback", err != nil &&
		scalar[int](t, "select count(*) from products where code = 'lavender'") == 1, err)
}

func TestMigration00004DownUp(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := config.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	p, err := migrations.NewProvider(sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	cols := func() int {
		return scalar[int](t, "select count(*) from information_schema.columns where table_name = 'products' and column_name in ('audience', 'occasions')")
	}
	check(t, "00004 terpasang: kolom audience + occasions ada", cols() == 2)
	if _, err := p.Down(ctx); err != nil {
		t.Fatal(err)
	}
	check(t, "down: kolom hilang", cols() == 0)
	if _, err := p.Up(ctx); err != nil {
		t.Fatal(err)
	}
	check(t, "up lagi: kolom kembali", cols() == 2)
	resetData(t)
}
