package notification

import (
	"bytes"
	"fmt"
	"text/template"
)

// eventTemplate adalah satu baris registry: template judul/isi (bahasa
// Indonesia, TANPA emoji — lihat docs/10-design-system.md), channel default
// (di LUAR in_app — in_app selalu baseline, lihat Service.Send), dan link
// frontend opsional disematkan ke notifications.link.
type eventTemplate struct {
	titleTpl *template.Template
	bodyTpl  *template.Template
	Channels []string
	Link     string
}

func mustTemplate(name, title, body string, channels []string, link string) eventTemplate {
	return eventTemplate{
		titleTpl: template.Must(template.New(name + ".title").Parse(title)),
		bodyTpl:  template.Must(template.New(name + ".body").Parse(body)),
		Channels: channels,
		Link:     link,
	}
}

// registry — event -> template. Modul baru menambah event = daftarkan
// konstanta (model.go) + baris di sini (docs/08-notification.md "Event
// awal": "modul lain menambah event = daftarkan konstanta event + template +
// baris di tabel ini").
//
// Catatan cakupan Fase 9: `announcement.published` SENGAJA TIDAK didaftarkan
// di sini — docs/08 menandainya sebagai event yang tidak dikirim modul ini
// pada fase ini (lihat instruksi tugas "SKIP, jangan"); modul announcement
// belum diubah untuk memanggil Notify.
var registry = map[string]eventTemplate{
	// Data: student, date (YYYY-MM-DD), status (hadir/terlambat/izin/sakit/alpa).
	EventAttendanceStudentAbsent: mustTemplate("attendance_student_absent",
		"Absensi {{.student}}",
		"{{.student}} tercatat {{.status}} pada {{.date}}.",
		[]string{ChannelWhatsApp, ChannelWebPush},
		"/kehadiran",
	),
	// Data: teacher, type, date_start, date_end (YYYY-MM-DD).
	EventLeaveSubmitted: mustTemplate("leave_submitted",
		"Pengajuan izin baru",
		"{{.teacher}} mengajukan {{.type}} tanggal {{.date_start}} s.d. {{.date_end}}. Mohon ditinjau.",
		[]string{ChannelWebPush},
		"/izin/persetujuan",
	),
	// Data: type, date_start, date_end (YYYY-MM-DD), decision (label Indonesia,
	// mis. "disetujui"/"ditolak" — modul leave yang menerjemahkan).
	EventLeaveDecided: mustTemplate("leave_decided",
		"Pengajuan izin diputuskan",
		"Pengajuan {{.type}} tanggal {{.date_start}} s.d. {{.date_end}} telah {{.decision}}.",
		[]string{ChannelWebPush, ChannelEmail},
		"/izin",
	),
	// Data: student, type (nama jenis pelanggaran), points (int).
	// Fase 14 Gelombang A (docs/12-sion-parity.md) — dikirim ke orang tua
	// SETIAP catatan pelanggaran (routine, webpush saja — beda dari surat SP
	// di bawah yang formal/jarang).
	EventDisciplineRecorded: mustTemplate("discipline_recorded",
		"Catatan pelanggaran {{.student}}",
		"{{.student}} mendapat catatan pelanggaran: {{.type}} (+{{.points}} poin).",
		[]string{ChannelWebPush},
		"/kedisiplinan",
	),
	// Data: student, level (int 1-3).
	// Dikirim ke orang tua DAN wali kelas saat surat peringatan terbit
	// otomatis — dokumen formal, webpush + email (pola sama leave.decided).
	EventDisciplineSPIssued: mustTemplate("discipline_sp_issued",
		"Surat Peringatan SP{{.level}}",
		"Surat Peringatan SP{{.level}} diterbitkan untuk {{.student}}.",
		[]string{ChannelWebPush, ChannelEmail},
		"/kedisiplinan",
	),
	// Data: student, type ("sakit"/"izin"), date_start, date_end (YYYY-MM-DD).
	// Fase 14 Gelombang B1 (docs/12-sion-parity.md) — dikirim ke pemegang
	// review tahap 1 (wali kelas rombel siswa, fallback pemegang flag
	// leave_homeroom_review) saat siswa/ortu mengajukan izin terencana.
	EventStudentLeaveSubmitted: mustTemplate("studentleave_submitted",
		"Pengajuan izin siswa baru",
		"{{.student}} mengajukan {{.type}} tanggal {{.date_start}} s.d. {{.date_end}}. Mohon ditinjau.",
		[]string{ChannelWebPush},
		"/izin-siswa/persetujuan",
	),
	// Data: student, type. Dikirim ke pemegang flag leave_issuance (BK) saat
	// wali kelas menyetujui tahap 1.
	EventStudentLeaveForwarded: mustTemplate("studentleave_forwarded",
		"Izin siswa menunggu penerbitan surat",
		"Pengajuan {{.type}} {{.student}} sudah disetujui wali kelas, menunggu penerbitan surat BK.",
		[]string{ChannelWebPush},
		"/izin-siswa/persetujuan",
	),
	// Data: student, type, decision (label Indonesia, mis. "diterbitkan"/
	// "ditolak wali kelas"/"ditolak BK"). Dikirim ke pengaju+orang tua (+wali
	// kelas bila surat terbit) saat keputusan akhir (tolak di tahap mana pun,
	// atau surat terbit).
	EventStudentLeaveDecided: mustTemplate("studentleave_decided",
		"Pengajuan izin siswa diputuskan",
		"Pengajuan {{.type}} {{.student}} telah {{.decision}}.",
		[]string{ChannelWebPush, ChannelEmail},
		"/izin-siswa",
	),
	// Fase 14 Gelombang B2 (docs/12-sion-parity.md). Data: student.
	// Dikirim ke siswa+ortu saat rantai QR dispensasi keluar selesai
	// (pending_leadership -> issued).
	EventExitPermitIssued: mustTemplate("exitpermit_issued",
		"Izin keluar disetujui",
		"Izin dispensasi keluar {{.student}} disetujui — tunjukkan QR di gerbang.",
		[]string{ChannelWebPush},
		"/izin-keluar",
	),
	// Data: student, time (HH:MM lokal sekolah). Dikirim ke ortu saat siswa
	// discan security di gerbang (issued -> exited).
	EventExitPermitExited: mustTemplate("exitpermit_exited",
		"{{.student}} keluar sekolah",
		"{{.student}} keluar sekolah pukul {{.time}}.",
		[]string{ChannelWebPush},
		"/izin-keluar",
	),
	// Fase 15 GAP 3 (docs/ROADMAP.md). Data: student, comment. Dikirim ke
	// siswa+ortu saat approver tahap berjalan (atau admin) menolak pengajuan
	// dispensasi keluar.
	EventExitPermitRejected: mustTemplate("exitpermit_rejected",
		"Izin keluar ditolak",
		"Izin keluar ditolak {{.student}}: {{.comment}}",
		[]string{ChannelWebPush},
		"/izin-keluar",
	),
	// Data: student, late_count (int), action (label Indonesia, "" bila
	// ActionNone — template menyembunyikan kalimat aksi lewat {{if .action}}).
	// Dikirim ke ortu setiap kali keterlambatan tercatat (scan piket pertama
	// hari itu).
	EventLateArrivalRecorded: mustTemplate("latearrival_recorded",
		"Keterlambatan {{.student}}",
		"{{.student}} tercatat terlambat ke-{{.late_count}} hari ini.{{if .action}} Tindakan: {{.action}}.{{end}}",
		[]string{ChannelWebPush},
		"/keterlambatan",
	),
	// Fase 14 Gelombang C (docs/12-sion-parity.md). Data: subject (nama
	// mapel). Dikirim ke SELURUH siswa kelas (yang sudah aktivasi akun) +
	// orang tua mereka saat guru/admin mempublikasikan nilai kelas-mapel.
	EventGradingPublished: mustTemplate("grading_published",
		"Nilai {{.subject}} dipublikasikan",
		"Nilai {{.subject}} telah dipublikasikan.",
		[]string{ChannelWebPush},
		"/nilai",
	),
	// Fase 14 Gelombang D (docs/12-sion-parity.md). Data: teacher (pengaju),
	// class, subject, date (YYYY-MM-DD). Dikirim ke guru yang diminta jadi
	// pengganti.
	EventSubstitutionRequested: mustTemplate("substitution_requested",
		"Permintaan menjadi guru pengganti",
		"{{.teacher}} meminta Anda menggantikan mengajar {{.subject}} {{.class}} pada {{.date}}.",
		[]string{ChannelWebPush},
		"/substitusi",
	),
	// Data: substitute, class, subject, date (YYYY-MM-DD), decision (label
	// Indonesia, mis. "diterima"/"ditolak"/"dibatalkan"). Dikirim ke pengaju
	// saat pengganti memutuskan.
	EventSubstitutionDecided: mustTemplate("substitution_decided",
		"Permintaan guru pengganti diputuskan",
		"Permintaan pengganti {{.subject}} {{.class}} tanggal {{.date}} telah {{.decision}} oleh {{.substitute}}.",
		[]string{ChannelWebPush},
		"/substitusi",
	),
}

// renderTemplate mengeksekusi title/body suatu event terhadap data — fungsi
// murni (mudah dites tanpa DB, lihat service_test.go).
func renderTemplate(t eventTemplate, data map[string]any) (title, body string, err error) {
	var tb, bb bytes.Buffer
	if err := t.titleTpl.Execute(&tb, data); err != nil {
		return "", "", fmt.Errorf("notification: render judul: %w", err)
	}
	if err := t.bodyTpl.Execute(&bb, data); err != nil {
		return "", "", fmt.Errorf("notification: render isi: %w", err)
	}
	return tb.String(), bb.String(), nil
}
