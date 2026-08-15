package attendance

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
)

// Berkas ini mengurus export Excel rekap absensi bulanan (Fase 11,
// GET /api/attendance/export?month=YYYY-MM&class_id=). Dipisah jadi dua
// lapisan supaya bisa dites tanpa DB (docs tugas: "boleh test level service
// menyusun matrix"): buildMonthlyMatrices (murni, dari []MonthlyRecordRow ->
// struktur per-kelas/siswa/tanggal) dan renderMonthlyXLSX (murni, struktur ->
// bytes .xlsx via excelize) — Service.ExportMonthlyXLSX hanya menjembatani
// keduanya dengan repo/parameter HTTP.

// statusAbbrev — kode satu huruf per status kanonik, dipakai isi sel harian.
var statusAbbrev = map[string]string{
	StatusHadir:     "H",
	StatusTerlambat: "T",
	StatusIzin:      "I",
	StatusSakit:     "S",
	StatusAlpa:      "A",
}

// statusOrder — urutan kolom ringkasan (H, T, I, S, A) di ujung kanan sheet.
var statusOrder = []string{StatusHadir, StatusTerlambat, StatusIzin, StatusSakit, StatusAlpa}

var statusColumnLabel = map[string]string{
	StatusHadir:     "Hadir",
	StatusTerlambat: "Terlambat",
	StatusIzin:      "Izin",
	StatusSakit:     "Sakit",
	StatusAlpa:      "Alpa",
}

// monthlyStudentRow — satu baris siswa pada satu sheet (kelas).
type monthlyStudentRow struct {
	StudentID int64
	Name      string
	NIS       string
	ByDay     map[int]string // hari-dalam-bulan (1..31) -> kode status
}

// classMonthlyMatrix — satu sheet (satu rombel).
type classMonthlyMatrix struct {
	ClassID   int64
	ClassName string
	Students  []monthlyStudentRow
}

// buildMonthlyMatrices adalah fungsi MURNI (tanpa I/O) yang mengelompokkan
// baris mentah repo (satu baris per siswa per tanggal sesi) menjadi satu
// matriks per rombel, urutan kemunculan pertama dipertahankan (rows dari
// repo SUDAH terurut nama kelas -> nama siswa -> tanggal, lihat queries.sql
// MonthlyAttendanceRecords). Ditest langsung tanpa DB — lihat export_test.go.
func buildMonthlyMatrices(rows []MonthlyRecordRow) []classMonthlyMatrix {
	classOrder := make([]int64, 0)
	byClass := make(map[int64]*classMonthlyMatrix)
	studentIdx := make(map[int64]map[int64]int) // classID -> studentID -> index di byClass[classID].Students

	for _, r := range rows {
		cm, ok := byClass[r.ClassID]
		if !ok {
			cm = &classMonthlyMatrix{ClassID: r.ClassID, ClassName: r.ClassName}
			byClass[r.ClassID] = cm
			classOrder = append(classOrder, r.ClassID)
			studentIdx[r.ClassID] = make(map[int64]int)
		}
		si, ok := studentIdx[r.ClassID][r.StudentID]
		if !ok {
			cm.Students = append(cm.Students, monthlyStudentRow{
				StudentID: r.StudentID, Name: r.StudentName, NIS: r.StudentNIS, ByDay: make(map[int]string),
			})
			si = len(cm.Students) - 1
			studentIdx[r.ClassID][r.StudentID] = si
		}
		if !r.Date.IsZero() && r.Status != "" {
			if abbr, ok := statusAbbrev[r.Status]; ok {
				cm.Students[si].ByDay[r.Date.Day()] = abbr
			}
		}
	}

	out := make([]classMonthlyMatrix, 0, len(classOrder))
	for _, id := range classOrder {
		out = append(out, *byClass[id])
	}
	return out
}

var invalidSheetChars = regexp.MustCompile(`[\[\]:*?/\\]`)

// sanitizeSheetName — nama sheet Excel dilarang berisi []:*?/\ dan maksimal
// 31 karakter.
func sanitizeSheetName(name string) string {
	name = invalidSheetChars.ReplaceAllString(name, " ")
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Kelas"
	}
	if len(name) > 31 {
		name = name[:31]
	}
	return name
}

