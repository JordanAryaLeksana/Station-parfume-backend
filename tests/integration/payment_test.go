//go:build integration

package integration

import (
	"backend/src/config"
	"backend/src/modules/payment/midtrans"
	paymentservices "backend/src/modules/payment/services"
	"context"
	"fmt"
	"strings"
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
	ref := func(o uint) string { // order_id di Midtrans = payments.reference_id
		return scalar[string](t, "select reference_id::text from payments where order_id = ? order by id desc limit 1", o)
	}
	orderStatus := func(o uint) string { return scalar[string](t, "select status from orders where id = ?", o) }
	payStatus := func(o uint) string {
		return scalar[string](t, "select status from payments where order_id = ? order by id desc limit 1", o)
	}

	// ---------------- bayar & notifikasi
	o1 := checkout(map[uint]int{dg: 1, bs: 1})
	r = pay(o1)
	url1, _ := r.Data()["checkout_url"].(string)
	tok1, _ := r.Data()["snap_token"].(string)
	check(t, "bayar → checkout_url + snap_token Midtrans, status PENDING", r.Status == 200 && url1 != "" && tok1 != "" &&
		r.Data()["status"] == "PENDING" && r.Data()["provider"] == "midtrans" && num(r.Data()["amount"]) == 100000, r.Status, string(r.Raw))
	r = pay(o1)
	check(t, "bayar lagi → link & token yang sama (idempoten, 1 transaksi Snap)", r.Data()["checkout_url"] == url1 && r.Data()["snap_token"] == tok1 && fake.creates == 1, fake.creates)
	check(t, "botol spray di-reserve 1", stock("reserved") == 1)
	ref1 := ref(o1)
	check(t, "notifikasi dengan signature salah → 401", notify(t, ref1, "settlement", "SB-Mid-server-lain").Status == 401)
	r = call(t, "POST", "/api/v1/webhooks/midtrans", "", []byte(`{"order_id":"x"}`))
	check(t, "notifikasi tanpa field wajib → 400", r.Status == 400, r.Status)
	sig := midtrans.Signature("uji-dashboard-1", "200", "10000.00", serverKey)
	r = call(t, "POST", "/api/v1/webhooks/midtrans", "", []byte(`{"order_id":"uji-dashboard-1","status_code":"200","gross_amount":"10000.00",
		"transaction_status":"settlement","transaction_id":"tx-uji","signature_key":"`+sig+`"}`))
	check(t, "notifikasi uji dari Dashboard (order_id bukan milik toko) → 200, dicatat", r.Status == 200 &&
		scalar[int](t, "select count(*) from payment_events where provider_payment_id = 'tx-uji' and processed_at is not null and process_error is not null") == 1, r.Status)
	fake.choose(ref1)
	r = notify(t, ref1, "pending", serverKey)
	check(t, "notifikasi pending → 200, payment tetap PENDING", r.Status == 200 && payStatus(o1) == "PENDING", r.Status)
	r = notify(t, ref1, "settlement", serverKey)
	check(t, "notifikasi 'settlement' padahal di Midtrans masih pending → 503 (Midtrans kirim ulang)", r.Status == 503 && orderStatus(o1) == "PENDING_PAYMENT", r.Status)
	fake.complete(ref1, 0)
	r = notify(t, ref1, "settlement", serverKey)
	check(t, "notifikasi lunas → order & payment PAID", r.Status == 200 && r.Body["received"] == true && orderStatus(o1) == "PAID" && payStatus(o1) == "PAID", r.Status)
	check(t, "metode & bank + transaction_id tercatat", scalar[string](t, "select payment_method from payments where order_id = ?", o1) == "BANK_TRANSFER" &&
		scalar[string](t, "select payment_channel from payments where order_id = ?", o1) == "BCA" &&
		scalar[string](t, "select provider_payment_id from payments where order_id = ?", o1) == "tx-"+tok1)
	check(t, "stok terpotong: 5 → 4, reserved 0, 1 riwayat sale", stock("stock") == 4 && stock("reserved") == 0 &&
		scalar[int](t, "select count(*) from stock_movements where order_id = ?", o1) == 1)
	check(t, "notifikasi dobel → 200, tidak memotong stok lagi", notify(t, ref1, "settlement", serverKey).Status == 200 && stock("stock") == 4)
	r = call(t, "GET", fmt.Sprintf("/api/v1/orders/%d", o1), buyer, nil)
	check(t, "GET /orders/:id memuat status PAID + payments", r.Status == 200 && r.Data()["status"] == "PAID" && len(r.Data()["payments"].([]any)) == 1, r.Status)
	check(t, "bayar order milik orang lain → 404", call(t, "POST", fmt.Sprintf("/api/v1/orders/%d/pay", o1), admin, nil).Status == 404)
	r = pay(o1)
	check(t, "bayar order yang sudah lunas → ditolak", r.Status == 400 || r.Status == 409, r.Status)

	// ---------------- nominal tidak cocok
	o2 := checkout(map[uint]int{dg: 1})
	pay(o2)
	fake.complete(ref(o2), 1)
	notify(t, ref(o2), "settlement", serverKey)
	check(t, "nominal lunas ≠ total → payment FAILED, order tetap belum bayar", payStatus(o2) == "FAILED" && orderStatus(o2) == "PENDING_PAYMENT", payStatus(o2), orderStatus(o2))

	// ---------------- pembeli membatalkan (sudah memilih metode → di-expire di Midtrans)
	o3 := checkout(map[uint]int{bs: 1})
	pay(o3)
	ref3 := ref(o3)
	fake.choose(ref3)
	check(t, "reserve order 3", stock("reserved") == 1)
	r = call(t, "PUT", fmt.Sprintf("/api/v1/orders/%d/cancel", o3), buyer, map[string]any{"reason": "berubah pikiran"})
	check(t, "pembeli batal → transaksi Midtrans expire, payment CANCELLED, reserve dilepas", r.Status == 200 && fake.status(ref3) == "expire" &&
		payStatus(o3) == "CANCELLED" && orderStatus(o3) == "CANCELLED" && stock("reserved") == 0, r.Status, fake.status(ref3), payStatus(o3))

	// ---------------- kedaluwarsa + pembayaran telat
	o4 := checkout(map[uint]int{bs: 1})
	pay(o4)
	ref4 := ref(o4)
	fake.choose(ref4)
	o4b := checkout(map[uint]int{dg: 1}) // pembeli belum memilih metode bayar (status → 404)
	pay(o4b)
	o5 := checkout(map[uint]int{bs: 2})
	pay(o5)
	if err := config.DB.Exec("update orders set expires_at = now() - interval '1 minute' where id in (?, ?)", o4, o4b).Error; err != nil {
		t.Fatal(err)
	}
	n, err := paymentservices.ReconcileAndExpire(context.Background(), time.Now())
	check(t, "rekonsiliasi: order 4 & 4b EXPIRED (4 di-expire di Midtrans, 4b belum ada transaksi), reserve order 5 tetap 2",
		err == nil && n == 2 && orderStatus(o4) == "EXPIRED" && orderStatus(o4b) == "EXPIRED" && fake.status(ref4) == "expire" &&
			payStatus(o4) == "EXPIRED" && payStatus(o4b) == "EXPIRED" && stock("reserved") == 2,
		n, err, orderStatus(o4), orderStatus(o4b), payStatus(o4), payStatus(o4b), stock("reserved"))
	fake.complete(ref4, 0) // order_id yang di-expire masih bisa dibayar lewat halaman Snap yang terbuka
	notify(t, ref4, "settlement", serverKey)
	check(t, "pembayaran telat: EXPIRED → PAID, jejak cancelled_at tetap ada", orderStatus(o4) == "PAID" && payStatus(o4) == "PAID" &&
		scalar[int](t, "select count(*) from orders where id = ? and cancelled_at is not null and paid_at is not null", o4) == 1)
	check(t, "stok 4 → 3 tanpa menyentuh reserve order 5", stock("stock") == 3 && stock("reserved") == 2, stock("stock"), stock("reserved"))
	call(t, "PUT", fmt.Sprintf("/api/v1/orders/%d/cancel", o5), buyer, nil)
	check(t, "batal order 5 (belum pilih metode) → payment CANCELLED, reserve 0", stock("reserved") == 0 && payStatus(o5) == "CANCELLED", payStatus(o5))

	// ---------------- pembayaran belum dikonfigurasi
	o6 := checkout(map[uint]int{dg: 1})
	paymentservices.Configure(nil, paymentservices.Settings{})
	r = pay(o6)
	rw := notify(t, ref1, "settlement", serverKey)
	paymentservices.Configure(fake, paymentservices.Settings{ServerKey: serverKey, SuccessURL: frontendOrigin + "/bayar/sukses"})
	check(t, "pembayaran belum dikonfigurasi → bayar & notifikasi 503", r.Status == 503 && rw.Status == 503, r.Status, rw.Status)

	// ---------------- PUT /orders/:id/status oleh staf
	call(t, "POST", "/api/v1/users", owner, map[string]any{"name": "Gudang", "email": "gudang@test.local", "password": buyerPassword, "role": "warehouse"})
	gudang, _ := login(t, "/api/v1/auth/login/admin", "gudang@test.local", buyerPassword)
	o7 := checkout(map[uint]int{bs: 1})
	pay(o7)
	ref7 := ref(o7)
	fake.choose(ref7)
	setStatus := func(token string, o uint, status string) resp {
		return call(t, "PUT", fmt.Sprintf("/api/v1/orders/%d/status", o), token, map[string]any{"status": status, "reason": "uji"})
	}
	check(t, "staf set PAID manual → 400 (hanya dari Midtrans)", setStatus(owner, o7, "PAID").Status == 400 && orderStatus(o7) == "PENDING_PAYMENT")
	check(t, "staf set EXPIRED manual → 400", setStatus(owner, o7, "EXPIRED").Status == 400)
	check(t, "warehouse membatalkan → 403", setStatus(gudang, o7, "CANCELLED").Status == 403 && orderStatus(o7) == "PENDING_PAYMENT")
	r = setStatus(owner, o7, "CANCELLED")
	check(t, "owner membatalkan → transaksi Midtrans ikut expire, payment CANCELLED, reserve dilepas",
		r.Status == 200 && orderStatus(o7) == "CANCELLED" && fake.status(ref7) == "expire" && payStatus(o7) == "CANCELLED" && stock("reserved") == 0,
		r.Status, fake.status(ref7), payStatus(o7), stock("reserved"))
	check(t, "warehouse refund → 403", setStatus(gudang, o1, "REFUNDED").Status == 403)
	r = setStatus(owner, o1, "REFUNDED")
	check(t, "owner REFUNDED → pembayaran lunas ikut REFUNDED", r.Status == 200 && orderStatus(o1) == "REFUNDED" && payStatus(o1) == "REFUNDED",
		r.Status, orderStatus(o1), payStatus(o1))

	// ---------------- data internal Midtrans tidak terlihat pembeli
	r = call(t, "GET", fmt.Sprintf("/api/v1/orders/%d", o2), buyer, nil)
	raw := string(r.Raw)
	check(t, "response pembeli tanpa ID internal Midtrans & failure_reason", r.Status == 200 && payStatus(o2) == "FAILED" &&
		!strings.Contains(raw, "provider_session_id") && !strings.Contains(raw, "provider_payment") && !strings.Contains(raw, "failure_reason") &&
		!strings.Contains(raw, "provider_response"), raw)

	// ---------------- cari pesanan pakai nomor pesanan (halaman finish Midtrans ?order=SP…)
	orderNo := scalar[string](t, "select order_no from orders where id = ?", o1)
	r = call(t, "GET", "/api/v1/orders/"+orderNo, buyer, nil)
	check(t, "GET /orders/SP… → pesanan yang sama", r.Status == 200 && uint(num(r.Data()["id"])) == o1, r.Status)
	check(t, "nomor pesanan huruf kecil juga dikenali", call(t, "GET", "/api/v1/orders/"+strings.ToLower(orderNo), buyer, nil).Status == 200)
	check(t, "nomor pesanan milik orang lain → 404", call(t, "GET", "/api/v1/orders/"+orderNo, admin, nil).Status == 404)
	check(t, "nomor pesanan tidak ada → 404", call(t, "GET", "/api/v1/orders/SP000000NOTEXIST", buyer, nil).Status == 404)
}
