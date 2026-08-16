package billing

import (
	"bytes"
	"context"
	"fmt"

	"github.com/xuri/excelize/v2"
)

// Berkas ini mengurus export Excel laporan pendapatan (fase 13 Gelombang 2,
// GET /api/admin/revenue/export?year=) — pola SAMA dengan
// internal/attendance/export.go (renderRevenueXLSX murni, hanya menyusun
// workbook di memori dari struct sudah diagregasi buildRevenueReport di
// service.go; Service.ExportRevenueXLSX hanya menjembatani repo + agregasi +
// render). Dua sheet: "Ringkasan" (total tahun + rekap bulanan + rekap per
// plan) dan "Invoice" (satu baris per invoice paid: number, sekolah, plan,
// amount, paid_at).

var monthNamesIDRevenue = [...]string{
	"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember",
}

func monthLabelIDRevenue(m int) string {
	if m < 1 || m > 12 {
		return ""
	}
	return monthNamesIDRevenue[m]
}

// renderRevenueXLSX adalah fungsi MURNI — tidak menyentuh repo/HTTP.
func renderRevenueXLSX(report RevenueReport, rows []RevenueInvoiceRow) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	headerStyle, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}

	// -- sheet 1: Ringkasan --
	summarySheet := "Ringkasan"
	f.SetSheetName("Sheet1", summarySheet)

	_ = f.SetCellValue(summarySheet, "A1", "Tahun")
	_ = f.SetCellValue(summarySheet, "B1", report.Year)
	_ = f.SetCellValue(summarySheet, "A2", "Total Pendapatan")
	_ = f.SetCellValue(summarySheet, "B2", report.Total)

	row := 4
	setHeaderRow := func(headers ...string) {
		for i, h := range headers {
			cell, _ := excelize.CoordinatesToCellName(i+1, row)
			_ = f.SetCellValue(summarySheet, cell, h)
			_ = f.SetCellStyle(summarySheet, cell, cell, headerStyle)
		}
		row++
	}

	setHeaderRow("Bulan", "Total Dibayar", "Jumlah Invoice")
	for _, m := range report.Months {
		_ = f.SetCellValue(summarySheet, fmt.Sprintf("A%d", row), monthLabelIDRevenue(m.Month))
		_ = f.SetCellValue(summarySheet, fmt.Sprintf("B%d", row), m.PaidTotal)
		_ = f.SetCellValue(summarySheet, fmt.Sprintf("C%d", row), m.InvoiceCount)
		row++
	}

	row++ // baris kosong pemisah
	setHeaderRow("Plan", "Total Dibayar", "Jumlah Invoice")
	for _, bp := range report.ByPlan {
		_ = f.SetCellValue(summarySheet, fmt.Sprintf("A%d", row), bp.PlanCode)
		_ = f.SetCellValue(summarySheet, fmt.Sprintf("B%d", row), bp.PaidTotal)
		_ = f.SetCellValue(summarySheet, fmt.Sprintf("C%d", row), bp.InvoiceCount)
		row++
	}
	_ = f.SetColWidth(summarySheet, "A", "A", 20)

	// -- sheet 2: Invoice --
	invSheet := "Invoice"
	if _, err := f.NewSheet(invSheet); err != nil {
		return nil, err
	}
	invHeaders := []string{"Nomor", "Sekolah", "Plan", "Jumlah", "Dibayar Pada"}
	for i, h := range invHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(invSheet, cell, h)
		_ = f.SetCellStyle(invSheet, cell, cell, headerStyle)
	}
	for i, inv := range rows {
		r := i + 2
		_ = f.SetCellValue(invSheet, fmt.Sprintf("A%d", r), inv.Number)
		_ = f.SetCellValue(invSheet, fmt.Sprintf("B%d", r), inv.SchoolName)
		_ = f.SetCellValue(invSheet, fmt.Sprintf("C%d", r), inv.PlanCode)
		_ = f.SetCellValue(invSheet, fmt.Sprintf("D%d", r), inv.Amount)
		_ = f.SetCellValue(invSheet, fmt.Sprintf("E%d", r), inv.PaidAt.Format(dateLayout))
	}
	_ = f.SetColWidth(invSheet, "B", "B", 24)

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ExportRevenueXLSX — GET /api/admin/revenue/export?year=YYYY (default tahun
// berjalan).
func (s *Service) ExportRevenueXLSX(ctx context.Context, year int) (filename string, data []byte, err error) {
	year, from, to := s.revenueYearRange(year)
	rows, err := s.repo.ListPaidInvoicesInRange(ctx, from, to)
	if err != nil {
		return "", nil, err
	}
	report := buildRevenueReport(year, rows)
	buf, err := renderRevenueXLSX(report, rows)
	if err != nil {
		return "", nil, err
	}
	filename = fmt.Sprintf("Pendapatan %d.xlsx", year)
	return filename, buf, nil
}
