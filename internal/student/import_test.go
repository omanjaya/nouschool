package student

import (
	"testing"
)

// fakeStudentLookup implementasi studentImportLookup dengan map in-memory.
type fakeStudentLookup struct {
	nisToID   map[string]int64
	classToID map[string]int64
}

func (f fakeStudentLookup) StudentIDByNIS(nis string) (int64, bool) {
	id, ok := f.nisToID[nis]
	return id, ok
}

func (f fakeStudentLookup) ClassIDByName(name string) (int64, bool) {
	id, ok := f.classToID[name]
	return id, ok
}

func TestParseStudentRowsHeaderCaseInsensitive(t *testing.T) {
	records := [][]string{
		{"NIS", "Nama", "Jenis_Kelamin", "Rombel"}, // header campur besar/kecil, urutan bebas
		{"1001", "Ahmad", "L", "XII RPL 1"},
	}
	lookup := fakeStudentLookup{nisToID: map[string]int64{}, classToID: map[string]int64{"XII RPL 1": 5}}
	rows, err := parseStudentRows(records, lookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.NIS != "1001" || r.Name != "Ahmad" || r.Gender != "L" || r.ClassID != 5 {
		t.Fatalf("parse salah: %+v", r)
	}
	if r.Action != "create" {
		t.Fatalf("expected action create, got %q (messages: %v)", r.Action, r.Messages)
	}
}

func TestParseStudentRowsHeaderOrderIndependent(t *testing.T) {
	// Kolom sengaja diacak urutannya — parser harus tetap benar (by name, bukan posisi).
	records := [][]string{
		{"rombel", "nama", "nis", "nisn", "tanggal_lahir", "jenis_kelamin"},
		{"XI RPL 2", "Budi", "2002", "9988", "2008-05-01", "L"},
	}
	lookup := fakeStudentLookup{nisToID: map[string]int64{}, classToID: map[string]int64{"XI RPL 2": 6}}
	rows, err := parseStudentRows(records, lookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := rows[0]
	if r.NIS != "2002" || r.NISN != "9988" || r.ClassID != 6 {
		t.Fatalf("parse salah kolom by posisi, bukan by nama: %+v", r)
	}
	if r.BirthDate == nil || r.BirthDate.Format("2006-01-02") != "2008-05-01" {
		t.Fatalf("tanggal lahir salah: %+v", r.BirthDate)
	}
}

func TestParseStudentRowsDateFormats(t *testing.T) {
	records := [][]string{
		{"nis", "nama", "tanggal_lahir"},
		{"1", "A", "2008-05-01"}, // YYYY-MM-DD
		{"2", "B", "01/05/2008"}, // DD/MM/YYYY
		{"3", "C", "39569"},      // serial Excel (2008-05-01)
		{"4", "D", "bukan-tanggal"},
	}
	lookup := fakeStudentLookup{nisToID: map[string]int64{}, classToID: map[string]int64{}}
	rows, err := parseStudentRows(records, lookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "2008-05-01"
	for i, r := range rows[:3] {
		if r.BirthDate == nil {
			t.Fatalf("baris %d: tanggal seharusnya kebaca, dapat nil (messages: %v)", i, r.Messages)
		}
		if got := r.BirthDate.Format("2006-01-02"); got != want {
			t.Fatalf("baris %d: tanggal salah, dapat %q ingin %q", i, got, want)
		}
	}
	// Baris ke-4: format tidak dikenali -> error.
	if rows[3].Action != "error" {
		t.Fatalf("baris 4 (tanggal rusak) seharusnya error, dapat %q", rows[3].Action)
	}
}

func TestParseStudentRowsDuplicateNISInFileIsErrorOnSecondRow(t *testing.T) {
	records := [][]string{
		{"nis", "nama"},
		{"1001", "Ahmad"},
		{"1001", "Ahmad Duplikat"},
	}
	lookup := fakeStudentLookup{nisToID: map[string]int64{}, classToID: map[string]int64{}}
	rows, err := parseStudentRows(records, lookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Action == "error" {
		t.Fatalf("baris pertama (NIS pertama kali muncul) seharusnya TIDAK error: %+v", rows[0])
	}
	if rows[1].Action != "error" {
		t.Fatalf("baris kedua (NIS duplikat) seharusnya error, dapat %q", rows[1].Action)
	}
	found := false
	for _, m := range rows[1].Messages {
		if m == "NIS duplikat dalam file." {
			found = true
		}
	}
	if !found {
		t.Fatalf("pesan duplikat NIS tidak ditemukan: %v", rows[1].Messages)
	}
}

func TestParseStudentRowsUpdateVsCreate(t *testing.T) {
	records := [][]string{
		{"nis", "nama"},
		{"1001", "Ahmad Sudah Ada"}, // NIS sudah terdaftar -> update
		{"9999", "Siswa Baru"},      // NIS belum ada -> create
	}
	lookup := fakeStudentLookup{nisToID: map[string]int64{"1001": 42}, classToID: map[string]int64{}}
	rows, err := parseStudentRows(records, lookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows[0].Action != "update" || !rows[0].Existing || rows[0].ExistingID != 42 {
		t.Fatalf("baris NIS terdaftar seharusnya update: %+v", rows[0])
	}
	if rows[1].Action != "create" || rows[1].Existing {
		t.Fatalf("baris NIS baru seharusnya create: %+v", rows[1])
	}
}

func TestParseStudentRowsUnknownRombelIsError(t *testing.T) {
	records := [][]string{
		{"nis", "nama", "rombel"},
		{"1001", "Ahmad", "Kelas Siluman"},
	}
	lookup := fakeStudentLookup{nisToID: map[string]int64{}, classToID: map[string]int64{"XII RPL 1": 5}}
	rows, err := parseStudentRows(records, lookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows[0].Action != "error" {
		t.Fatalf("rombel tak dikenal seharusnya error baris, dapat %q", rows[0].Action)
	}
}

func TestParseStudentRowsMissingRequiredHeaderFails(t *testing.T) {
	records := [][]string{
		{"nisn", "nama"}, // kolom "nis" tidak ada
		{"9988", "Ahmad"},
	}
	lookup := fakeStudentLookup{nisToID: map[string]int64{}, classToID: map[string]int64{}}
	if _, err := parseStudentRows(records, lookup); err == nil {
		t.Fatal("expected error karena kolom nis wajib tidak ada di header")
	}
}

// -- import guru --

type fakeTeacherLookup struct {
	byEmail map[string]int64
	byNIP   map[string]int64
}

func (f fakeTeacherLookup) TeacherIDByEmail(email string) (int64, bool) {
	id, ok := f.byEmail[email]
	return id, ok
}

func (f fakeTeacherLookup) TeacherIDByNIP(nip string) (int64, bool) {
	id, ok := f.byNIP[nip]
	return id, ok
}

func TestParseTeacherRowsMatchByEmailFallbackNIP(t *testing.T) {
	records := [][]string{
		{"email", "nama", "nip"},
		{"budi@sch.id", "Budi", "111"},   // match by email
		{"", "Sari", "222"},              // tidak ada email -> fallback NIP
		{"baru@sch.id", "Guru Baru", ""}, // tidak match apa pun -> create
	}
	lookup := fakeTeacherLookup{
		byEmail: map[string]int64{"budi@sch.id": 7},
		byNIP:   map[string]int64{"222": 8},
	}
	rows, err := parseTeacherRows(records, lookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows[0].Action != "update" || rows[0].ExistingID != 7 {
		t.Fatalf("baris 1 seharusnya update by email: %+v", rows[0])
	}
	if rows[1].Action != "update" || rows[1].ExistingID != 8 {
		t.Fatalf("baris 2 seharusnya update by NIP fallback: %+v", rows[1])
	}
	if rows[2].Action != "create" {
		t.Fatalf("baris 3 seharusnya create: %+v", rows[2])
	}
}

func TestParseTeacherRowsRequiresEmailOrNIP(t *testing.T) {
	records := [][]string{
		{"email", "nama", "nip"},
		{"", "Tanpa Identitas", ""},
	}
	lookup := fakeTeacherLookup{byEmail: map[string]int64{}, byNIP: map[string]int64{}}
	rows, err := parseTeacherRows(records, lookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows[0].Action != "error" {
		t.Fatalf("baris tanpa email & NIP seharusnya error: %+v", rows[0])
	}
}
