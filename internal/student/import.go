package student

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
)

// Berkas ini berisi pipeline import Excel/CSV (docs/03-student.md:
// ImportRows -> validate -> preview -> commit). Fungsi parse murni (tanpa
// I/O, tanpa DB) supaya bisa dites lewat fake lookup — lihat import_test.go.

// ImportRowResult adalah satu baris hasil preview/commit, dikirim ke klien.
type ImportRowResult struct {
	Row      int            `json:"row"`
	Data     map[string]any `json:"data"`
	Action   string         `json:"action"` // create|update|error
	Messages []string       `json:"messages,omitempty"`
}

// ImportSummary meringkas hasil parse satu file.
type ImportSummary struct {
	Total  int `json:"total"`
	Create int `json:"create"`
	Update int `json:"update"`
	Error  int `json:"error"`
}

// ImportPreview adalah response POST /api/import/{students,teachers}.
type ImportPreview struct {
	UploadID string            `json:"upload_id"`
	Summary  ImportSummary     `json:"summary"`
	Rows     []ImportRowResult `json:"rows"`
}

// -- pembacaan file --

func readRecords(filename string, content []byte) ([][]string, error) {
	lower := strings.ToLower(strings.TrimSpace(filename))
	switch {
	case strings.HasSuffix(lower, ".xlsx"):
		return readXLSX(bytes.NewReader(content))
	case strings.HasSuffix(lower, ".csv"):
		return readCSV(bytes.NewReader(content))
	default:
		return nil, fmt.Errorf("format file tidak didukung — gunakan .xlsx atau .csv")
	}
}

func readCSV(r io.Reader) ([][]string, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true
	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("gagal membaca file CSV: %w", err)
	}
	return records, nil
}

func readXLSX(r io.Reader) ([][]string, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca file Excel: %w", err)
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("file Excel tidak punya sheet")
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("gagal membaca sheet Excel: %w", err)
	}
	return rows, nil
}

// -- header & sel --

