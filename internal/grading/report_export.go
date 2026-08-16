package grading

import (
	"bytes"
	"context"
	"fmt"

	"github.com/xuri/excelize/v2"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
)

// Berkas ini mengurus export Excel rapor kelas (GET
// /api/grading/report/export?class_id= — Fase 15 GAP 1): SATU sheet berisi
// baris siswa x kolom SEMUA mapel yang punya komponen penilaian di kelas itu
// (nilai rapor TER-RESOLUSI, manual menang atas computed final, + label
// rentang) + kolom rata-rata di akhir, dan sheet KEDUA "TP" berisi pemetaan
// TP per mapel. Pola DUA LAPIS yang sama dengan export.go: renderReportXLSX
// adalah fungsi MURNI, Service.ReportExportXLSX menjembatani ke repository.

func renderReportXLSX(subjects []SubjectRef, roster []RosterStudent,
	resolvedBySubject map[int64]map[int64]reportResolution, ranges []GradeRange, tpMappings []TPMappingWithSubject) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	const sheetRapor = "Rapor Kelas"
	f.SetSheetName("Sheet1", sheetRapor)
	headerStyle, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}

	headers := []string{"NIS", "Nama"}
	for _, sub := range subjects {
		headers = append(headers, sub.Name)
	}
	headers = append(headers, "Rata-rata")
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheetRapor, cell, h)
		_ = f.SetCellStyle(sheetRapor, cell, cell, headerStyle)
	}

	for r, st := range roster {
		rowNum := r + 2
		values := []any{st.NIS, st.Name}
		var sum float64
		var count int
		for _, sub := range subjects {
			res, ok := resolvedBySubject[sub.ID][st.ID]
			if !ok || res.Resolved == nil {
				values = append(values, "")
				continue
			}
			v := *res.Resolved
			label := labelForScore(ranges, roundHalfUp(v))
			cellText := fmt.Sprintf("%.2f", v)
			if label != nil {
				cellText = fmt.Sprintf("%.2f (%s)", v, *label)
			}
			values = append(values, cellText)
			sum += v
			count++
		}
		if count > 0 {
			values = append(values, fmt.Sprintf("%.2f", sum/float64(count)))
		} else {
			values = append(values, "")
		}
		for c, v := range values {
			cell, _ := excelize.CoordinatesToCellName(c+1, rowNum)
			_ = f.SetCellValue(sheetRapor, cell, v)
		}
	}
	_ = f.SetColWidth(sheetRapor, "A", "A", 12)
	_ = f.SetColWidth(sheetRapor, "B", "B", 24)
	if err := f.SetPanes(sheetRapor, &excelize.Panes{Freeze: true, XSplit: 0, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return nil, err
	}

	const sheetTP = "TP"
	if _, err := f.NewSheet(sheetTP); err != nil {
		return nil, err
	}
	tpHeaders := []string{"Mapel", "Komponen", "Kode TP", "Deskripsi"}
	for i, h := range tpHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheetTP, cell, h)
		_ = f.SetCellStyle(sheetTP, cell, cell, headerStyle)
	}
	for r, m := range tpMappings {
		rowNum := r + 2
		values := []any{m.SubjectName, m.ComponentName, m.TPCode, m.Description}
		for c, v := range values {
			cell, _ := excelize.CoordinatesToCellName(c+1, rowNum)
			_ = f.SetCellValue(sheetTP, cell, v)
		}
	}
	_ = f.SetColWidth(sheetTP, "A", "B", 22)
	_ = f.SetColWidth(sheetTP, "D", "D", 40)

	f.SetActiveSheet(0)

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ReportExportXLSX — GET /api/grading/report/export?class_id= (guard toggle
// grading + requireManage + object-level KELAS, bukan per-subject — pola
// sama bintang kelas requireClassAccess: admin bebas, guru boleh bila punya
// slot jadwal di kelas ini mapel APA PUN, karena satu file merangkum
// SELURUH mapel kelas).
func (s *Service) ReportExportXLSX(ctx context.Context, schoolID, classID int64) (filename string, data []byte, err error) {
	if _, err := s.requireEnabled(ctx, schoolID); err != nil {
		return "", nil, err
	}
	if err := s.requireManage(ctx); err != nil {
		return "", nil, err
	}
	if classID == 0 {
		return "", nil, httpx.Validation("class_id wajib diisi.")
	}
	if err := s.requireClassAccess(ctx, schoolID, classID); err != nil {
		return "", nil, err
	}
	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return "", nil, err
	}
	subjects, err := s.repo.ListSubjectsWithComponentsForClass(ctx, schoolID, classID)
	if err != nil {
		return "", nil, err
	}
	roster, err := s.repo.ListClassRoster(ctx, schoolID, classID)
	if err != nil {
		return "", nil, err
	}
	settings, err := s.loadSettings(ctx, schoolID)
	if err != nil {
		return "", nil, err
	}

	resolvedBySubject := make(map[int64]map[int64]reportResolution, len(subjects))
	for _, sub := range subjects {
		components, cerr := s.repo.ListComponentsRaw(ctx, schoolID, classID, sub.ID)
		if cerr != nil {
			return "", nil, cerr
		}
		resolutions, rerr := s.buildReportResolutions(ctx, schoolID, yearID, classID, sub.ID, components, roster)
		if rerr != nil {
			return "", nil, rerr
		}
		resolvedBySubject[sub.ID] = resolutions
	}
	tpMappings, err := s.repo.ListTPMappingsForClass(ctx, schoolID, classID)
	if err != nil {
		return "", nil, err
	}

	buf, err := renderReportXLSX(subjects, roster, resolvedBySubject, settings.Ranges, tpMappings)
	if err != nil {
		return "", nil, err
	}
	className := fmt.Sprintf("Kelas %d", classID)
	if class, ok, cerr := s.repo.GetClassByID(ctx, schoolID, classID); cerr == nil && ok {
		className = class.Name
	}
	filename = fmt.Sprintf("Rapor %s.xlsx", className)
	return filename, buf, nil
}
