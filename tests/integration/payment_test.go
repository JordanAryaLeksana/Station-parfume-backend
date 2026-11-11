//go:build integration

package integration

import (
	"backend/src/config"
	paymentservices "backend/src/modules/payment/services"
	"context"
	"fmt"
	"testing"
	"time"
)

func TestPaymentsAndRoles(t *testing.T) {
	resetData(t)
	owner := ownerToken(t)

	// ---------------- peran di /users
	r := call(t, "POST", "/api/v1/users", owner, map[string]any{"name": "Admin Toko", "email": "admin@test.local", "password": ownerPassword, "role": "admin"})
	check(t, "owner membuat staf admin", r.Status == 201 && r.Data()["role"] == "admin", r.Status, string(r.Raw))
	admin, _ := login(t, "/api/v1/auth/login/admin", "admin@test.local", ownerPassword)
	check(t, "admin TIDAK bisa membuat owner", call(t, "POST", "/api/v1/users", admin, map[string]any{"name": "X", "email": "x@test.local", "password": buyerPassword, "role": "owner"}).Status == 403)
	check(t, "admin TIDAK bisa membuat staf", call(t, "POST", "/api/v1/users", admin, map[string]any{"name": "G", "email": "g@test.local", "password": buyerPassword, "role": "warehouse"}).Status == 403)
	ownerID := scalar[uint](t, "select id from users where email = ?", ownerEmail)
	check(t, "admin TIDAK bisa mengubah akun owner", call(t, "PUT", fmt.Sprintf("/api/v1/users/%d", ownerID), admin, map[string]any{"name": "Hack", "email": ownerEmail, "password": "baru-12345678"}).Status == 403)
	check(t, "admin bisa membuat pembeli", call(t, "POST", "/api/v1/users", admin, map[string]any{"name": "Pembeli", "email": "pembeli@test.local", "password": buyerPassword}).Status == 201)
	buyerID := scalar[uint](t, "select id from users where email = 'pembeli@test.local'")
	check(t, "admin TIDAK bisa menaikkan pembeli jadi admin", call(t, "PUT", fmt.Sprintf("/api/v1/users/%d", buyerID), admin, map[string]any{"name": "Pembeli", "email": "pembeli@test.local", "password": buyerPassword, "role": "admin"}).Status == 403)
	check(t, "daftar & detail user", len(call(t, "GET", "/api/v1/users", owner, nil).List()) == 3 && call(t, "GET", fmt.Sprintf("/api/v1/users/%d", buyerID), owner, nil).Status == 200)
	buyer, _ := login(t, "/api/v1/auth/login", "pembeli@test.local", buyerPassword)

	dg := scalar[uint](t, "select id from variants where code = 'annasui-dolly-girl-30-murni'")
	bs := scalar[uint](t, "select v.id from variants v join products p on p.id = v.product_id where p.code = 'botol-spray' order by v.id limit 1")
	call(t, "POST", "/api/v1/admin/inventory/adjustments", owner, map[string]any{"adjustments": []map[string]any{{"variant_id": bs, "mode": "set", "quantity": 5}}})
	stock := func(col string) int { return scalar[int](t, "select "+col+" from variants where id = ?", bs) }
	addr := map[string]any{"recipient_name": "Pembeli", "recipient_phone": "081234567890", "province": "Jawa Barat", "city": "Karawang",
		"district": "Telukjambe", "postal_code": "41361", "address": "Jl. Contoh 1"}
	checkout := func(items map[uint]int) uint {
		t.Helper()
		for v, q := range items {
			if r := call(t, "POST", "/api/v1/cart/items", buyer, map[string]any{"variant_id": v, "quantity": q}); r.Status != 200 {
				t.Fatalf("keranjang: %d %s", r.Status, r.Raw)
			}
		}
		r := call(t, "POST", "/api/v1/orders", buyer, addr)
		if r.Status != 201 {
			t.Fatalf("checkout: %d %s", r.Status, r.Raw)
		}
		return uint(num(r.Data()["id"]))
	}
	pay := func(o uint) resp { return call(t, "POST", fmt.Sprintf("/api/v1/orders/%d/pay", o), buyer, nil) }
	session := func(o uint) string {
		return scalar[string](t, "select provider_session_id from payments where order_id = ? order by id desc limit 1", o)
	}
	orderStatus := func(o uint) string { return scalar[string](t, "select status from orders where id = ?", o) }
	payStatus := func(o uint) string {
		return scalar[string](t, "select status from payments where order_id = ? order by id desc limit 1", o)
	}

	// ---------------- bayar & webhook
	o1 := checkout(map[uint]int{dg: 1, bs: 1})
	r = pay(o1)
	url1, _ := r.Data()["checkout_url"].(string)
	check(t, "bayar → checkout_url Xendit, status PENDING", r.Status == 200 && url1 != "" && r.Data()["status"] == "PENDING" && num(r.Data()["amount"]) == 100000, r.Status, string(r.Raw))
	r = pay(o1)
	check(t, "bayar lagi → link yang sama (idempoten, 1 sesi)", r.Data()["checkout_url"] == url1 && fake.creates == 1, fake.creates)
	check(t, "botol spray di-reserve 1", stock("reserved") == 1)
	sid1 := session(o1)
	check(t, "webhook token salah → 401", webhook(t, "payment_session.completed", sid1, "salah").Status == 401)
	r = webhook(t, "payment_session.completed", sid1, webhookToken)
	check(t, "webhook 'completed' padahal sesi masih ACTIVE → 503 (Xendit kirim ulang)", r.Status == 503 && orderStatus(o1) == "PENDING_PAYMENT", r.Status)
	fake.complete(sid1, 0)
	r = webhook(t, "payment_session.completed", sid1, webhookToken)
	check(t, "webhook lunas → order & payment PAID", r.Status == 200 && r.Body["received"] == true && orderStatus(o1) == "PAID" && payStatus(o1) == "PAID", r.Status)
	check(t, "stok terpotong: 5 → 4, reserved 0, 1 riwayat sale", stock("stock") == 4 && stock("reserved") == 0 &&
		scalar[int](t, "select count(*) from stock_movements where order_id = ?", o1) == 1)
	check(t, "webhook dobel → 200, tidak memotong stok lagi", webhook(t, "payment_session.completed", sid1, webhookToken).Status == 200 && stock("stock") == 4)
	r = call(t, "GET", fmt.Sprintf("/api/v1/orders/%d", o1), buyer, nil)
	check(t, "GET /orders/:id memuat status PAID + payments", r.Status == 200 && r.Data()["status"] == "PAID" && len(r.Data()["payments"].([]any)) == 1, r.Status)
	check(t, "bayar order milik orang lain → 404", call(t, "POST", fmt.Sprintf("/api/v1/orders/%d/pay", o1), admin, nil).Status == 404)
	r = pay(o1)
	check(t, "bayar order yang sudah lunas → ditolak", r.Status == 400 || r.Status == 409, r.Status)

	// ---------------- nominal tidak cocok
	o2 := checkout(map[uint]int{dg: 1})
	pay(o2)
	fake.complete(session(o2), 1)
	webhook(t, "payment_session.completed", session(o2), webhookToken)
	check(t, "nominal lunas ≠ total → payment FAILED, order tetap belum bayar", payStatus(o2) == "FAILED" && orderStatus(o2) == "PENDING_PAYMENT", payStatus(o2), orderStatus(o2))

	// ---------------- pembeli membatalkan
	o3 := checkout(map[uint]int{bs: 1})
	pay(o3)
	sid3 := session(o3)
	check(t, "reserve order 3", stock("reserved") == 1)
	r = call(t, "PUT", fmt.Sprintf("/api/v1/orders/%d/cancel", o3), buyer, map[string]any{"reason": "berubah pikiran"})
	check(t, "pembeli batal → sesi Xendit CANCELED, payment CANCELLED, reserve dilepas", r.Status == 200 && fake.status(sid3) == "CANCELED" &&
		payStatus(o3) == "CANCELLED" && orderStatus(o3) == "CANCELLED" && stock("reserved") == 0, r.Status, fake.status(sid3), payStatus(o3))

	// ---------------- kedaluwarsa + pembayaran telat
	o4 := checkout(map[uint]int{bs: 1})
	pay(o4)
	sid4 := session(o4)
	o5 := checkout(map[uint]int{bs: 2})
	pay(o5)
	if err := config.DB.Exec("update orders set expires_at = now() - interval '1 minute' where id = ?", o4).Error; err != nil {
		t.Fatal(err)
	}
	n, err := paymentservices.ReconcileAndExpire(context.Background(), time.Now())
	check(t, "rekonsiliasi: order 4 EXPIRED, sesinya dibatalkan, reserve order 5 tetap 2", err == nil && n == 1 && orderStatus(o4) == "EXPIRED" &&
		fake.status(sid4) == "CANCELED" && stock("reserved") == 2, n, err, orderStatus(o4), stock("reserved"))
	fake.complete(sid4, 0) // dibayar tepat saat batas waktu habis
	webhook(t, "payment_session.completed", sid4, webhookToken)
	check(t, "pembayaran telat: EXPIRED → PAID, jejak cancelled_at tetap ada", orderStatus(o4) == "PAID" &&
		scalar[int](t, "select count(*) from orders where id = ? and cancelled_at is not null and paid_at is not null", o4) == 1)
	check(t, "stok 4 → 3 tanpa menyentuh reserve order 5", stock("stock") == 3 && stock("reserved") == 2, stock("stock"), stock("reserved"))
	call(t, "PUT", fmt.Sprintf("/api/v1/orders/%d/cancel", o5), buyer, nil)
	check(t, "batal order 5 → reserve 0", stock("reserved") == 0)

	// ---------------- pembayaran belum dikonfigurasi
	o6 := checkout(map[uint]int{dg: 1})
	paymentservices.Configure(nil, paymentservices.Settings{})
	r = pay(o6)
	paymentservices.Configure(fake, paymentservices.Settings{WebhookToken: webhookToken,
		SuccessURL: frontendOrigin + "/bayar/sukses", CancelURL: frontendOrigin + "/bayar/batal"})
	check(t, "pembayaran belum dikonfigurasi → 503", r.Status == 503, r.Status)
}
