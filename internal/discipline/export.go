package discipline

import (
	"bytes"
	"context"
	"fmt"

	"github.com/xuri/excelize/v2"
)

// Berkas ini mengurus export Excel rekap kedisiplinan
// (GET /api/discipline/export?class_id=&from=&to=) — pola DUA LAPIS yang
// sama dengan internal/attendance/export.go: renderSummaryXLSX adalah fungsi
// MURNI (SummaryRow -> bytes .xlsx via excelize), Service.ExportSummaryXLSX
// hanya menjembatani dengan Service.Summary (SATU sumber data, tidak
// menduplikasi query/agregasi).

// renderSummaryXLSX adalah fungsi MURNI (tanpa I/O) — satu sheet, kolom
// Nama/NIS/Kelas/Total Poin/Jumlah Catatan/Terakhir, header bold, freeze
// baris header, urut sesuai input (Service.Summary sudah urut poin desc).
func renderSummaryXLSX(rows []SummaryRow) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	const sheet = "Rekap Kedisiplinan"
	f.SetSheetName("Sheet1", sheet)

	headerStyle, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	headers := []string{"NIS", "Nama", "Kelas", "Total Poin", "Jumlah Catatan", "Terakhir"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
		_ = f.SetCellStyle(sheet, cell, cell, headerStyle)
	}

	for r, row := range rows {
		rowNum := r + 2
		last := ""
		if row.LastAt != nil {
			last = row.LastAt.Format("2006-01-02")
		}
		values := []any{row.Student.NIS, row.Student.Name, row.Student.ClassName, row.TotalPoints, row.RecordCount, last}
		for c, v := range values {
			cell, _ := excelize.CoordinatesToCellName(c+1, rowNum)
			_ = f.SetCellValue(sheet, cell, v)
		}
	}

	_ = f.SetColWidth(sheet, "A", "A", 12)
	_ = f.SetColWidth(sheet, "B", "B", 24)
	_ = f.SetColWidth(sheet, "C", "C", 14)
	if err := f.SetPanes(sheet, &excelize.Panes{Freeze: true, XSplit: 0, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ExportSummaryXLSX — GET /api/discipline/export?class_id=&from=&to= (perm
// discipline:read, gerbang SAMA dengan Service.Summary — panggil method itu
// langsung, TIDAK menduplikasi query).
func (s *Service) ExportSummaryXLSX(ctx context.Context, schoolID int64, q SummaryQuery) (filename string, data []byte, err error) {
	rows, err := s.Summary(ctx, schoolID, q)
	if err != nil {
		return "", nil, err
	}
	buf, err := renderSummaryXLSX(rows)
	if err != nil {
		return "", nil, err
	}
	label := "Semua Rentang"
	if q.From != "" || q.To != "" {
		label = fmt.Sprintf("%s_%s", orDash(q.From), orDash(q.To))
	}
	filename = fmt.Sprintf("Kedisiplinan %s.xlsx", label)
	return filename, buf, nil
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
