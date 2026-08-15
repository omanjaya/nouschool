package student

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
)

// Berkas ini berisi adaptor parser untuk file export Dapodik (Fase 11,
// docs/03-student.md "2. File export Dapodik"): format export Dapodik
// bervariasi antar versi/sekolah (header berbeda kata, urutan kolom beda,
// beberapa aplikasi menambah kolom lain) — parser di sini TOLERAN: mapping
// kolom by NAMA header (case/spasi-insensitive, sinonim umum), BUKAN posisi.
// Hasil akhirnya adalah []studentImportRow — TIPE YANG SAMA dipakai pipeline
// import Excel/CSV template NouSchool (import.go) — supaya preview & commit
// memakai SATU jalur (ImportStore, buildStudentPreview, CommitStudentImport)
// persis seperti diminta docs/03: "Feed ke pipeline ImportRows existing".

// dapodikHeaderSynonyms memetakan variasi nama kolom umum export Dapodik ->
// kunci kanonik dipakai indexDapodikHeader. Variasi TIDAK lengkap (Dapodik
// tidak punya format resmi tunggal) — daftar ini boleh bertambah kalau ada
// contoh file baru yang gagal ter-parse.
var dapodikHeaderSynonyms = map[string]string{
	"nama peserta didik":     "nama",
	"nama siswa":             "nama",
	"nama lengkap":           "nama",
	"nama":                   "nama",
	"nisn":                   "nisn",
	"nis":                    "nis",
	"no induk":               "nis",
	"no induk sekolah":       "nis",
	"nomor induk":            "nis",
	"nomor induk sekolah":    "nis",
	"jk":                     "jenis_kelamin",
	"jenis kelamin":          "jenis_kelamin",
	"jenis_kelamin":          "jenis_kelamin",
	"l/p":                    "jenis_kelamin",
	"tanggal lahir":          "tanggal_lahir",
	"tgl lahir":              "tanggal_lahir",
	"tanggal lahir siswa":    "tanggal_lahir",
	"tanggal_lahir":          "tanggal_lahir",
	"rombel":                 "rombel",
	"rombongan belajar":      "rombel",
	"nama rombongan belajar": "rombel",
	"kelas":                  "rombel",
}

// normalizeHeaderLoose lebih longgar dari normalizeHeader (import.go): buang
// tanda titik ("No. Induk" -> "no induk") dan kolaps spasi berulang, supaya
// variasi spasi/tanda baca header Dapodik tetap cocok dengan sinonim di atas.
func normalizeHeaderLoose(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, ".", "")
	return strings.Join(strings.Fields(s), " ")
}

// indexDapodikHeader memetakan kunci kanonik (nama, nisn, nis, jenis_kelamin,
// tanggal_lahir, rombel) -> indeks kolom pada baris header file.
func indexDapodikHeader(header []string) map[string]int {
	idx := make(map[string]int, len(header))
	for i, h := range header {
		canon, ok := dapodikHeaderSynonyms[normalizeHeaderLoose(h)]
		if !ok {
			continue
		}
		if _, exists := idx[canon]; exists {
			continue // header pertama yang cocok menang (menghindari kolom duplikat tak sengaja)
		}
		idx[canon] = i
	}
	return idx
}

// parseDapodikGender menerima L/P, "Laki-laki"/"Perempuan" (dengan variasi
// spasi/hubung), case-insensitive. String kosong dianggap valid (tidak diisi).
func parseDapodikGender(raw string) (string, bool) {
	v := strings.ToUpper(strings.TrimSpace(raw))
	v = strings.ReplaceAll(v, "-", " ")
	v = strings.Join(strings.Fields(v), " ")
	switch v {
	case "":
		return "", true
	case "L", "LAKI LAKI", "LAKI":
		return "L", true
	case "P", "PEREMPUAN":
		return "P", true
	default:
		return "", false
	}
}

