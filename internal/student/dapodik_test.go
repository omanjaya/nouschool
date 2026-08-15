package student

import "testing"

// fakeDapodikLookup implementasi dapodikImportLookup dengan map in-memory.
type fakeDapodikLookup struct {
	nisnToID  map[string]int64
	nisToID   map[string]int64
	classToID map[string]int64
}

func (f fakeDapodikLookup) StudentIDByNISN(nisn string) (int64, bool) {
	id, ok := f.nisnToID[nisn]
	return id, ok
}

func (f fakeDapodikLookup) StudentIDByNIS(nis string) (int64, bool) {
	id, ok := f.nisToID[nis]
	return id, ok
}

func (f fakeDapodikLookup) ClassIDByName(name string) (int64, bool) {
	id, ok := f.classToID[name]
	return id, ok
}

func emptyDapodikLookup() fakeDapodikLookup {
	return fakeDapodikLookup{nisnToID: map[string]int64{}, nisToID: map[string]int64{}, classToID: map[string]int64{}}
}

func TestParseDapodikRowsHeaderVariants(t *testing.T) {
	// Header gaya export Dapodik: "Nama Peserta Didik", "No. Induk" (NIS),
	// "JK" (jenis kelamin), "Rombongan Belajar" — TIDAK sama persis dengan
	// header template NouSchool (nis/nama/jenis_kelamin/rombel).
	records := [][]string{
		{"No. Induk", "NISN", "Nama Peserta Didik", "JK", "Tanggal Lahir", "Rombongan Belajar"},
		{"1001", "0099887766", "Ahmad Fauzi", "L", "2008-05-01", "XII RPL 1"},
	}
	lookup := emptyDapodikLookup()
	lookup.classToID["XII RPL 1"] = 5
	rows, err := parseDapodikRows(records, lookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.NIS != "1001" || r.NISN != "0099887766" || r.Name != "Ahmad Fauzi" || r.Gender != "L" || r.ClassID != 5 {
		t.Fatalf("parse header dapodik salah: %+v", r)
	}
	if r.Action != "create" {
		t.Fatalf("expected action create, got %q (messages: %v)", r.Action, r.Messages)
	}
}

func TestParseDapodikRowsGenderFormats(t *testing.T) {
	records := [][]string{
		{"nis", "nama", "jk"},
		{"1", "A", "L"},
		{"2", "B", "P"},
		{"3", "C", "Laki-laki"},
		{"4", "D", "PEREMPUAN"},
		{"5", "E", "laki laki"},
		{"6", "F", "X"}, // tidak dikenal -> error
	}
	rows, err := parseDapodikRows(records, emptyDapodikLookup())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"L", "P", "L", "P", "L"}
	for i, w := range want {
		if rows[i].Gender != w {
			t.Fatalf("baris %d: gender salah, dapat %q ingin %q (messages: %v)", i, rows[i].Gender, w, rows[i].Messages)
		}
	}
	if rows[5].Action != "error" {
		t.Fatalf("baris 6 (JK tidak dikenal) seharusnya error, dapat %q", rows[5].Action)
	}
}

func TestParseDapodikRowsDateFormats(t *testing.T) {
	records := [][]string{
		{"nis", "nama", "tanggal_lahir"},
		{"1", "A", "2008-05-01"}, // ISO
		{"2", "B", "01/05/2008"}, // DD/MM/YYYY
		{"3", "C", "01-05-2008"}, // DD-MM-YYYY (gaya Dapodik umum)
		{"4", "D", "2008/05/01"}, // YYYY/MM/DD
		{"5", "E", "39569"},      // serial Excel (2008-05-01)
	}
	rows, err := parseDapodikRows(records, emptyDapodikLookup())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "2008-05-01"
	for i, r := range rows {
		if r.BirthDate == nil {
			t.Fatalf("baris %d: tanggal seharusnya kebaca, dapat nil (messages: %v)", i, r.Messages)
		}
		if got := r.BirthDate.Format("2006-01-02"); got != want {
			t.Fatalf("baris %d: tanggal salah, dapat %q ingin %q", i, got, want)
		}
	}
}

func TestParseDapodikRowsWithoutNISOrNISNIsError(t *testing.T) {
	records := [][]string{
		{"nis", "nisn", "nama"},
		{"", "", "Tanpa Identitas"},
	}
	rows, err := parseDapodikRows(records, emptyDapodikLookup())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows[0].Action != "error" {
		t.Fatalf("baris tanpa NIS & NISN seharusnya error, dapat %q", rows[0].Action)
	}
	found := false
	for _, m := range rows[0].Messages {
		if m == "NIS dan NISN tidak diisi — minimal salah satu wajib." {
			found = true
		}
	}
	if !found {
		t.Fatalf("pesan error NIS/NISN kosong tidak ditemukan: %v", rows[0].Messages)
	}
}

func TestParseDapodikRowsMatchByNISNFirstFallbackNIS(t *testing.T) {
	records := [][]string{
		{"nis", "nisn", "nama"},
		{"1001", "9988", "Cocok Lewat NISN"}, // NISN terdaftar -> update, walau NIS beda
		{"2002", "", "Cocok Lewat NIS"},      // tanpa NISN -> fallback NIS
		{"3003", "7777", "Siswa Baru"},       // tidak match apa pun -> create
	}
	lookup := emptyDapodikLookup()
	lookup.nisnToID["9988"] = 42
	lookup.nisToID["2002"] = 43
	rows, err := parseDapodikRows(records, lookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows[0].Action != "update" || rows[0].ExistingID != 42 {
		t.Fatalf("baris 1 seharusnya update by NISN: %+v", rows[0])
	}
	if rows[1].Action != "update" || rows[1].ExistingID != 43 {
		t.Fatalf("baris 2 seharusnya update by NIS (fallback): %+v", rows[1])
	}
	if rows[2].Action != "create" || rows[2].Existing {
		t.Fatalf("baris 3 seharusnya create: %+v", rows[2])
	}
}

func TestParseDapodikRowsNewStudentWithoutNISIsError(t *testing.T) {
	// NISN diisi tapi tidak ditemukan siswa manapun dengan NISN itu, dan NIS
	// kosong -> tidak bisa dijadikan baris create (kolom nis NOT NULL di DB).
	records := [][]string{
		{"nisn", "nama"},
		{"555999", "Siswa Tanpa NIS"},
	}
	rows, err := parseDapodikRows(records, emptyDapodikLookup())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows[0].Action != "error" {
		t.Fatalf("baris NISN tak dikenal tanpa NIS seharusnya error baris, dapat %q (messages: %v)", rows[0].Action, rows[0].Messages)
	}
}

func TestParseDapodikRowsMissingBothIDColumnsFails(t *testing.T) {
	records := [][]string{
		{"nama"}, // tidak ada kolom nis maupun nisn sama sekali
		{"Ahmad"},
	}
	if _, err := parseDapodikRows(records, emptyDapodikLookup()); err == nil {
		t.Fatal("expected error karena tidak ada kolom NIS maupun NISN di header")
	}
}
