-- Gateway pembayaran pindah dari Xendit ke Midtrans Snap (2026-11).
-- Arti kolom payments untuk Midtrans:
--   reference_id        = order_id yang dikirim ke Midtrans
--   provider_session_id = Snap token
--   provider_payment_id = transaction_id (diisi setelah pembeli memilih metode bayar)
--   checkout_url        = redirect_url halaman Snap
--   provider_payment_request_id tidak dipakai Midtrans.
-- 'xendit' tetap diizinkan supaya baris uji lama di database dev tidak melanggar CHECK.

-- +goose Up
ALTER TABLE payments DROP CONSTRAINT payments_provider_chk;
ALTER TABLE payments ADD CONSTRAINT payments_provider_chk CHECK (provider IN ('midtrans', 'xendit'));
ALTER TABLE payments ALTER COLUMN provider SET DEFAULT 'midtrans';

-- +goose Down
-- Gagal (CHECK violation) kalau sudah ada baris provider = 'midtrans': data
-- pembayaran Midtrans tidak boleh hilang diam-diam.
ALTER TABLE payments ALTER COLUMN provider SET DEFAULT 'xendit';
ALTER TABLE payments DROP CONSTRAINT payments_provider_chk;
ALTER TABLE payments ADD CONSTRAINT payments_provider_chk CHECK (provider IN ('xendit'));