func normalizeHeader(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// indexHeader memetakan nama header (case-insensitive) -> indeks kolom,
// supaya urutan kolom di file import bebas (docs/03: "mapping kolom by
// header name, bukan posisi").
func indexHeader(header []string) map[string]int {
	idx := make(map[string]int, len(header))
	for i, h := range header {
		idx[normalizeHeader(h)] = i
	}
	return idx
}

func cell(row []string, idx map[string]int, name string) string {
	i, ok := idx[name]
	if !ok || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func isBlankRow(rec []string) bool {
	for _, c := range rec {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// parseFlexibleDate menerima teks "YYYY-MM-DD", "DD/MM/YYYY", atau serial
// tanggal Excel (angka hari sejak 1899-12-30) — lihat docs/03-student.md.
func parseFlexibleDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{"2006-01-02", "02/01/2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		if t, err := excelize.ExcelDateToTime(f, false); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// -- import siswa --

// studentImportRow adalah satu baris siswa yang sudah divalidasi.
type studentImportRow struct {
	RowNum     int
	NIS        string
	NISN       string
	Name       string
	Gender     string
	BirthDate  *Date
	ClassName  string
	ClassID    int64
	Existing   bool
	ExistingID int64
	Action     string // create|update|error
	Messages   []string
}

// studentImportLookup adalah kebutuhan parser dari data sekolah (NIS sudah
// terdaftar? rombel dikenal?) — dideklarasikan sebagai interface supaya
// parseStudentRows bisa dites dengan fake map, tanpa DB.
type studentImportLookup interface {
	StudentIDByNIS(nis string) (id int64, exists bool)
	ClassIDByName(name string) (id int64, exists bool)
}

func parseStudentRows(records [][]string, lookup studentImportLookup) ([]studentImportRow, error) {
	if len(records) == 0 {
		return nil, fmt.Errorf("file kosong")
	}
	idx := indexHeader(records[0])
	for _, required := range []string{"nis", "nama"} {
		if _, ok := idx[required]; !ok {
			return nil, fmt.Errorf("kolom %q wajib ada di header", required)
		}
	}

	seenNIS := make(map[string]bool)
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
		if row.NIS == "" {
			msgs = append(msgs, "NIS wajib diisi.")
		}
		if row.Name == "" {
			msgs = append(msgs, "Nama wajib diisi.")
		}
		if genderRaw := strings.ToUpper(cell(rec, idx, "jenis_kelamin")); genderRaw != "" {
			if genderRaw != "L" && genderRaw != "P" {
				msgs = append(msgs, "Jenis kelamin harus L atau P.")
			} else {
				row.Gender = genderRaw
			}
		}
		if bd := cell(rec, idx, "tanggal_lahir"); bd != "" {
			if t, ok := parseFlexibleDate(bd); ok {
				d := NewDate(t)
				row.BirthDate = &d
			} else {
				msgs = append(msgs, "Format tanggal lahir tidak dikenali (pakai YYYY-MM-DD atau DD/MM/YYYY).")
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

		if len(msgs) > 0 {
			row.Action = "error"
		} else if id, ok := lookup.StudentIDByNIS(row.NIS); ok {
			row.Action = "update"
			row.Existing = true
			row.ExistingID = id
		} else {
			row.Action = "create"
		}
		row.Messages = msgs
		rows = append(rows, row)
	}
	return rows, nil
}

func buildStudentPreview(rows []studentImportRow) ImportPreview {
	var sum ImportSummary
	results := make([]ImportRowResult, 0, len(rows))
	for _, r := range rows {
		sum.Total++
		switch r.Action {
		case "create":
			sum.Create++
		case "update":
			sum.Update++
		case "error":
			sum.Error++
		}
		var birth any
		if r.BirthDate != nil {
			birth = r.BirthDate.Format("2006-01-02")
		}
		data := map[string]any{
			"nis": r.NIS, "nisn": r.NISN, "nama": r.Name, "jenis_kelamin": r.Gender,
			"tanggal_lahir": birth, "rombel": r.ClassName,
		}
		results = append(results, ImportRowResult{Row: r.RowNum, Data: data, Action: r.Action, Messages: r.Messages})
	}
	return ImportPreview{Summary: sum, Rows: results}
}

// -- import guru --

type teacherImportRow struct {
	RowNum     int
	NIP        string
	Name       string
	Email      string
	Existing   bool
	ExistingID int64
	Action     string
	Messages   []string
}

// teacherImportLookup — lihat studentImportLookup. Matching by email,
// fallback NIP (docs/03-student.md).
type teacherImportLookup interface {
	TeacherIDByEmail(email string) (id int64, exists bool)
	TeacherIDByNIP(nip string) (id int64, exists bool)
}

func parseTeacherRows(records [][]string, lookup teacherImportLookup) ([]teacherImportRow, error) {
	if len(records) == 0 {
		return nil, fmt.Errorf("file kosong")
	}
	idx := indexHeader(records[0])
	if _, ok := idx["nama"]; !ok {
		return nil, fmt.Errorf("kolom %q wajib ada di header", "nama")
	}

	seenEmail := make(map[string]bool)
	seenNIP := make(map[string]bool)
	rows := make([]teacherImportRow, 0, len(records)-1)
	for i := 1; i < len(records); i++ {
		rec := records[i]
		if isBlankRow(rec) {
			continue
		}
		row := teacherImportRow{RowNum: i + 1}
		row.NIP = cell(rec, idx, "nip")
		row.Name = cell(rec, idx, "nama")
		row.Email = strings.ToLower(cell(rec, idx, "email"))

		var msgs []string
		if row.Name == "" {
			msgs = append(msgs, "Nama wajib diisi.")
		}
		if row.Email == "" && row.NIP == "" {
			msgs = append(msgs, "Email atau NIP wajib diisi salah satu.")
		}
		if row.Email != "" {
			if seenEmail[row.Email] {
				msgs = append(msgs, "Email duplikat dalam file.")
			} else {
				seenEmail[row.Email] = true
			}
		}
		if row.NIP != "" {
			if seenNIP[row.NIP] {
				msgs = append(msgs, "NIP duplikat dalam file.")
			} else {
				seenNIP[row.NIP] = true
			}
		}

		if len(msgs) > 0 {
			row.Action = "error"
		} else {
			if row.Email != "" {
				if id, ok := lookup.TeacherIDByEmail(row.Email); ok {
					row.Action, row.Existing, row.ExistingID = "update", true, id
				}
			}
			if row.Action == "" && row.NIP != "" {
				if id, ok := lookup.TeacherIDByNIP(row.NIP); ok {
					row.Action, row.Existing, row.ExistingID = "update", true, id
				}
			}
			if row.Action == "" {
				row.Action = "create"
			}
		}
		row.Messages = msgs
		rows = append(rows, row)
	}
	return rows, nil
}

func buildTeacherPreview(rows []teacherImportRow) ImportPreview {
	var sum ImportSummary
	results := make([]ImportRowResult, 0, len(rows))
	for _, r := range rows {
		sum.Total++
		switch r.Action {
		case "create":
			sum.Create++
		case "update":
			sum.Update++
		case "error":
			sum.Error++
		}
		data := map[string]any{"nip": r.NIP, "nama": r.Name, "email": r.Email}
		results = append(results, ImportRowResult{Row: r.RowNum, Data: data, Action: r.Action, Messages: r.Messages})
	}
	return ImportPreview{Summary: sum, Rows: results}
}

// -- orkestrasi service (preview & commit) --

// repoStudentLookup mengadaptasi Repository (di dalam satu tahun ajaran &
// sekolah tertentu) menjadi studentImportLookup dipakai parseStudentRows.
type repoStudentLookup struct {
	ctx      context.Context
	repo     studentRepository
	schoolID int64
	yearID   int64
}

func (l repoStudentLookup) StudentIDByNIS(nis string) (int64, bool) {
	rec, err := l.repo.GetStudentByNIS(l.ctx, l.schoolID, nis)
	if err != nil {
		return 0, false
	}
	return rec.ID, true
}

func (l repoStudentLookup) ClassIDByName(name string) (int64, bool) {
	rec, err := l.repo.GetClassByNameYear(l.ctx, l.schoolID, l.yearID, name)
	if err != nil {
		return 0, false
	}
	return rec.ID, true
}

// PreviewStudentImport mem-parse & memvalidasi file (xlsx/csv), menyimpan
// hasilnya di ImportStore (TTL 15 menit) dan mengembalikan preview.
func (s *Service) PreviewStudentImport(ctx context.Context, schoolID int64, filename string, content []byte) (ImportPreview, error) {
	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return ImportPreview{}, err
	}
	records, err := readRecords(filename, content)
	if err != nil {
		return ImportPreview{}, httpx.Validation(err.Error())
	}
	lookup := repoStudentLookup{ctx: ctx, repo: s.repo, schoolID: schoolID, yearID: yearID}
	rows, err := parseStudentRows(records, lookup)
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

// CommitStudentImport menerapkan hasil preview (upload_id) secara
// transaksional: baris error dilewati, baris create/update di-upsert +
// dienroll ke rombel. Satu entri audit_log ringkasan.
func (s *Service) CommitStudentImport(ctx context.Context, actorUserID, schoolID int64, uploadID string) (created, updated, skipped int, err error) {
	rows, ok := s.importStore.getStudents(uploadID)
	if !ok {
		return 0, 0, 0, httpx.Validation("Sesi import tidak ditemukan atau sudah kedaluwarsa. Silakan upload ulang.")
	}
	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return 0, 0, 0, err
	}

	commitRows := make([]ImportStudentRow, 0, len(rows))
	for _, r := range rows {
		if r.Action == "error" {
			skipped++
			continue
		}
		commitRows = append(commitRows, ImportStudentRow{
			NIS: r.NIS, NISN: r.NISN, Name: r.Name, Gender: r.Gender, BirthDate: r.BirthDate,
			ClassID: r.ClassID, Existing: r.Existing,
		})
	}

	created, updated, err = s.repo.CommitStudentImport(ctx, schoolID, yearID, commitRows)
	if err != nil {
		return 0, 0, 0, err
	}
	s.audit(ctx, schoolID, actorUserID, "student.import_commit", "student", 0, nil,
		map[string]any{"created": created, "updated": updated, "skipped": skipped, "total": len(rows)})
	s.emitStudents(schoolID)
	return created, updated, skipped, nil
}

// repoTeacherLookup mengadaptasi Repository menjadi teacherImportLookup.
type repoTeacherLookup struct {
	ctx      context.Context
	repo     studentRepository
	schoolID int64
}

func (l repoTeacherLookup) TeacherIDByEmail(email string) (int64, bool) {
	t, err := l.repo.GetTeacherByEmail(l.ctx, l.schoolID, email)
	if err != nil {
		return 0, false
	}
	return t.ID, true
}

func (l repoTeacherLookup) TeacherIDByNIP(nip string) (int64, bool) {
	t, err := l.repo.GetTeacherByNIP(l.ctx, l.schoolID, nip)
	if err != nil {
		return 0, false
	}
	return t.ID, true
}

// PreviewTeacherImport — lihat PreviewStudentImport.
func (s *Service) PreviewTeacherImport(ctx context.Context, schoolID int64, filename string, content []byte) (ImportPreview, error) {
	records, err := readRecords(filename, content)
	if err != nil {
		return ImportPreview{}, httpx.Validation(err.Error())
	}
	lookup := repoTeacherLookup{ctx: ctx, repo: s.repo, schoolID: schoolID}
	rows, err := parseTeacherRows(records, lookup)
	if err != nil {
		return ImportPreview{}, httpx.Validation(err.Error())
	}
	uploadID, err := s.importStore.putTeachers(rows)
	if err != nil {
		return ImportPreview{}, err
	}
	preview := buildTeacherPreview(rows)
	preview.UploadID = uploadID
	return preview, nil
}

// CommitTeacherImport — baris create membuat akun guru baru (password
// placeholder acak, seperti CreateTeacher); baris update memperbarui NIP.
// TIDAK dibungkus satu transaksi DB (pembuatan akun lewat IdentityGateway
// tidak bisa ikut serta dalam pgx.Tx repository student) — kegagalan
// sebagian baris tidak membatalkan baris yang sudah berhasil, konsisten
// dengan sifat "commit menerapkan apa yang preview tandai valid" pada modul
// ini; kegagalan di tengah jalan dilaporkan sebagai error ke klien.
func (s *Service) CommitTeacherImport(ctx context.Context, actorUserID, schoolID int64, uploadID string) (created, updated, skipped int, err error) {
	rows, ok := s.importStore.getTeachers(uploadID)
	if !ok {
		return 0, 0, 0, httpx.Validation("Sesi import tidak ditemukan atau sudah kedaluwarsa. Silakan upload ulang.")
	}

	for _, r := range rows {
		if r.Action == "error" {
			skipped++
			continue
		}
		if r.Existing {
			if _, err := s.repo.UpdateTeacher(ctx, schoolID, r.ExistingID, r.NIP); err != nil {
				return 0, 0, 0, err
			}
			updated++
			continue
		}
		if _, err := s.createTeacherAccount(ctx, schoolID, r.Name, r.Email, r.NIP); err != nil {
			return 0, 0, 0, err
		}
		created++
	}

	s.audit(ctx, schoolID, actorUserID, "teacher.import_commit", "teacher", 0, nil,
		map[string]any{"created": created, "updated": updated, "skipped": skipped, "total": len(rows)})
	return created, updated, skipped, nil
}
