-- Gateway pembayaran pindah dari Midtrans Snap ke DOKU Checkout (2026-11).
-- Arti kolom payments untuk DOKU:
--   reference_id                = order.invoice_number yang dikirim ke DOKU: nomor pesanan
--                                 + "P" + nomor percobaan (mis. SP261116A1B2C3D4P01). Maks 30
--                                 karakter tanpa simbol supaya aman untuk kartu kredit & KKI,
--                                 jadi tidak bisa lagi bertipe UUID.
--   provider_session_id         = token_id halaman Checkout
--   provider_payment_request_id = Request-Id saat create (dibutuhkan API cancel)
--   checkout_url                = payment.url halaman Checkout
--   provider_payment_id         tidak dipakai DOKU.
-- reference_id lama (UUID dari Xendit/Midtrans) tetap valid sebagai teks.
-- 'midtrans' & 'xendit' tetap diizinkan untuk baris lama di database dev.

-- +goose Up
ALTER TABLE payments ALTER COLUMN reference_id TYPE VARCHAR(64) USING reference_id::text;
ALTER TABLE payments DROP CONSTRAINT payments_provider_chk;
ALTER TABLE payments ADD CONSTRAINT payments_provider_chk CHECK (provider IN ('doku', 'midtrans', 'xendit'));
ALTER TABLE payments ALTER COLUMN provider SET DEFAULT 'doku';

-- +goose Down
-- Gagal kalau sudah ada baris provider = 'doku' (CHECK) atau reference_id yang bukan UUID:
-- data pembayaran DOKU tidak boleh hilang diam-diam.
ALTER TABLE payments ALTER COLUMN provider SET DEFAULT 'midtrans';
ALTER TABLE payments DROP CONSTRAINT payments_provider_chk;
ALTER TABLE payments ADD CONSTRAINT payments_provider_chk CHECK (provider IN ('midtrans', 'xendit'));
ALTER TABLE payments ALTER COLUMN reference_id TYPE UUID USING reference_id::uuid;
