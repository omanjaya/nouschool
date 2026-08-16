package discipline

import (
	"bytes"

	"github.com/go-pdf/fpdf"
)

// levelLabel — label Indonesia per level SP (dipakai PDF & HTML).
func levelLabel(level int) string {
	switch level {
	case 1:
		return "Surat Peringatan Pertama (SP1)"
	case 2:
		return "Surat Peringatan Kedua (SP2)"
	case 3:
		return "Surat Peringatan Ketiga (SP3)"
	default:
		return "Surat Peringatan"
	}
}

var monthNamesID = [...]string{
	"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember",
}

// indonesianDate memformat "YYYY-MM-DD" -> "15 Agustus 2026" (fallback ke
// nilai asli bila format tidak dikenali — snapshot lama seharusnya SELALU
// valid, ini murni jaga-jaga).
func indonesianDate(iso string) string {
	if len(iso) != 10 || iso[4] != '-' || iso[7] != '-' {
		return iso
	}
	year, month, day := iso[0:4], iso[5:7], iso[8:10]
	m := 0
	for i, c := range month {
		_ = i
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

// BuildLetterPDF menyusun PDF surat peringatan — DIRENDER dari snapshot
// (docs/12-sion-parity.md: "surat = potret saat terbit"), pola yang sama
// dengan internal/billing/pdf.go (fpdf, satu halaman A4). footerNote (Fase 14
// Gelombang D, school_settings module "letters") ditambahkan di BAGIAN BAWAH
// halaman BILA NON-KOSONG — "" (default) = tidak ada perubahan tampilan sama
// sekali dibanding sebelum Gelombang D ini ada.
func BuildLetterPDF(letter LetterRecord, snap LetterSnapshot, footerNote string) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 8, snap.SchoolName, "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, "Surat Peringatan Kedisiplinan Siswa", "", 1, "C", false, 0, "")
	pdf.Ln(2)
	pdf.SetDrawColor(180, 180, 180)
	y := pdf.GetY()
	pdf.Line(20, y, 190, y)
	pdf.Ln(8)

	pdf.SetFont("Helvetica", "B", 14)
	pdf.CellFormat(0, 8, levelLabel(letter.Level), "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, "Nomor: "+letter.Number, "", 1, "C", false, 0, "")
	pdf.Ln(6)

	row := func(label, value string) {
		pdf.SetFont("Helvetica", "B", 10)
		pdf.CellFormat(45, 7, label, "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 10)
		pdf.CellFormat(0, 7, value, "", 1, "L", false, 0, "")
	}
	row("Nama Siswa", snap.Student.Name)
	row("NIS", snap.Student.NIS)
	row("Kelas", snap.Student.ClassName)
	row("Total Poin", itoaSimple(snap.Total)+" (ambang "+itoaSimple(snap.Threshold)+")")
	pdf.Ln(4)

	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(0, 7, "Rincian Pelanggaran", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "B", 9)
	pdf.CellFormat(90, 7, "Jenis Pelanggaran", "1", 0, "L", false, 0, "")
	pdf.CellFormat(30, 7, "Poin", "1", 0, "C", false, 0, "")
	pdf.CellFormat(50, 7, "Tanggal", "1", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	for _, v := range snap.Violations {
		pdf.CellFormat(90, 7, v.Name, "1", 0, "L", false, 0, "")
		pdf.CellFormat(30, 7, itoaSimple(v.Points), "1", 0, "C", false, 0, "")
		pdf.CellFormat(50, 7, indonesianDate(v.Date), "1", 1, "C", false, 0, "")
	}

	pdf.Ln(10)
	pdf.SetFont("Helvetica", "", 10)
	pdf.MultiCell(0, 6, "Surat ini diterbitkan otomatis oleh sistem berdasarkan akumulasi poin pelanggaran siswa yang telah melewati ambang "+levelLabel(letter.Level)+". Mohon perhatian orang tua/wali untuk membina kedisiplinan siswa.", "", "L", false)

	pdf.Ln(10)
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, snap.SchoolName+", "+indonesianDate(snap.IssuedOn), "", 1, "R", false, 0, "")
	pdf.Ln(2)
	pdf.CellFormat(0, 6, "Kepala Sekolah", "", 1, "R", false, 0, "")

	if footerNote != "" {
		pdf.Ln(10)
		pdf.SetDrawColor(180, 180, 180)
		y := pdf.GetY()
		pdf.Line(20, y, 190, y)
		pdf.Ln(4)
		pdf.SetFont("Helvetica", "I", 9)
		pdf.MultiCell(0, 5, footerNote, "", "L", false)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
