import argparse
import csv
import io
import os
import re
import sys
import zipfile
from xml.etree import ElementTree as ET

M = "{http://schemas.openxmlformats.org/spreadsheetml/2006/main}"
R = "{http://schemas.openxmlformats.org/officeDocument/2006/relationships}"
PKG = "{http://schemas.openxmlformats.org/package/2006/relationships}"

AUDIENCES = ["pria", "wanita", "unisex"]
OCCASIONS = ["harian", "kantor", "malam", "acara", "santai", "olahraga", "ibadah"]
STOPWORDS = set(
    "merah muda biru hijau kuning jingga ungu putih coklat emas abu "
    "eau de du la le les toilette parfum parfume perfume extrait edt edp for men women woman man pour homme femme "
    "the and dan yang tidak belum bukan bibit luzi nama dugaan cium dulu kemungkinan sama dengan grade exclusive regular "
    "merek lokal indonesia teridentifikasi limited edition alternatif tipe bermerek versi ada juga lewat bernama "
    "riset parfum asli kalau bisa".split()
)


class Laporan:
    def __init__(self):
        self.galat, self.peringatan, self.info = [], [], []

    def cetak(self):
        for judul, isi in (("INFO", self.info), ("PERINGATAN", self.peringatan), ("GALAT", self.galat)):
            if isi:
                print(f"\n{judul} ({len(isi)})")
                for baris in isi:
                    print(f"  - {baris}")


def slugify(s):
    return re.sub(r"[^a-z0-9]+", "-", s.lower()).strip("-")


def norm(s):
    return " ".join((s or "").lower().split())


def kolom(ref):
    n = 0
    for ch in re.match(r"[A-Z]+", ref).group(0):
        n = n * 26 + ord(ch) - 64
    return n


def teks(el):
    return "".join(t.text or "" for t in el.iter(f"{M}t"))


def baca_xlsx(path):
    z = zipfile.ZipFile(path)
    shared = []
    if "xl/sharedStrings.xml" in z.namelist():
        shared = [teks(si) for si in ET.fromstring(z.read("xl/sharedStrings.xml")).findall(f"{M}si")]
    wb = ET.fromstring(z.read("xl/workbook.xml"))
    rels = {r.get("Id"): r.get("Target") for r in ET.fromstring(z.read("xl/_rels/workbook.xml.rels")).iter(f"{PKG}Relationship")}
    sheets = {}
    for s in wb.iter(f"{M}sheet"):
        target = rels[s.get(f"{R}id")]
        part = target.lstrip("/") if target.startswith("/") else "xl/" + target
        sheets[s.get("name")] = baca_sheet(ET.fromstring(z.read(part)), shared)
    return sheets


def baca_sheet(root, shared):
    rows = []
    for row in root.iter(f"{M}row"):
        vals = {}
        for pos, c in enumerate(row.findall(f"{M}c"), start=1):
            idx = kolom(c.get("r")) if c.get("r") else pos
            t = c.get("t")
            v = c.find(f"{M}v")
            if t == "s" and v is not None:
                val = shared[int(v.text)]
            elif t == "inlineStr":
                val = teks(c)
            else:
                val = v.text if v is not None and v.text else ""
            vals[idx] = val.strip()
        rows.append(vals)
    if not rows:
        return []
    header = {i: re.sub(r"[✎?]", "", h).strip().lower() for i, h in rows[0].items()}
    return [{header[i]: v for i, v in r.items() if i in header} for r in rows[1:] if any(r.values())]


def nomor(v):
    try:
        return int(float(v))
    except (TypeError, ValueError):
        return None


