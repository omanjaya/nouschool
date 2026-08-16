package grading

import (
	"bytes"
	"context"
	"fmt"

	"github.com/xuri/excelize/v2"
)

// Berkas ini mengurus export Excel rekap nilai
// (GET /api/grading/export?class_id=&subject_id=) — pola DUA LAPIS yang sama
// dengan internal/discipline/export.go: renderRecapXLSX adalah fungsi MURNI
// (RecapView -> bytes .xlsx via excelize), Service.ExportXLSX hanya
// menjembatani dengan Service.Recap (SATU sumber data, tidak menduplikasi
// query/normalisasi).

// renderRecapXLSX adalah fungsi MURNI (tanpa I/O) — satu sheet, kolom
// NIS/Nama + satu kolom per komponen (header menyertakan bobot, docs tugas
// "header bobot") + Nilai Akhir + Label, header bold, freeze baris header.
func renderRecapXLSX(recap RecapView) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	const sheet = "Rekap Nilai"
	f.SetSheetName("Sheet1", sheet)

	headerStyle, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	headers := []string{"NIS", "Nama"}
	for _, c := range recap.Components {
		headers = append(headers, fmt.Sprintf("%s (bobot %d)", c.Name, c.Weight))
	}
	headers = append(headers, "Nilai Akhir", "Label")
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
		_ = f.SetCellStyle(sheet, cell, cell, headerStyle)
	}

	for r, st := range recap.Students {
		rowNum := r + 2
		values := []any{st.NIS, st.Name}
		for _, c := range recap.Components {
			if v := st.Scores[c.ID]; v != nil {
				values = append(values, *v)
			} else {
				values = append(values, "")
			}
		}
		if st.Final != nil {
			values = append(values, *st.Final)
		} else {
			values = append(values, "")
		}
		if st.Label != nil {
			values = append(values, *st.Label)
		} else {
			values = append(values, "")
		}
		for c, v := range values {
			cell, _ := excelize.CoordinatesToCellName(c+1, rowNum)
			_ = f.SetCellValue(sheet, cell, v)
		}
	}

	_ = f.SetColWidth(sheet, "A", "A", 12)
	_ = f.SetColWidth(sheet, "B", "B", 24)
	if err := f.SetPanes(sheet, &excelize.Panes{Freeze: true, XSplit: 0, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ExportXLSX — GET /api/grading/export?class_id=&subject_id= (gerbang SAMA
// dengan Service.Recap — panggil method itu langsung, TIDAK menduplikasi
// query/normalisasi).
func (s *Service) ExportXLSX(ctx context.Context, schoolID int64, q RecapQuery) (filename string, data []byte, err error) {
	recap, err := s.Recap(ctx, schoolID, q)
	if err != nil {
		return "", nil, err
	}
	buf, err := renderRecapXLSX(recap)
	if err != nil {
		return "", nil, err
	}
	className := fmt.Sprintf("Kelas %d", q.ClassID)
	if class, ok, cerr := s.repo.GetClassByID(ctx, schoolID, q.ClassID); cerr == nil && ok {
		className = class.Name
	}
	subjectName := fmt.Sprintf("Mapel %d", q.SubjectID)
	if subject, ok, serr := s.repo.GetSubjectByID(ctx, schoolID, q.SubjectID); serr == nil && ok {
		subjectName = subject.Name
	}
	filename = fmt.Sprintf("Nilai %s %s.xlsx", className, subjectName)
	return filename, buf, nil
}
