//go:build integration

package integration

import (
	"backend/src/config"
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

	dg := scalar[uint](t, "select id from variants where code = 'lavender-30-murni'")
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
	ref := func(o uint) string { // invoice di DOKU = payments.reference_id
		return scalar[string](t, "select reference_id from payments where order_id = ? order by id desc limit 1", o)
	}
	orderStatus := func(o uint) string { return scalar[string](t, "select status from orders where id = ?", o) }
	payStatus := func(o uint) string {
		return scalar[string](t, "select status from payments where order_id = ? order by id desc limit 1", o)
	}

	// ---------------- bayar & notifikasi
	o1 := checkout(map[uint]int{dg: 1, bs: 1})
	r = pay(o1)
	url1, _ := r.Data()["checkout_url"].(string)
	orderNo1 := scalar[string](t, "select order_no from orders where id = ?", o1)
	check(t, "bayar → checkout_url DOKU, invoice = nomor pesanan + P01, status PENDING", r.Status == 200 && url1 != "" &&
		r.Data()["reference_id"] == orderNo1+"P01" && r.Data()["status"] == "PENDING" && r.Data()["provider"] == "doku" &&
		num(r.Data()["amount"]) == 100000 && r.Data()["snap_token"] == nil, r.Status, string(r.Raw))
	req := fake.lastReq
	check(t, "request ke DOKU: invoice, nominal, batas bayar, callback ?order=, data pembeli", req.Order.InvoiceNumber == orderNo1+"P01" &&
		req.Order.Amount == 100000 && req.Payment.PaymentDueDate > 0 && strings.Contains(req.Order.CallbackURL, "order="+orderNo1) &&
		req.Customer != nil && req.Customer.Email == "pembeli@test.local" && len(req.Order.LineItems) == 2, j(req))
	r = pay(o1)
	check(t, "bayar lagi → link yang sama (idempoten, 1 halaman Checkout)", r.Data()["checkout_url"] == url1 && fake.creates == 1, fake.creates)
	check(t, "botol spray di-reserve 1", stock("reserved") == 1)
	ref1 := ref(o1)
	check(t, "notifikasi dengan signature salah → 401", notify(t, ref1, "SUCCESS", "secret-lain").Status == 401)
	check(t, "notifikasi tanpa header signature → 401", call(t, "POST", "/api/v1/webhooks/doku", "", []byte(`{"order":{"invoice_number":"x"}}`)).Status == 401)
	r = notifyBody(t, []byte(`{"order":{"amount":1}}`), dokuSecret)
	check(t, "notifikasi tanpa invoice → 400", r.Status == 400, r.Status)
	r = notifyBody(t, []byte(`{"order":{"invoice_number":"UJIDASHBOARD1","amount":10000},"transaction":{"status":"SUCCESS"}}`), dokuSecret)
	check(t, "notifikasi uji dari dashboard (invoice bukan milik toko) → 200, dicatat", r.Status == 200 &&
		scalar[int](t, "select count(*) from payment_events where provider_payment_id = 'UJIDASHBOARD1' and processed_at is not null and process_error is not null") == 1, r.Status)
	fake.choose(ref1, "VIRTUAL_ACCOUNT", "VIRTUAL_ACCOUNT_BRI")
	r = notify(t, ref1, "PENDING", dokuSecret)
	check(t, "notifikasi PENDING → 200, payment tetap PENDING", r.Status == 200 && payStatus(o1) == "PENDING", r.Status)
	r = notify(t, ref1, "FAILED", dokuSecret)
	check(t, "notifikasi FAILED → 200, payment tetap PENDING (pembeli bisa coba lagi)", r.Status == 200 && payStatus(o1) == "PENDING", r.Status)
	r = notify(t, ref1, "SUCCESS", dokuSecret)
	check(t, "notifikasi SUCCESS padahal di DOKU masih PENDING → 503 (DOKU kirim ulang)", r.Status == 503 && orderStatus(o1) == "PENDING_PAYMENT", r.Status)
	fake.complete(ref1, 0)
	r = notify(t, ref1, "SUCCESS", dokuSecret)
	check(t, "notifikasi lunas → order & payment PAID", r.Status == 200 && r.Body["received"] == true && orderStatus(o1) == "PAID" && payStatus(o1) == "PAID", r.Status)
	check(t, "metode & channel + token/Request-Id DOKU tercatat", scalar[string](t, "select payment_method from payments where order_id = ?", o1) == "VIRTUAL_ACCOUNT" &&
		scalar[string](t, "select payment_channel from payments where order_id = ?", o1) == "VIRTUAL_ACCOUNT_BRI" &&
		scalar[string](t, "select provider_session_id from payments where order_id = ?", o1) == "tok-"+strings.TrimPrefix(fake.txs[ref1].requestID, "req-") &&
		scalar[string](t, "select provider_payment_request_id from payments where order_id = ?", o1) == fake.txs[ref1].requestID)
	check(t, "notifikasi 503 tadi ikut tercatat selesai", scalar[int](t, "select count(*) from payment_events where provider_payment_id = ? and event_type = 'SUCCESS' and processed_at is not null", ref1) == 1)
	check(t, "stok terpotong: 5 → 4, reserved 0, 1 riwayat sale", stock("stock") == 4 && stock("reserved") == 0 &&
		scalar[int](t, "select count(*) from stock_movements where order_id = ?", o1) == 1)
	check(t, "notifikasi dobel → 200, tidak memotong stok lagi", notify(t, ref1, "SUCCESS", dokuSecret).Status == 200 && stock("stock") == 4)
	r = call(t, "GET", fmt.Sprintf("/api/v1/orders/%d", o1), buyer, nil)
	check(t, "GET /orders/:id memuat status PAID + payments", r.Status == 200 && r.Data()["status"] == "PAID" && len(r.Data()["payments"].([]any)) == 1, r.Status)
	check(t, "bayar order milik orang lain → 404", call(t, "POST", fmt.Sprintf("/api/v1/orders/%d/pay", o1), admin, nil).Status == 404)
	r = pay(o1)
	check(t, "bayar order yang sudah lunas → ditolak", r.Status == 400 || r.Status == 409, r.Status)

	// ---------------- nominal tidak cocok
	o2 := checkout(map[uint]int{dg: 1})
	pay(o2)
	fake.complete(ref(o2), 1)
	notify(t, ref(o2), "SUCCESS", dokuSecret)
	check(t, "nominal lunas ≠ total → payment FAILED, order tetap belum bayar", payStatus(o2) == "FAILED" && orderStatus(o2) == "PENDING_PAYMENT", payStatus(o2), orderStatus(o2))
	r = pay(o2)
	check(t, "bayar ulang setelah gagal → invoice baru P02", r.Status == 200 && r.Data()["reference_id"] == scalar[string](t, "select order_no from orders where id = ?", o2)+"P02", string(r.Raw))

	// ---------------- pembeli membatalkan
	o3 := checkout(map[uint]int{bs: 1})
	pay(o3)
	ref3 := ref(o3)
	fake.choose(ref3, "VIRTUAL_ACCOUNT", "VIRTUAL_ACCOUNT_BRI")
	check(t, "reserve order 3", stock("reserved") == 1)
	r = call(t, "PUT", fmt.Sprintf("/api/v1/orders/%d/cancel", o3), buyer, map[string]any{"reason": "berubah pikiran"})
	check(t, "pembeli batal (VA) → order di-cancel di DOKU, payment CANCELLED, reserve dilepas", r.Status == 200 && fake.status(ref3) == "CANCELLED" &&
		payStatus(o3) == "CANCELLED" && orderStatus(o3) == "CANCELLED" && stock("reserved") == 0, r.Status, fake.status(ref3), payStatus(o3))
	o3b := checkout(map[uint]int{dg: 1})
	pay(o3b)
	ref3b := ref(o3b)
	fake.choose(ref3b, "EMONEY", "EMONEY_SHOPEE_PAY")
	r = call(t, "PUT", fmt.Sprintf("/api/v1/orders/%d/cancel", o3b), buyer, nil)
	check(t, "pembeli batal (e-wallet, DOKU tidak bisa cancel) → payment CANCELLED di sisi toko, link DOKU masih PENDING",
		r.Status == 200 && payStatus(o3b) == "CANCELLED" && orderStatus(o3b) == "CANCELLED" && fake.status(ref3b) == "PENDING", r.Status, payStatus(o3b), fake.status(ref3b))
	fake.complete(ref3b, 0)
	notify(t, ref3b, "SUCCESS", dokuSecret)
	check(t, "tetap dibayar lewat link lama → bayar telat, order PAID", orderStatus(o3b) == "PAID" && payStatus(o3b) == "PAID", orderStatus(o3b), payStatus(o3b))

	// ---------------- kedaluwarsa + pembayaran telat
	o4 := checkout(map[uint]int{dg: 1})
	pay(o4)
	ref4 := ref(o4)
	fake.choose(ref4, "QRIS", "QRIS")
	o4b := checkout(map[uint]int{bs: 1}) // pembeli belum memilih channel (DOKU tidak bisa cancel)
	pay(o4b)
	ref4b := ref(o4b)
	o5 := checkout(map[uint]int{bs: 2})
	pay(o5)
	if err := config.DB.Exec("update orders set expires_at = now() - interval '1 minute' where id in (?, ?)", o4, o4b).Error; err != nil {
		t.Fatal(err)
	}
	n, err := paymentservices.ReconcileAndExpire(context.Background(), time.Now())
	check(t, "rekonsiliasi: order 4 & 4b EXPIRED (4 di-cancel di DOKU, 4b ditutup di sisi toko), reserve order 5 tetap 2",
		err == nil && n == 2 && orderStatus(o4) == "EXPIRED" && orderStatus(o4b) == "EXPIRED" && fake.status(ref4) == "CANCELLED" &&
			fake.status(ref4b) == "PENDING" && payStatus(o4) == "EXPIRED" && payStatus(o4b) == "EXPIRED" && stock("reserved") == 2,
		n, err, orderStatus(o4), orderStatus(o4b), payStatus(o4), payStatus(o4b), stock("reserved"))
	fake.complete(ref4b, 0) // halaman Checkout yang belum habis masih bisa dibayar
	notify(t, ref4b, "SUCCESS", dokuSecret)
	check(t, "pembayaran telat: EXPIRED → PAID, jejak cancelled_at tetap ada", orderStatus(o4b) == "PAID" && payStatus(o4b) == "PAID" &&
		scalar[int](t, "select count(*) from orders where id = ? and cancelled_at is not null and paid_at is not null", o4b) == 1)
	check(t, "stok 4 → 3 tanpa menyentuh reserve order 5", stock("stock") == 3 && stock("reserved") == 2, stock("stock"), stock("reserved"))
	call(t, "PUT", fmt.Sprintf("/api/v1/orders/%d/cancel", o5), buyer, nil)
	check(t, "batal order 5 (belum pilih channel) → payment CANCELLED, reserve 0", stock("reserved") == 0 && payStatus(o5) == "CANCELLED", payStatus(o5))

	// ---------------- notifikasi SUCCESS yang terlalu cepat dikejar rekonsiliasi
	o8 := checkout(map[uint]int{dg: 1})
	pay(o8)
	ref8 := ref(o8)
	fake.choose(ref8, "VIRTUAL_ACCOUNT", "VIRTUAL_ACCOUNT_BNI")
	check(t, "SUCCESS saat DOKU masih PENDING → 503", notify(t, ref8, "SUCCESS", dokuSecret).Status == 503)
	fake.complete(ref8, 0)
	_, err = paymentservices.ReconcileAndExpire(context.Background(), time.Now())
	check(t, "rekonsiliasi berikutnya (payment < 10 menit) → PAID, notifikasi tercatat selesai", err == nil && orderStatus(o8) == "PAID" &&
		scalar[int](t, "select count(*) from payment_events where provider_payment_id = ? and processed_at is null", ref8) == 0, err, orderStatus(o8))

	// ---------------- notifikasi EXPIRED
	o9 := checkout(map[uint]int{dg: 1})
	pay(o9)
	ref9 := ref(o9)
	fake.setStatus(ref9, "EXPIRED")
	r = notifyBody(t, []byte(`{"order":{"invoice_number":"`+ref9+`","amount":95000,"status":"ORDER_EXPIRED"}}`), dokuSecret)
	check(t, "notifikasi kedaluwarsa halaman Checkout → payment EXPIRED", r.Status == 200 && payStatus(o9) == "EXPIRED", r.Status, payStatus(o9))

	// ---------------- pembayaran belum dikonfigurasi
	o6 := checkout(map[uint]int{dg: 1})
	paymentservices.Configure(nil, paymentservices.Settings{})
	r = pay(o6)
	rw := notify(t, ref1, "SUCCESS", dokuSecret)
	paymentservices.Configure(fake, paymentSettings)
	check(t, "pembayaran belum dikonfigurasi → bayar & notifikasi 503", r.Status == 503 && rw.Status == 503, r.Status, rw.Status)

	// ---------------- PUT /orders/:id/status oleh staf
	call(t, "POST", "/api/v1/users", owner, map[string]any{"name": "Gudang", "email": "gudang@test.local", "password": buyerPassword, "role": "warehouse"})
	gudang, _ := login(t, "/api/v1/auth/login/admin", "gudang@test.local", buyerPassword)
	o7 := checkout(map[uint]int{bs: 1})
	pay(o7)
	ref7 := ref(o7)
	fake.choose(ref7, "VIRTUAL_ACCOUNT", "VIRTUAL_ACCOUNT_MANDIRI")
	setStatus := func(token string, o uint, status string) resp {
		return call(t, "PUT", fmt.Sprintf("/api/v1/orders/%d/status", o), token, map[string]any{"status": status, "reason": "uji"})
	}
	check(t, "staf set PAID manual → 400 (hanya dari DOKU)", setStatus(owner, o7, "PAID").Status == 400 && orderStatus(o7) == "PENDING_PAYMENT")
	check(t, "staf set EXPIRED manual → 400", setStatus(owner, o7, "EXPIRED").Status == 400)
	check(t, "warehouse membatalkan → 403", setStatus(gudang, o7, "CANCELLED").Status == 403 && orderStatus(o7) == "PENDING_PAYMENT")
	r = setStatus(owner, o7, "CANCELLED")
	check(t, "owner membatalkan → order DOKU ikut di-cancel, payment CANCELLED, reserve dilepas",
		r.Status == 200 && orderStatus(o7) == "CANCELLED" && fake.status(ref7) == "CANCELLED" && payStatus(o7) == "CANCELLED" && stock("reserved") == 0,
		r.Status, fake.status(ref7), payStatus(o7), stock("reserved"))
	check(t, "warehouse refund → 403", setStatus(gudang, o1, "REFUNDED").Status == 403)
	r = setStatus(owner, o1, "REFUNDED")
	check(t, "owner REFUNDED → pembayaran lunas ikut REFUNDED", r.Status == 200 && orderStatus(o1) == "REFUNDED" && payStatus(o1) == "REFUNDED",
		r.Status, orderStatus(o1), payStatus(o1))

	// ---------------- data internal DOKU tidak terlihat pembeli
	r = call(t, "GET", fmt.Sprintf("/api/v1/orders/%d", o2), buyer, nil)
	raw := string(r.Raw)
	check(t, "response pembeli tanpa ID internal DOKU & failure_reason", r.Status == 200 && strings.Contains(raw, `"status":"FAILED"`) &&
		!strings.Contains(raw, "provider_session_id") && !strings.Contains(raw, "provider_payment") && !strings.Contains(raw, "failure_reason") &&
		!strings.Contains(raw, "provider_response"), raw)

	// ---------------- cari pesanan pakai nomor pesanan (halaman sukses ?order=SP…)
	orderNo := scalar[string](t, "select order_no from orders where id = ?", o1)
	r = call(t, "GET", "/api/v1/orders/"+orderNo, buyer, nil)
	check(t, "GET /orders/SP… → pesanan yang sama", r.Status == 200 && uint(num(r.Data()["id"])) == o1, r.Status)
	check(t, "nomor pesanan huruf kecil juga dikenali", call(t, "GET", "/api/v1/orders/"+strings.ToLower(orderNo), buyer, nil).Status == 200)
	check(t, "nomor pesanan milik orang lain → 404", call(t, "GET", "/api/v1/orders/"+orderNo, admin, nil).Status == 404)
	check(t, "nomor pesanan tidak ada → 404", call(t, "GET", "/api/v1/orders/SP000000NOTEXIST", buyer, nil).Status == 404)
}
