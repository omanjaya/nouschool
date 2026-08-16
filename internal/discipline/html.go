package discipline

import (
	"bytes"
	"html/template"
)

// letterHTMLTpl — HTML sederhana tanpa CSS eksternal (self-contained, sama
// prinsip dengan pola artefak/print lain di codebase ini: tanpa dependency
// CDN). DIRENDER dari snapshot (surat = potret saat terbit).
var letterHTMLTpl = template.Must(template.New("letter").Parse(`<!doctype html>
<html lang="id">
<head>
<meta charset="utf-8">
<title>Surat {{.Number}}</title>
<style>
body { font-family: Helvetica, Arial, sans-serif; color: #1a1a1a; max-width: 720px; margin: 40px auto; padding: 0 16px; }
h1 { text-align: center; font-size: 20px; margin-bottom: 4px; }
h2 { text-align: center; font-size: 14px; font-weight: normal; margin-top: 0; color: #444; }
h3 { text-align: center; font-size: 16px; margin-bottom: 2px; }
.number { text-align: center; margin-bottom: 20px; color: #444; }
table { width: 100%; border-collapse: collapse; margin-top: 8px; }
th, td { border: 1px solid #ccc; padding: 6px 8px; text-align: left; font-size: 13px; }
th { background: #f2f2f2; }
.info { margin: 16px 0; }
.info div { margin-bottom: 4px; }
.label { display: inline-block; width: 120px; font-weight: bold; }
.note { margin-top: 24px; font-size: 13px; }
.sign { margin-top: 40px; text-align: right; }
</style>
</head>
<body>
<h1>{{.SchoolName}}</h1>
<h2>Surat Peringatan Kedisiplinan Siswa</h2>
<h3>{{.LevelLabel}}</h3>
<div class="number">Nomor: {{.Number}}</div>

<div class="info">
<div><span class="label">Nama Siswa</span> : {{.Student.Name}}</div>
<div><span class="label">NIS</span> : {{.Student.NIS}}</div>
<div><span class="label">Kelas</span> : {{.Student.ClassName}}</div>
<div><span class="label">Total Poin</span> : {{.Total}} (ambang {{.Threshold}})</div>
</div>

<table>
<thead><tr><th>Jenis Pelanggaran</th><th>Poin</th><th>Tanggal</th></tr></thead>
<tbody>
{{range .Violations}}<tr><td>{{.Name}}</td><td>{{.Points}}</td><td>{{.DateLabel}}</td></tr>
{{end}}
</tbody>
</table>

<p class="note">Surat ini diterbitkan otomatis oleh sistem berdasarkan akumulasi poin pelanggaran siswa yang telah melewati ambang {{.LevelLabel}}. Mohon perhatian orang tua/wali untuk membina kedisiplinan siswa.</p>

<div class="sign">
<div>{{.SchoolName}}, {{.IssuedOnLabel}}</div>
<div style="margin-top:2px;">Kepala Sekolah</div>
</div>
</body>
</html>
`))

type letterHTMLViolation struct {
	Name      string
	Points    int
	DateLabel string
}

type letterHTMLData struct {
	Number        string
	SchoolName    string
	LevelLabel    string
	Student       LetterSnapshotStudent
	Total         int
	Threshold     int
	Violations    []letterHTMLViolation
	IssuedOnLabel string
}

// RenderLetterHTML menyusun HTML surat peringatan dari snapshot (pola yang
// sama dengan BuildLetterPDF — dua fungsi murni yang membaca sumber data
// IDENTIK, tidak menduplikasi logika tampilan selain format output).
func RenderLetterHTML(letter LetterRecord, snap LetterSnapshot) string {
	violations := make([]letterHTMLViolation, 0, len(snap.Violations))
	for _, v := range snap.Violations {
		violations = append(violations, letterHTMLViolation{Name: v.Name, Points: v.Points, DateLabel: indonesianDate(v.Date)})
	}
	data := letterHTMLData{
		Number: letter.Number, SchoolName: snap.SchoolName, LevelLabel: levelLabel(letter.Level),
		Student: snap.Student, Total: snap.Total, Threshold: snap.Threshold,
		Violations: violations, IssuedOnLabel: indonesianDate(snap.IssuedOn),
	}
	var buf bytes.Buffer
	// Error template.Execute HANYA bisa terjadi bila data tidak cocok tipe
	// (tidak mungkin di sini, tipe statis) — diabaikan sama seperti pola
	// notification.renderTemplate untuk template internal yang sudah divalidasi.
	_ = letterHTMLTpl.Execute(&buf, data)
	return buf.String()
}
