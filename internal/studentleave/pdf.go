package studentleave

import (
	"bytes"

	"github.com/go-pdf/fpdf"
	"github.com/skip2/go-qrcode"
)

var monthNamesID = [...]string{
	"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember",
}

func itoaSimple(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

// indonesianDate memformat "YYYY-MM-DD" -> "15 Agustus 2026" (pola sama
// internal/discipline/pdf.go).
func indonesianDate(iso string) string {
	if len(iso) != 10 || iso[4] != '-' || iso[7] != '-' {
		return iso
	}
	year, month, day := iso[0:4], iso[5:7], iso[8:10]
	m := 0
	for _, c := range month {
		m = m*10 + int(c-'0')
	}
	if m < 1 || m > 12 {
		return iso
	}
	d := 0
	for _, c := range day {
		d = d*10 + int(c-'0')
	}
	return itoaSimple(d) + " " + monthNamesID[m] + " " + year
}

func typeLabel(t string) string {
	if t == TypeSakit {
		return "Sakit"
	}
	return "Izin"
}

// qrPixelSize — pola sama internal/schedule/qr.go (QR ruangan).
const qrPixelSize = 300

// BuildLeavePDF menyusun PDF surat izin (fpdf, satu halaman A4): kop
// app_name, nomor surat, identitas siswa+kelas, jenis+rentang+alasan, jejak
// persetujuan (wali & BK + waktu), DAN QR code verifikasi (docs tugas: PNG
// embed berisi verifyURL + teks URL di bawahnya).
func BuildLeavePDF(d RequestDetail, appName, verifyURL string) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 8, appName, "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, "Surat Izin Siswa", "", 1, "C", false, 0, "")
	pdf.Ln(2)
	pdf.SetDrawColor(180, 180, 180)
	y := pdf.GetY()
	pdf.Line(20, y, 190, y)
	pdf.Ln(8)

	pdf.SetFont("Helvetica", "B", 13)
	pdf.CellFormat(0, 7, "Nomor: "+d.LetterNumber, "", 1, "C", false, 0, "")
	pdf.Ln(6)

	row := func(label, value string) {
		pdf.SetFont("Helvetica", "B", 10)
		pdf.CellFormat(45, 7, label, "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 10)
		pdf.CellFormat(0, 7, value, "", 1, "L", false, 0, "")
	}
	row("Nama Siswa", d.StudentName)
	row("NIS", d.StudentNIS)
	row("Kelas", d.ClassName)
	row("Jenis Izin", typeLabel(d.Type))
	row("Tanggal", indonesianDate(d.DateStart.Format("2006-01-02"))+" s.d. "+indonesianDate(d.DateEnd.Format("2006-01-02")))
	pdf.Ln(2)

	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(0, 7, "Alasan:", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.MultiCell(0, 6, d.Reason, "", "L", false)
	pdf.Ln(4)

	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(0, 7, "Jejak Persetujuan", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "B", 9)
	pdf.CellFormat(50, 7, "Tahap", "1", 0, "L", false, 0, "")
	pdf.CellFormat(70, 7, "Oleh", "1", 0, "L", false, 0, "")
	pdf.CellFormat(50, 7, "Waktu", "1", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.CellFormat(50, 7, "Wali Kelas", "1", 0, "L", false, 0, "")
	pdf.CellFormat(70, 7, d.HomeroomDecidedByName, "1", 0, "L", false, 0, "")
	homeroomTime := ""
	if !d.HomeroomDecidedAt.IsZero() {
		homeroomTime = d.HomeroomDecidedAt.Format("02-01-2006 15:04")
	}
	pdf.CellFormat(50, 7, homeroomTime, "1", 1, "L", false, 0, "")
	pdf.CellFormat(50, 7, "Guru BK", "1", 0, "L", false, 0, "")
	pdf.CellFormat(70, 7, d.BKDecidedByName, "1", 0, "L", false, 0, "")
	bkTime := ""
	if !d.BKDecidedAt.IsZero() {
		bkTime = d.BKDecidedAt.Format("02-01-2006 15:04")
	}
	pdf.CellFormat(50, 7, bkTime, "1", 1, "L", false, 0, "")

	pdf.Ln(8)
	qrPNG, err := qrcode.Encode(verifyURL, qrcode.Medium, qrPixelSize)
	if err == nil {
		reader := bytes.NewReader(qrPNG)
		opts := fpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}
		pdf.RegisterImageOptionsReader("leave-qr", opts, reader)
		qrY := pdf.GetY()
		pdf.ImageOptions("leave-qr", 20, qrY, 30, 30, false, opts, 0, "")
		pdf.SetXY(55, qrY+8)
		pdf.SetFont("Helvetica", "", 8)
		pdf.MultiCell(130, 5, "Pindai untuk verifikasi keaslian surat ini, atau kunjungi:\n"+verifyURL, "", "L", false)
		pdf.SetY(qrY + 32)
	}

	pdf.Ln(6)
	pdf.SetFont("Helvetica", "", 9)
	pdf.MultiCell(0, 5, "Surat ini diterbitkan oleh sistem setelah disetujui wali kelas dan Guru BK. Keasliannya dapat diverifikasi lewat QR/tautan di atas.", "", "L", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
