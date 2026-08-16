package counseling

import (
	"bytes"
	"html/template"
)

// reportHTMLTpl — laporan sesi konseling rapi (docs tugas: "kop app_name,
// identitas siswa+kelas, 3 seksi teks, konselor+tanggal"). HTML self-contained
// TANPA dependency CDN — pola yang sama dengan internal/discipline/html.go.
var reportHTMLTpl = template.Must(template.New("counseling_report").Parse(`<!doctype html>
<html lang="id">
<head>
<meta charset="utf-8">
<title>Laporan Konseling — {{.Student.Name}}</title>
<style>
body { font-family: Helvetica, Arial, sans-serif; color: #1a1a1a; max-width: 720px; margin: 40px auto; padding: 0 16px; }
h1 { text-align: center; font-size: 20px; margin-bottom: 4px; }
h2 { text-align: center; font-size: 14px; font-weight: normal; margin-top: 0; color: #444; }
.info { margin: 20px 0; }
.info div { margin-bottom: 4px; }
.label { display: inline-block; width: 120px; font-weight: bold; }
section { margin: 16px 0; }
section h3 { font-size: 14px; margin-bottom: 4px; }
section p { white-space: pre-wrap; font-size: 13px; line-height: 1.5; margin: 0; }
.sign { margin-top: 40px; text-align: right; font-size: 13px; }
</style>
</head>
<body>
<h1>{{.AppName}}</h1>
<h2>Laporan Sesi Konseling BK</h2>

<div class="info">
<div><span class="label">Nama Siswa</span> : {{.Student.Name}}</div>
<div><span class="label">NIS</span> : {{.Student.NIS}}</div>
<div><span class="label">Kelas</span> : {{.Student.ClassName}}</div>
</div>

<section>
<h3>Tujuan Karir</h3>
<p>{{.CareerGoals}}</p>
</section>

<section>
<h3>Deskripsi Masalah</h3>
<p>{{.ProblemDescription}}</p>
</section>

<section>
<h3>Rencana Tindak Lanjut</h3>
<p>{{.FollowUpPlan}}</p>
</section>

<div class="sign">
<div>Konselor: {{.CounselorName}}</div>
<div>Tanggal: {{.DateLabel}}</div>
</div>
</body>
</html>
`))

var reportMonthNamesID = [...]string{
	"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember",
}

func reportItoa(v int) string {
	if v == 0 {
		return "0"
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	return string(b)
}

type reportHTMLData struct {
	AppName            string
	Student            StudentRef
	CareerGoals        string
	ProblemDescription string
	FollowUpPlan       string
	CounselorName      string
	DateLabel          string
}

// RenderReportHTML menyusun HTML laporan sesi konseling (docs tugas: "laporan
// sesi rapi").
func RenderReportHTML(row Row, appName string) string {
	t := row.CreatedAt
	dateLabel := reportItoa(t.Day()) + " " + reportMonthNamesID[int(t.Month())] + " " + reportItoa(t.Year())
	data := reportHTMLData{
		AppName:     appName,
		Student:     StudentRef{ID: row.StudentID, Name: row.StudentName, NIS: row.StudentNIS, ClassName: row.ClassName},
		CareerGoals: row.CareerGoals, ProblemDescription: row.ProblemDescription, FollowUpPlan: row.FollowUpPlan,
		CounselorName: row.CounselorName, DateLabel: dateLabel,
	}
	var buf bytes.Buffer
	_ = reportHTMLTpl.Execute(&buf, data)
	return buf.String()
}