class CSVSeed:
    def __init__(self, path, wajib=True):
        self.path, self.komentar, self.header, self.rows = path, [], [], []
        if not os.path.exists(path):
            if wajib:
                sys.exit(f"{path} tidak ada")
            return
        for line in open(path, encoding="utf-8").read().split("\n"):
            if line.startswith("#"):
                self.komentar.append(line)
            elif line.strip():
                rec = next(csv.reader([line]))
                if not self.header:
                    self.header = rec
                else:
                    self.rows.append(dict(zip(self.header, rec + [""] * (len(self.header) - len(rec)))))

    def tulis(self, header=None, komentar=None):
        if header:
            self.header = header
        if komentar is not None:
            self.komentar = komentar
        buf = io.StringIO()
        w = csv.writer(buf, lineterminator="\n")
        for k in self.komentar:
            buf.write(k + "\n")
        w.writerow(self.header)
        for r in self.rows:
            w.writerow([r.get(h, "") for h in self.header])
        os.makedirs(os.path.dirname(self.path), exist_ok=True)
        with open(self.path, "w", encoding="utf-8", newline="") as f:
            f.write(buf.getvalue())


def kata_merek(*teks_):
    out = set()
    for t in teks_:
        for w in re.findall(r"[a-z0-9]+", (t or "").lower()):
            if len(w) >= 3 and w not in STOPWORDS and not w.isdigit():
                out.add(w)
    return out


def keputusan(status, dipilih, setuju, lama, lap, label):
    status, setuju, dipilih = norm(status), norm(setuju), (dipilih or "").strip()
    if status == "aman":
        return None
    if setuju not in ("ya", "ganti"):
        lap.peringatan.append(f"{label}: belum diputuskan (kolom Setuju? kosong), nama lama dipakai dulu")
        return None
    if not dipilih or norm(dipilih) == norm(lama):
        return None
    if "(" in dipilih or ")" in dipilih or "?" in dipilih:
        lap.galat.append(f"{label}: nama {dipilih!r} belum lengkap")
        return None
    return " ".join(dipilih.split())


def cocok_untuk(row, lap, label):
    nilai = [norm(x) for x in re.split(r"[;,]", row.get("cocok untuk", "")) if norm(x)]
    salah = [x for x in nilai if x not in OCCASIONS]
    if salah:
        lap.galat.append(f"{label}: Cocok untuk tidak dikenal: {', '.join(salah)} (pilihan: {', '.join(OCCASIONS)})")
    nilai = [x for x in nilai if x in OCCASIONS]
    ibadah = norm(row.get("ibadah", ""))
    if ibadah == "ya" and "ibadah" not in nilai:
        nilai.append("ibadah")
    if ibadah == "kurang" and "ibadah" in nilai:
        nilai.remove("ibadah")
        lap.info.append(f"{label}: Ibadah = Kurang, jadi 'ibadah' dibuang dari Cocok untuk")
    return ";".join(x for x in OCCASIONS if x in nilai)


