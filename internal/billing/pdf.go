package billing

import (
	"bytes"
	"fmt"

	"github.com/go-pdf/fpdf"
)

// rupiah memformat rupiah kasar (tanpa desimal, pemisah ribuan titik) —
// cukup untuk kebutuhan kuitansi PDF, tanpa dependency i18n tambahan.
func rupiah(amount int64) string {
	s := fmt.Sprintf("%d", amount)
	neg := false
	if len(s) > 0 && s[0] == '-' {
		neg = true
		s = s[1:]
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, c)
	}
	res := "Rp " + string(out)
	if neg {
		res = "-" + res
	}
	return res
}

// InvoicePDFData adalah data yang dibutuhkan BuildInvoicePDF (dikumpulkan
// handler.go dari InvoiceRecord + nama sekolah via SchoolGateway).
type InvoicePDFData struct {
	Number      string
	SchoolName  string
	PlanCode    string
	PlanName    string
	MaxStudents int
	Amount      int64
	Status      string
	DueAt       string
	PaidAt      string
}

var invoiceStatusLabel = map[string]string{
	InvoiceUnpaid:               "Belum Dibayar",
	InvoiceAwaitingVerification: "Menunggu Verifikasi",
	InvoicePaid:                 "Lunas",
	InvoiceVoid:                 "Dibatalkan",
	InvoiceExpired:              "Kedaluwarsa",
}

// BuildInvoicePDF menyusun PDF invoice sederhana (docs/09-billing.md scope
// Fase 10: "kop NouSchool, nomor, sekolah, rincian plan+bracket, jumlah,
// status, rekening pembayaran placeholder").
func BuildInvoicePDF(data InvoicePDFData) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 18)
	pdf.CellFormat(0, 10, "NouSchool", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, "Platform Manajemen Sekolah", "", 1, "L", false, 0, "")
	pdf.Ln(4)
	pdf.SetDrawColor(200, 200, 200)
	y := pdf.GetY()
	pdf.Line(20, y, 190, y)
	pdf.Ln(8)

	pdf.SetFont("Helvetica", "B", 14)
	pdf.CellFormat(0, 8, "INVOICE "+data.Number, "", 1, "L", false, 0, "")
	pdf.Ln(2)

	row := func(label, value string) {
		pdf.SetFont("Helvetica", "B", 10)
		pdf.CellFormat(45, 7, label, "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 10)
		pdf.CellFormat(0, 7, value, "", 1, "L", false, 0, "")
	}

	row("Sekolah", data.SchoolName)
	row("Paket", fmt.Sprintf("%s (%s)", data.PlanName, data.PlanCode))
	row("Bracket Siswa", fmt.Sprintf("hingga %d siswa", data.MaxStudents))
	row("Jumlah Tagihan", rupiah(data.Amount))
	row("Status", statusLabel(data.Status))
	row("Jatuh Tempo", data.DueAt)
	if data.PaidAt != "" {
		row("Dibayar Pada", data.PaidAt)
	}

	pdf.Ln(8)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(0, 7, "Rekening Pembayaran (Transfer Manual)", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.MultiCell(0, 6, "Bank BCA 1234567890 a.n. PT NouSchool Indonesia\n"+
		"Konfirmasi pembayaran dengan mengunggah bukti transfer di panel Billing sekolah Anda.", "", "L", false)

	pdf.Ln(6)
	pdf.SetFont("Helvetica", "I", 8)
	pdf.MultiCell(0, 5, "Invoice ini diterbitkan otomatis oleh sistem NouSchool dan sah digunakan sebagai lampiran SPJ BOS tanpa tanda tangan basah.", "", "L", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func statusLabel(status string) string {
	if l, ok := invoiceStatusLabel[status]; ok {
		return l
	}
	return status
}
