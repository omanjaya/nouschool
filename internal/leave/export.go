package leave

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// Berkas ini mengurus export Excel daftar izin (Fase 11,
// GET /api/leave/export?from=&to=). Sama pola dua-lapis dengan
// internal/attendance/export.go: approverNameFor & renderLeaveXLSX MURNI
// (dari []Request, tipe response yang SAMA dipakai GET /api/leave/requests —
// sudah lengkap dgn steps/approver, tidak butuh query baru selain filter
// rentang tanggal) supaya bisa dites tanpa DB/HTTP.

// approverNameFor mengembalikan nama approver TERAKHIR yang sudah memutuskan
// step (approved/rejected), atau "-" bila belum ada step yang diputuskan
// (masih menunggu, atau auto-approved tanpa step sama sekali).
func approverNameFor(req Request) string {
	var lastDecided *StepView
	for i := range req.Steps {
		if req.Steps[i].Decision != nil {
			lastDecided = &req.Steps[i]
		}
	}
	if lastDecided != nil && lastDecided.ApproverName != nil && *lastDecided.ApproverName != "" {
		return *lastDecided.ApproverName
	}
	return "-"
}

var leaveStatusLabel = map[string]string{
	StatusPending:  "Menunggu",
	StatusApproved: "Disetujui",
	StatusRejected: "Ditolak",
	StatusCanceled: "Dibatalkan",
}

func statusLabelID(status string) string {
	if l, ok := leaveStatusLabel[status]; ok {
		return l
	}
	return status
}

// renderLeaveXLSX adalah fungsi MURNI (hanya menyusun workbook di memori) —
// satu sheet, satu baris per pengajuan, kolom: guru, jenis, tanggal
// mulai/selesai, jumlah hari, status, disetujui/ditolak oleh. Header bold +
// freeze baris pertama (docs tugas: "Header rapi, freeze pane" — persyaratan
// eksplisit hanya untuk export absensi, diterapkan juga di sini untuk
// konsistensi UX kedua export).
func renderLeaveXLSX(reqs []Request) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	const sheet = "Izin"
	f.SetSheetName("Sheet1", sheet)

	headerStyle, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}

	headers := []string{"Guru", "Jenis Izin", "Tanggal Mulai", "Tanggal Selesai", "Jumlah Hari", "Status", "Disetujui/Ditolak Oleh"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
		_ = f.SetCellStyle(sheet, cell, cell, headerStyle)
	}

	for i, req := range reqs {
		row := i + 2
		values := []any{
			req.Teacher.Name, req.TypeLabel,
			req.DateStart.Format("2006-01-02"), req.DateEnd.Format("2006-01-02"),
			req.Days, statusLabelID(req.Status), approverNameFor(req),
		}
		for c, v := range values {
			cell, _ := excelize.CoordinatesToCellName(c+1, row)
			_ = f.SetCellValue(sheet, cell, v)
		}
	}

	for col, width := range map[string]float64{"A": 22, "B": 16, "C": 14, "D": 14, "E": 12, "F": 14, "G": 24} {
		_ = f.SetColWidth(sheet, col, col, width)
	}
	if err := f.SetPanes(sheet, &excelize.Panes{
		Freeze: true, Split: false, XSplit: 0, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft",
	}); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ExportXLSX — GET /api/leave/export?from=&to= (perm leave:manage, sama
// gerbang dengan Summary).
func (s *Service) ExportXLSX(ctx context.Context, schoolID int64, fromStr, toStr string) (filename string, data []byte, err error) {
	role := reqctx.Role(ctx)
	if !s.identity.HasPermission(role, PermLeaveManage) && role != RoleKepalaSekolah {
		return "", nil, httpx.ErrForbidden
	}
	from, ferr := parseDate(fromStr)
	if ferr != nil {
		return "", nil, ferr
	}
	to, terr := parseDate(toStr)
	if terr != nil {
		return "", nil, terr
	}
	if from.After(to) {
		return "", nil, httpx.Validation("'from' tidak boleh setelah 'to'.")
	}

	recs, err := s.repo.ListRequestsInRange(ctx, schoolID, from, to)
	if err != nil {
		return "", nil, err
	}
	views, err := s.buildRequestViews(ctx, schoolID, recs)
	if err != nil {
		return "", nil, err
	}

	buf, err := renderLeaveXLSX(views)
	if err != nil {
		return "", nil, err
	}
	filename = fmt.Sprintf("Izin %s_%s.xlsx", strings.ReplaceAll(from.Format("2006-01-02"), "-", ""), strings.ReplaceAll(to.Format("2006-01-02"), "-", ""))
	return filename, buf, nil
}