// renderMonthlyXLSX adalah fungsi MURNI (hanya menyusun workbook di memori,
// tidak menyentuh repo/HTTP) — satu sheet per rombel, kolom tanggal 1..N hari
// dalam bulan, kolom ringkasan total per status, header bold, freeze pane
// (baris header + 2 kolom NIS/Nama) supaya tetap terlihat saat scroll.
func renderMonthlyXLSX(matrices []classMonthlyMatrix, month time.Time) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	daysInMonth := time.Date(month.Year(), month.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	headerStyle, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}

	if len(matrices) == 0 {
		matrices = []classMonthlyMatrix{{ClassName: "Absensi"}}
	}

	usedNames := make(map[string]int)
	for i, m := range matrices {
		sheetName := sanitizeSheetName(m.ClassName)
		if n := usedNames[sheetName]; n > 0 {
			suffix := fmt.Sprintf(" (%d)", n+1)
			base := sheetName
			if len(base)+len(suffix) > 31 {
				base = base[:31-len(suffix)]
			}
			sheetName = base + suffix
		}
		usedNames[sheetName]++

		if i == 0 {
			f.SetSheetName("Sheet1", sheetName)
		} else {
			if _, err := f.NewSheet(sheetName); err != nil {
				return nil, err
			}
		}

		// -- header --
		col := 1
		setHeaderCell := func(text string) {
			cell, _ := excelize.CoordinatesToCellName(col, 1)
			_ = f.SetCellValue(sheetName, cell, text)
			_ = f.SetCellStyle(sheetName, cell, cell, headerStyle)
			col++
		}
		setHeaderCell("NIS")
		setHeaderCell("Nama")
		for d := 1; d <= daysInMonth; d++ {
			setHeaderCell(strconv.Itoa(d))
		}
		for _, st := range statusOrder {
			setHeaderCell(statusColumnLabel[st])
		}

		// -- baris siswa --
		for r, st := range m.Students {
			row := r + 2
			col = 1
			setCell := func(v any) {
				cell, _ := excelize.CoordinatesToCellName(col, row)
				_ = f.SetCellValue(sheetName, cell, v)
				col++
			}
			setCell(st.NIS)
			setCell(st.Name)
			counts := map[string]int{}
			for d := 1; d <= daysInMonth; d++ {
				code := st.ByDay[d]
				setCell(code)
				for status, abbr := range statusAbbrev {
					if abbr == code {
						counts[status]++
					}
				}
			}
			for _, status := range statusOrder {
				setCell(counts[status])
			}
		}

		_ = f.SetColWidth(sheetName, "A", "A", 14)
		_ = f.SetColWidth(sheetName, "B", "B", 24)
		if err := f.SetPanes(sheetName, &excelize.Panes{
			Freeze: true, Split: false, XSplit: 2, YSplit: 1, TopLeftCell: "C2", ActivePane: "bottomRight",
		}); err != nil {
			return nil, err
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var monthNamesID = [...]string{
	"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember",
}

func monthLabelID(t time.Time) string {
	return fmt.Sprintf("%s %d", monthNamesID[t.Month()], t.Year())
}

// ExportMonthlyXLSX — GET /api/attendance/export?month=YYYY-MM&class_id=
// (perm attendance:report). class_id=0 -> semua rombel tahun ajaran aktif,
// satu sheet per rombel; class_id spesifik -> hanya sheet rombel itu.
func (s *Service) ExportMonthlyXLSX(ctx context.Context, schoolID int64, monthStr string, classID int64) (filename string, data []byte, err error) {
	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return "", nil, err
	}

	month, perr := time.Parse("2006-01", strings.TrimSpace(monthStr))
	if perr != nil {
		return "", nil, httpx.Validation("Format bulan harus YYYY-MM, contoh 2026-08.")
	}
	from := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, -1)

	classLabel := "Semua Kelas"
	if classID != 0 {
		cls, cerr := s.repo.GetClassMeta(ctx, schoolID, classID)
		if cerr != nil {
			if errors.Is(cerr, ErrNotFound) {
				return "", nil, httpx.Validation("Rombel tidak ditemukan.")
			}
			return "", nil, cerr
		}
		classLabel = cls.Name
	}

	rows, err := s.repo.MonthlyAttendanceRecords(ctx, schoolID, yearID, from, to, classID)
	if err != nil {
		return "", nil, err
	}

	matrices := buildMonthlyMatrices(rows)
	buf, err := renderMonthlyXLSX(matrices, from)
	if err != nil {
		return "", nil, err
	}

	filename = fmt.Sprintf("Absensi %s %s.xlsx", classLabel, monthLabelID(from))
	return filename, buf, nil
}
