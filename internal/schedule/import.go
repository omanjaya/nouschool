package schedule

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
)

// Berkas ini berisi pipeline import Excel/CSV jadwal (docs/04-schedule.md:
// ParseRows -> validate & lookup referensi -> deteksi bentrok -> preview ->
// commit). Fungsi parse murni (tanpa I/O, tanpa DB) supaya bisa dites lewat
// fake lookup — lihat import_test.go. Pembacaan file & helper header/sel
// SENGAJA diduplikasi kecil dari internal/student/import.go — schedule TIDAK
// mengimpor package student (lihat CLAUDE.md, antar-modul lewat interface
// kecil, bukan import langsung).

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

func normalizeHeader(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

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

// -- import jadwal --

// slotImportRow adalah satu baris jadwal yang sudah divalidasi/di-lookup.
type slotImportRow struct {
	RowNum int

	ClassName string
	ClassID   int64

	DayRaw    string
	DayOfWeek int

	PeriodStart int
	PeriodEnd   int

	SubjectCode string
	SubjectID   int64
	SubjectName string

	TeacherEmail string
	TeacherID    int64
	TeacherName  string

	RoomName string
	RoomID   int64 // 0 = tanpa ruang (boleh kosong)

	Action   string // "ok" | "error"
	Messages []string
}

// scheduleImportLookup adalah kebutuhan parser dari data sekolah (rombel,
// mapel, guru, ruangan dikenal?) — dideklarasikan sebagai interface supaya
// parseSlotRows bisa dites dengan fake map, tanpa DB.
type scheduleImportLookup interface {
	ClassIDByName(name string) (id int64, ok bool)
	SubjectByCode(code string) (id int64, name string, ok bool)
	TeacherByEmail(email string) (id int64, name string, ok bool)
	RoomIDByName(name string) (id int64, ok bool)
}

var requiredScheduleHeaders = []string{"rombel", "hari", "jam_mulai", "jam_selesai", "kode_mapel", "email_guru"}

// parseSlotRows mem-parse & memvalidasi bentuk + referensi (BUKAN bentrok —
// itu langkah terpisah yang butuh state DB "slot lain", lihat
// Service.PreviewScheduleImport / markImportConflicts).
func parseSlotRows(records [][]string, lookup scheduleImportLookup) ([]slotImportRow, error) {
	if len(records) == 0 {
		return nil, fmt.Errorf("file kosong")
	}
	idx := indexHeader(records[0])
	for _, required := range requiredScheduleHeaders {
		if _, ok := idx[required]; !ok {
			return nil, fmt.Errorf("kolom %q wajib ada di header", required)
		}
	}

	rows := make([]slotImportRow, 0, len(records)-1)
	for i := 1; i < len(records); i++ {
		rec := records[i]
		if isBlankRow(rec) {
			continue
		}
		row := slotImportRow{RowNum: i + 1}
		row.ClassName = cell(rec, idx, "rombel")
		row.DayRaw = cell(rec, idx, "hari")
		startRaw := cell(rec, idx, "jam_mulai")
		endRaw := cell(rec, idx, "jam_selesai")
		row.SubjectCode = strings.ToUpper(cell(rec, idx, "kode_mapel"))
		row.TeacherEmail = strings.ToLower(cell(rec, idx, "email_guru"))
		row.RoomName = cell(rec, idx, "ruangan")

		var msgs []string

		if row.ClassName == "" {
			msgs = append(msgs, "Rombel wajib diisi.")
		} else if id, ok := lookup.ClassIDByName(row.ClassName); ok {
			row.ClassID = id
		} else {
			msgs = append(msgs, fmt.Sprintf("Rombel %q tidak dikenal.", row.ClassName))
		}

		if d, ok := dayNameToNumber(row.DayRaw); ok {
			row.DayOfWeek = d
		} else {
			msgs = append(msgs, fmt.Sprintf("Hari %q tidak dikenali (pakai Senin..Sabtu atau 1-6).", row.DayRaw))
		}

		if n, err := strconv.Atoi(strings.TrimSpace(startRaw)); err == nil && n > 0 {
			row.PeriodStart = n
		} else {
			msgs = append(msgs, "jam_mulai harus angka jam ke- (mis. 1, 2, 3).")
		}
		if n, err := strconv.Atoi(strings.TrimSpace(endRaw)); err == nil && n > 0 {
			row.PeriodEnd = n
		} else {
			msgs = append(msgs, "jam_selesai harus angka jam ke-.")
		}
		if row.PeriodStart != 0 && row.PeriodEnd != 0 && row.PeriodStart > row.PeriodEnd {
			msgs = append(msgs, "jam_mulai tidak boleh setelah jam_selesai.")
		}

		if row.SubjectCode == "" {
			msgs = append(msgs, "kode_mapel wajib diisi.")
		} else if id, name, ok := lookup.SubjectByCode(row.SubjectCode); ok {
			row.SubjectID, row.SubjectName = id, name
		} else {
			msgs = append(msgs, fmt.Sprintf("Mapel %q tidak dikenal.", row.SubjectCode))
		}

		if row.TeacherEmail == "" {
			msgs = append(msgs, "email_guru wajib diisi.")
		} else if id, name, ok := lookup.TeacherByEmail(row.TeacherEmail); ok {
			row.TeacherID, row.TeacherName = id, name
		} else {
			msgs = append(msgs, fmt.Sprintf("Guru %q tidak dikenal.", row.TeacherEmail))
		}

		if row.RoomName != "" {
			if id, ok := lookup.RoomIDByName(row.RoomName); ok {
				row.RoomID = id
			} else {
				msgs = append(msgs, fmt.Sprintf("Ruangan %q tidak dikenal.", row.RoomName))
			}
		}

		if len(msgs) > 0 {
			row.Action = "error"
		} else {
			row.Action = "ok"
		}
		row.Messages = msgs
		rows = append(rows, row)
	}
	return rows, nil
}

// markImportConflicts menandai baris "ok" yang bentrok — baik dengan slot
// yang SUDAH ADA di DB (existing) maupun dengan baris LAIN di file yang sama
// yang sudah diterima lebih dulu (working set, diakumulasi baris demi baris,
// pola yang sama dengan pengecekan NIS duplikat di internal/student/import.go).
func markImportConflicts(rows []slotImportRow, existing []SlotRecord) []slotImportRow {
	working := append([]SlotRecord{}, existing...)
	// syntheticID mulai dari -1 dan menurun — TIDAK PERNAH 0 supaya tidak
	// pernah "cocok" dengan excludeSlotID=0 yang dipakai findConflicts saat
	// membuat slot baru (lihat conflict.go: excludeSlotID=0 berarti "tidak
	// mengecualikan slot manapun", bukan "slot ber-ID 0").
	syntheticID := int64(-1)
	for i := range rows {
		r := &rows[i]
		if r.Action != "ok" {
			continue
		}
		conflicts := findConflicts(working, 0, r.DayOfWeek, r.PeriodStart, r.PeriodEnd, r.TeacherID, r.ClassID, r.RoomID)
		if len(conflicts) > 0 {
			r.Action = "error"
			r.Messages = append(r.Messages, conflictMessage(r.DayOfWeek, conflicts, r.TeacherID, r.ClassID, r.RoomID))
			continue
		}
		working = append(working, SlotRecord{
			ID:      syntheticID,
			ClassID: r.ClassID, ClassName: r.ClassName, SubjectID: r.SubjectID, SubjectName: r.SubjectName,
			TeacherID: r.TeacherID, TeacherName: r.TeacherName, RoomID: r.RoomID, RoomName: r.RoomName,
			DayOfWeek: r.DayOfWeek, PeriodStart: r.PeriodStart, PeriodEnd: r.PeriodEnd,
		})
		syntheticID--
	}
	return rows
}

// -- shape response (preview) --

type ImportRowResult struct {
	Row      int            `json:"row"`
	Data     map[string]any `json:"data"`
	Action   string         `json:"action"` // create|error
	Messages []string       `json:"messages,omitempty"`
}

type ImportSummary struct {
	Total  int `json:"total"`
	Create int `json:"create"`
	Error  int `json:"error"`
}

type ImportPreview struct {
	UploadID string            `json:"upload_id"`
	Summary  ImportSummary     `json:"summary"`
	Rows     []ImportRowResult `json:"rows"`
}

func buildSlotPreview(rows []slotImportRow) ImportPreview {
	var sum ImportSummary
	results := make([]ImportRowResult, 0, len(rows))
	for _, r := range rows {
		sum.Total++
		action := "error"
		if r.Action == "ok" {
			action = "create"
			sum.Create++
		} else {
			sum.Error++
		}
		data := map[string]any{
			"rombel": r.ClassName, "hari": r.DayRaw, "jam_mulai": r.PeriodStart, "jam_selesai": r.PeriodEnd,
			"kode_mapel": r.SubjectCode, "email_guru": r.TeacherEmail, "ruangan": r.RoomName,
		}
		results = append(results, ImportRowResult{Row: r.RowNum, Data: data, Action: action, Messages: r.Messages})
	}
	return ImportPreview{Summary: sum, Rows: results}
}

// -- orkestrasi service (preview & commit) --

// repoScheduleLookup mengadaptasi Repository (di dalam satu sekolah & tahun
// ajaran tertentu) menjadi scheduleImportLookup dipakai parseSlotRows.
type repoScheduleLookup struct {
	ctx      context.Context
	repo     scheduleRepository
	schoolID int64
	yearID   int64
}

func (l repoScheduleLookup) ClassIDByName(name string) (int64, bool) {
	id, ok, err := l.repo.LookupClassIDByName(l.ctx, l.schoolID, l.yearID, name)
	if err != nil {
		return 0, false
	}
	return id, ok
}

func (l repoScheduleLookup) SubjectByCode(code string) (int64, string, bool) {
	ref, ok, err := l.repo.LookupSubjectByCode(l.ctx, l.schoolID, code)
	if err != nil || !ok {
		return 0, "", false
	}
	return ref.ID, ref.Name, true
}

func (l repoScheduleLookup) TeacherByEmail(email string) (int64, string, bool) {
	ref, ok, err := l.repo.LookupTeacherByEmail(l.ctx, l.schoolID, email)
	if err != nil || !ok {
		return 0, "", false
	}
	return ref.ID, ref.Name, true
}

func (l repoScheduleLookup) RoomIDByName(name string) (int64, bool) {
	id, ok, err := l.repo.LookupRoomIDByName(l.ctx, l.schoolID, name)
	if err != nil {
		return 0, false
	}
	return id, ok
}

// PreviewScheduleImport mem-parse & memvalidasi file (xlsx/csv), menandai
// baris bentrok, menyimpan hasilnya di importStore (TTL 15 menit), dan
// mengembalikan preview.
func (s *Service) PreviewScheduleImport(ctx context.Context, schoolID int64, filename string, content []byte) (ImportPreview, error) {
	yearID, err := s.resolveYear(ctx, schoolID, 0)
	if err != nil {
		return ImportPreview{}, err
	}
	records, err := readRecords(filename, content)
	if err != nil {
		return ImportPreview{}, httpx.Validation(err.Error())
	}
	lookup := repoScheduleLookup{ctx: ctx, repo: s.repo, schoolID: schoolID, yearID: yearID}
	rows, err := parseSlotRows(records, lookup)
	if err != nil {
		return ImportPreview{}, httpx.Validation(err.Error())
	}

	existing, err := s.repo.ListSlotsForYear(ctx, schoolID, yearID)
	if err != nil {
		return ImportPreview{}, err
	}
	rows = markImportConflicts(rows, existing)

	uploadID, err := s.imports.put(rows, yearID)
	if err != nil {
		return ImportPreview{}, err
	}
	preview := buildSlotPreview(rows)
	preview.UploadID = uploadID
	return preview, nil
}

// CommitScheduleImport menerapkan hasil preview (upload_id): baris error
// dilewati, baris valid ditulis sebagai schedule_slots baru dalam satu
// transaksi (CreateSlotsBatch). Satu entri audit_log ringkasan.
func (s *Service) CommitScheduleImport(ctx context.Context, actorUserID, schoolID int64, uploadID string) (created, skipped int, err error) {
	rows, yearID, ok := s.imports.get(uploadID)
	if !ok {
		return 0, 0, httpx.Validation("Sesi import tidak ditemukan atau sudah kedaluwarsa. Silakan upload ulang.")
	}

	ins := make([]SlotInput, 0, len(rows))
	for _, r := range rows {
		if r.Action != "ok" {
			skipped++
			continue
		}
		ins = append(ins, SlotInput{
			SchoolID: schoolID, AcademicYearID: yearID, ClassID: r.ClassID, SubjectID: r.SubjectID,
			TeacherID: r.TeacherID, RoomID: r.RoomID, DayOfWeek: r.DayOfWeek, PeriodStart: r.PeriodStart, PeriodEnd: r.PeriodEnd,
		})
	}

	createdRecs, err := s.repo.CreateSlotsBatch(ctx, ins)
	if err != nil {
		return 0, 0, err
	}
	created = len(createdRecs)
	s.audit(ctx, schoolID, actorUserID, "schedule.import_commit", "schedule_slot", 0, nil,
		map[string]any{"created": created, "skipped": skipped, "total": len(rows)})
	s.emitSchedule(schoolID)
	return created, skipped, nil
}