def main():
    ap = argparse.ArgumentParser(description="Konversi XLSX nama baru dari pemilik ke CSV seed (nama, untuk, cocok untuk, deskripsi, ganti nama di tempat).")
    ap.add_argument("xlsx", help="file XLSX yang sudah diisi pemilik")
    ap.add_argument("--seed", default=os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "seed"), help="folder seed (default: seed/ di repo)")
    ap.add_argument("--periksa", action="store_true", help="hanya laporan, tidak menulis file")
    a = ap.parse_args()

    seed = os.path.abspath(a.seed)
    sheets = baca_xlsx(a.xlsx)
    if "Parfum" not in sheets or "Produk umum" not in sheets:
        sys.exit("XLSX harus punya sheet 'Parfum' dan 'Produk umum'")
    wangian = CSVSeed(os.path.join(seed, "wangian.csv"))
    umum = CSVSeed(os.path.join(seed, "produk_umum.csv"))
    varian = CSVSeed(os.path.join(seed, "varian_parfum.csv"), wajib=False)
    merek = CSVSeed(os.path.join(seed, "internal", "inspired_by.csv"), wajib=False)
    ganti = CSVSeed(os.path.join(seed, "internal", "ganti-nama.csv"), wajib=False)
    foto = CSVSeed(os.path.join(seed, "internal", "ganti-foto.csv"), wajib=False)
    for h in ("untuk", "cocok_untuk", "deskripsi"):
        if h not in wangian.header:
            wangian.header.append(h)

    lap = Laporan()
    by_name = {}
    for r in wangian.rows:
        by_name.setdefault((norm(r["group"]), norm(r["grade"])), []).append(r)
    umum_by_name = {}
    for r in umum.rows:
        umum_by_name.setdefault(norm(r["product_name"]), set()).add(r["product_code"])

    produk_ganti, wangian_ganti, terpakai = [], [], set()
    diputuskan = diganti = 0
    for x in sheets["Parfum"]:
        no, lama, grade = nomor(x.get("no.")), x.get("nama sekarang", ""), norm(x.get("grade"))
        label = f"No. {no} {lama}"
        rows = by_name.get((norm(lama), grade))
        baru = keputusan(x.get("status nama"), x.get("nama dipilih"), x.get("setuju"), lama, lap, label)
        if norm(x.get("setuju")) in ("ya", "ganti"):
            diputuskan += 1
        if not rows and baru:
            rows = by_name.get((norm(baru), grade))
            if rows:
                lap.info.append(f"{label}: sudah bernama {baru!r} di wangian.csv")
                baru = None
        if not rows:
            lap.galat.append(f"{label} ({grade}): tidak ditemukan di wangian.csv")
            continue
        terpakai.update(id(r) for r in rows)
        if len({r["group_code"] for r in rows}) > 1:
            lap.galat.append(f"{label}: nama sama dipakai lebih dari 1 group_code")
            continue
        untuk = norm(x.get("untuk"))
        if untuk and untuk not in AUDIENCES:
            lap.galat.append(f"{label}: Untuk {x.get('untuk')!r} harus Pria, Wanita, atau Unisex")
            untuk = ""
        cocok = cocok_untuk(x, lap, label)
        deskripsi = " ".join((x.get("deskripsi final") or "").split())
        for r in rows:
            if untuk:
                r["untuk"] = untuk
            if cocok:
                r["cocok_untuk"] = cocok
            if deskripsi:
                r["deskripsi"] = deskripsi
        if not baru:
            continue
        bocor = kata_merek(baru) & kata_merek(lama, x.get("inspired by (internal)"), x.get("parfum asli menurut riset (internal)"))
        if bocor:
            lap.galat.append(f"{label}: nama baru {baru!r} masih memuat kata dari nama lama/merek: {', '.join(sorted(bocor))}")
            continue
        code = slugify(baru)
        if not code:
            lap.galat.append(f"{label}: nama baru {baru!r} tidak menghasilkan code")
            continue
        r = rows[0]
        produk_ganti.append((label, r["group_code"], code, "perfume"))
        for r in rows:
            scent = code if len(rows) == 1 else f"{code}-{slugify(r['varian'])}"
            if r["code"] != scent:
                wangian_ganti.append((label, r["code"], scent))
            r["group_code"], r["code"], r["group"] = code, scent, baru
            r["varian"] = baru if len(rows) == 1 else r["varian"]
            r["image"] = f"{code}.jpg"
        diganti += 1

    for r in wangian.rows:
        if id(r) not in terpakai:
            lap.peringatan.append(f"wangian.csv {r['group_code']} ({r['group']}, aktif={r.get('aktif') or 'ya'}): tidak ada di XLSX, nama tidak diubah")

    for x in sheets["Produk umum"]:
        no, lama = nomor(x.get("no.")), x.get("nama sekarang", "")
        label = f"No. {no} {lama}"
        if norm(x.get("setuju")) in ("ya", "ganti"):
            diputuskan += 1
        baru = keputusan(x.get("status nama"), x.get("nama dipilih"), x.get("setuju"), lama, lap, label)
        if not baru:
            continue
        codes = umum_by_name.get(norm(lama))
        if not codes and umum_by_name.get(norm(baru)):
            lap.info.append(f"{label}: sudah bernama {baru!r} di produk_umum.csv")
            continue
        if not codes or len(codes) > 1:
            lap.galat.append(f"{label}: {'tidak ditemukan' if not codes else 'nama dipakai lebih dari 1 product_code'} di produk_umum.csv")
            continue
        bocor = kata_merek(baru) & kata_merek(lama)
        if bocor:
            lap.galat.append(f"{label}: nama baru {baru!r} masih memuat kata dari nama lama: {', '.join(sorted(bocor))}")
            continue
        lama_code, code = next(iter(codes)), slugify(baru)
        for r in umum.rows:
            if r["product_code"] == lama_code:
                r["product_code"], r["product_name"] = code, baru
        produk_ganti.append((label, lama_code, code, "general"))
        diganti += 1

    target = {baru for _, _, baru, _ in produk_ganti}
    semua_nama = {}
    for r in wangian.rows:
        semua_nama.setdefault(norm(r["group"]), set()).add(r["group_code"])
    for r in umum.rows:
        semua_nama.setdefault(norm(r["product_name"]), set()).add(r["product_code"])
    for nama, codes in semua_nama.items():
        if len(codes) > 1 and codes & target:
            lap.galat.append(f"nama {nama!r} dipakai lebih dari 1 produk: {', '.join(sorted(codes))}")
    for jenis, codes in (("group_code parfum", [r["group_code"] for r in wangian.rows]), ("code wangian", [r["code"] for r in wangian.rows])):
        for c in sorted({c for c in codes if codes.count(c) > 1}):
            lap.galat.append(f"{jenis} {c!r} dipakai lebih dari 1 baris")
    umum_awal = {c for codes in umum_by_name.values() for c in codes}
    for label, lama, baru, jenis in produk_ganti:
        if jenis == "general" and baru in umum_awal - {lama}:
            lap.galat.append(f"{label}: code {baru!r} sudah dipakai produk umum lain")
    for c in sorted({r["group_code"] for r in wangian.rows} & {r["product_code"] for r in umum.rows}):
        lap.galat.append(f"code {c!r} dipakai parfum dan produk umum")

    perfume_lama = {lama for _, lama, _, _ in produk_ganti}
    lap.info.insert(0, f"{diputuskan} nama diputuskan pemilik, {diganti} produk diganti namanya")
    lap.cetak()
    if lap.galat:
        print("\nTidak ada file yang ditulis: perbaiki GALAT di XLSX dulu.")
        sys.exit(1)
    if a.periksa:
        print("\nMode --periksa: tidak ada file yang ditulis.")
        return

    peta_wangian = {lama: baru for _, lama, baru in wangian_ganti}
    peta_produk = {lama: baru for _, lama, baru, _ in produk_ganti}
    wangian.rows.sort(key=lambda r: norm(r["group"]))
    wangian.tulis()
    umum.tulis()
    if varian.header:
        for r in varian.rows:
            r["scent_code"] = peta_wangian.get(r["scent_code"], r["scent_code"])
        varian.tulis()
    if merek.header:
        for r in merek.rows:
            r["code"] = peta_produk.get(r["code"], r["code"])
        merek.tulis()

    lama_ganti = {(r["jenis"], r["code_lama"]) for r in ganti.rows}
    for _, lama, baru, _ in produk_ganti:
        if ("produk", lama) not in lama_ganti:
            ganti.rows.append({"jenis": "produk", "code_lama": lama, "code_baru": baru})
    for _, lama, baru in wangian_ganti:
        if ("wangian", lama) not in lama_ganti:
            ganti.rows.append({"jenis": "wangian", "code_lama": lama, "code_baru": baru})
    ganti.tulis(header=["jenis", "code_lama", "code_baru"], komentar=["# INTERNAL: peta ganti nama (code lama → baru). Jangan di-commit."])

    sudah = {r["slug_lama"] for r in foto.rows}
    for _, lama, baru, jenis in produk_ganti:
        if lama not in sudah and lama in perfume_lama:
            foto.rows.append({"slug_lama": lama, "slug_baru": baru, "jenis": jenis})
    foto.tulis(header=["slug_lama", "slug_baru", "jenis"], komentar=["# INTERNAL: foto frontend public/products/<slug>-card|detail.jpg dan product-photos.ts yang perlu di-rename."])

    print(f"\nDitulis: wangian.csv, produk_umum.csv, internal/ganti-nama.csv ({len(ganti.rows)} baris), internal/ganti-foto.csv ({len(foto.rows)} baris)"
          + (", internal/inspired_by.csv" if merek.header else "") + (", varian_parfum.csv" if varian.header else ""))
    print("Berikutnya: go test ./src/config/seeders/ lalu go run ./seed -dry-run")


if __name__ == "__main__":
    main()