// dapodikDateLayouts — layout tambahan umum dijumpai export Dapodik, di luar
// yang sudah didukung parseFlexibleDate ("2006-01-02", "02/01/2006", serial Excel).
var dapodikDateLayouts = []string{
	"02-01-2006",
	"2006/01/02",
	"2-1-2006",
	"2/1/2006",
}

// parseDapodikDate — bungkus parseFlexibleDate (import.go) dengan layout
// tambahan gaya Dapodik.
func parseDapodikDate(s string) (time.Time, bool) {
	if t, ok := parseFlexibleDate(s); ok {
		return t, true
	}
	s = strings.TrimSpace(s)
	for _, layout := range dapodikDateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// dapodikImportLookup — kebutuhan parser dari data sekolah: matching by NISN
// dulu, fallback NIS (docs/03-student.md), + resolusi rombel by nama (sama
// seperti studentImportLookup). Dideklarasikan sebagai interface supaya
// parseDapodikRows bisa dites dengan fake map, tanpa DB (lihat import_test.go).
type dapodikImportLookup interface {
	StudentIDByNISN(nisn string) (id int64, exists bool)
	StudentIDByNIS(nis string) (id int64, exists bool)
	ClassIDByName(name string) (id int64, exists bool)
}

// parseDapodikRows mem-parse baris mentah (hasil readRecords, sama seperti
// parseStudentRows) hasil export Dapodik menjadi []studentImportRow — TIPE
// YANG SAMA dipakai template NouSchool, supaya buildStudentPreview/
// ImportStore/CommitStudentImport dipakai APA ADANYA (docs/03: "Feed ke
// pipeline ImportRows existing").
func parseDapodikRows(records [][]string, lookup dapodikImportLookup) ([]studentImportRow, error) {
	if len(records) == 0 {
		return nil, fmt.Errorf("file kosong")
	}
	idx := indexDapodikHeader(records[0])
	if _, ok := idx["nama"]; !ok {
		return nil, fmt.Errorf("kolom nama peserta didik tidak ditemukan di header")
	}
	if _, hasNIS := idx["nis"]; !hasNIS {
		if _, hasNISN := idx["nisn"]; !hasNISN {
			return nil, fmt.Errorf("kolom NIS atau NISN wajib ada di header (tidak ditemukan keduanya)")
		}
	}

	seenNIS := make(map[string]bool)
	seenNISN := make(map[string]bool)
	rows := make([]studentImportRow, 0, len(records)-1)
	for i := 1; i < len(records); i++ {
		rec := records[i]
		if isBlankRow(rec) {
			continue
		}
		row := studentImportRow{RowNum: i + 1}
		row.NIS = cell(rec, idx, "nis")
		row.NISN = cell(rec, idx, "nisn")
		row.Name = cell(rec, idx, "nama")
		row.ClassName = cell(rec, idx, "rombel")

		var msgs []string
		if row.NIS == "" && row.NISN == "" {
			msgs = append(msgs, "NIS dan NISN tidak diisi — minimal salah satu wajib.")
		}
		if row.Name == "" {
			msgs = append(msgs, "Nama wajib diisi.")
		}
		if genderRaw := cell(rec, idx, "jenis_kelamin"); genderRaw != "" {
			if g, ok := parseDapodikGender(genderRaw); ok {
				row.Gender = g
			} else {
				msgs = append(msgs, "Jenis kelamin tidak dikenali (pakai L/P atau Laki-laki/Perempuan).")
			}
		}
		if bd := cell(rec, idx, "tanggal_lahir"); bd != "" {
			if t, ok := parseDapodikDate(bd); ok {
				d := NewDate(t)
				row.BirthDate = &d
			} else {
				msgs = append(msgs, "Format tanggal lahir tidak dikenali.")
			}
		}
		if row.ClassName != "" {
			if id, ok := lookup.ClassIDByName(row.ClassName); ok {
				row.ClassID = id
			} else {
				msgs = append(msgs, fmt.Sprintf("Rombel %q tidak dikenal.", row.ClassName))
			}
		}
		if row.NIS != "" {
			if seenNIS[row.NIS] {
				msgs = append(msgs, "NIS duplikat dalam file.")
			} else {
				seenNIS[row.NIS] = true
			}
		}
		if row.NISN != "" {
			if seenNISN[row.NISN] {
				msgs = append(msgs, "NISN duplikat dalam file.")
			} else {
				seenNISN[row.NISN] = true
			}
		}

		if len(msgs) == 0 {
			// Matching by NISN dulu, fallback NIS (docs/03-student.md).
			if row.NISN != "" {
				if id, ok := lookup.StudentIDByNISN(row.NISN); ok {
					row.Action, row.Existing, row.ExistingID = "update", true, id
				}
			}
			if row.Action == "" && row.NIS != "" {
				if id, ok := lookup.StudentIDByNIS(row.NIS); ok {
					row.Action, row.Existing, row.ExistingID = "update", true, id
				}
			}
			if row.Action == "" {
				// Siswa baru — NIS wajib (kolom NOT NULL di tabel students).
				// NISN tanpa NIS yang cocok TIDAK bisa dijadikan baris create.
				if row.NIS == "" {
					msgs = append(msgs, "NIS wajib diisi untuk siswa baru (NISN ini belum terdaftar di sekolah).")
				} else {
					row.Action = "create"
				}
			}
		}

		if len(msgs) > 0 {
			row.Action = "error"
		}
		row.Messages = msgs
		rows = append(rows, row)
	}
	return rows, nil
}

// repoDapodikLookup mengadaptasi Repository menjadi dapodikImportLookup
// (sama pola dengan repoStudentLookup di import.go).
type repoDapodikLookup struct {
	ctx      context.Context
	repo     studentRepository
	schoolID int64
	yearID   int64
}

func (l repoDapodikLookup) StudentIDByNISN(nisn string) (int64, bool) {
	rec, err := l.repo.GetStudentByNISN(l.ctx, l.schoolID, nisn)
	if err != nil {
		return 0, false
	}
	return rec.ID, true
}

func (l repoDapodikLookup) StudentIDByNIS(nis string) (int64, bool) {
	rec, err := l.repo.GetStudentByNIS(l.ctx, l.schoolID, nis)
	if err != nil {
		return 0, false
	}
	return rec.ID, true
}

func (l repoDapodikLookup) ClassIDByName(name string) (int64, bool) {
	rec, err := l.repo.GetClassByNameYear(l.ctx, l.schoolID, l.yearID, name)
	if err != nil {
		return 0, false
	}
	return rec.ID, true
}

// PreviewDapodikImport — POST /api/import/dapodik. Sama alur dengan
// PreviewStudentImport (docs/03: "preview & validasi di layar"), hanya
// parser & lookup-nya beda (matching NISN dulu). Hasilnya disimpan di
// ImportStore YANG SAMA (kind "students") — CommitStudentImport dipakai APA
// ADANYA untuk commit (lihat handler.go).
func (s *Service) PreviewDapodikImport(ctx context.Context, schoolID int64, filename string, content []byte) (ImportPreview, error) {
	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return ImportPreview{}, err
	}
	records, err := readRecords(filename, content)
	if err != nil {
		return ImportPreview{}, httpx.Validation(err.Error())
	}
	lookup := repoDapodikLookup{ctx: ctx, repo: s.repo, schoolID: schoolID, yearID: yearID}
	rows, err := parseDapodikRows(records, lookup)
	if err != nil {
		return ImportPreview{}, httpx.Validation(err.Error())
	}
	uploadID, err := s.importStore.putStudents(rows)
	if err != nil {
		return ImportPreview{}, err
	}
	preview := buildStudentPreview(rows)
	preview.UploadID = uploadID
	return preview, nil
}
