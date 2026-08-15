package schedule

import "testing"

// fakeScheduleLookup implementasi scheduleImportLookup dengan map in-memory.
type fakeScheduleLookup struct {
	classes  map[string]int64
	subjects map[string][2]any // code -> [id, name]
	teachers map[string][2]any // email -> [id, name]
	rooms    map[string]int64
}

func (f fakeScheduleLookup) ClassIDByName(name string) (int64, bool) {
	id, ok := f.classes[name]
	return id, ok
}

func (f fakeScheduleLookup) SubjectByCode(code string) (int64, string, bool) {
	v, ok := f.subjects[code]
	if !ok {
		return 0, "", false
	}
	return v[0].(int64), v[1].(string), true
}

func (f fakeScheduleLookup) TeacherByEmail(email string) (int64, string, bool) {
	v, ok := f.teachers[email]
	if !ok {
		return 0, "", false
	}
	return v[0].(int64), v[1].(string), true
}

func (f fakeScheduleLookup) RoomIDByName(name string) (int64, bool) {
	id, ok := f.rooms[name]
	return id, ok
}

func baseLookup() fakeScheduleLookup {
	return fakeScheduleLookup{
		classes:  map[string]int64{"XII RPL 1": 100},
		subjects: map[string][2]any{"BDT": {int64(200), "Basis Data"}},
		teachers: map[string][2]any{"rendi@demo.sch.id": {int64(300), "Rendi"}},
		rooms:    map[string]int64{"R-101": 400},
	}
}

func TestParseSlotRows_DayNameOrNumber(t *testing.T) {
	records := [][]string{
		{"rombel", "hari", "jam_mulai", "jam_selesai", "kode_mapel", "email_guru", "ruangan"},
		{"XII RPL 1", "Senin", "1", "2", "BDT", "rendi@demo.sch.id", "R-101"},
		{"XII RPL 1", "3", "3", "4", "BDT", "rendi@demo.sch.id", ""},
	}
	rows, err := parseSlotRows(records, baseLookup())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].DayOfWeek != 1 {
		t.Fatalf("expected 'Senin' -> 1, got %d", rows[0].DayOfWeek)
	}
	if rows[0].Action != "ok" {
		t.Fatalf("expected action ok, got %q (messages: %v)", rows[0].Action, rows[0].Messages)
	}
	if rows[1].DayOfWeek != 3 {
		t.Fatalf("expected '3' -> 3, got %d", rows[1].DayOfWeek)
	}
	if rows[1].RoomID != 0 {
		t.Fatalf("expected tanpa ruang (kosong boleh), got room_id=%d", rows[1].RoomID)
	}
}

func TestParseSlotRows_UnknownReferences(t *testing.T) {
	records := [][]string{
		{"rombel", "hari", "jam_mulai", "jam_selesai", "kode_mapel", "email_guru", "ruangan"},
		{"XII RPL ZZZ", "Senin", "1", "2", "XXX", "siapa@demo.sch.id", "Lab Hantu"},
	}
	rows, err := parseSlotRows(records, baseLookup())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.Action != "error" {
		t.Fatalf("expected action error, got %q", r.Action)
	}
	if len(r.Messages) != 4 { // rombel, mapel, guru, ruangan semua tak dikenal
		t.Fatalf("expected 4 pesan (rombel/mapel/guru/ruangan tak dikenal), got %d: %v", len(r.Messages), r.Messages)
	}
}

func TestParseSlotRows_InvalidDay(t *testing.T) {
	records := [][]string{
		{"rombel", "hari", "jam_mulai", "jam_selesai", "kode_mapel", "email_guru", "ruangan"},
		{"XII RPL 1", "Minggu", "1", "2", "BDT", "rendi@demo.sch.id", ""},
	}
	rows, err := parseSlotRows(records, baseLookup())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows[0].Action != "error" {
		t.Fatalf("expected 'Minggu' ditolak (bukan hari sekolah 1-6), got action=%q", rows[0].Action)
	}
}

func TestParseSlotRows_MissingRequiredHeader(t *testing.T) {
	records := [][]string{
		{"rombel", "hari", "kode_mapel", "email_guru"}, // tanpa jam_mulai/jam_selesai
		{"XII RPL 1", "Senin", "BDT", "rendi@demo.sch.id"},
	}
	_, err := parseSlotRows(records, baseLookup())
	if err == nil {
		t.Fatal("expected error kolom wajib hilang, got nil")
	}
}

func TestParseSlotRows_PeriodStartAfterEnd(t *testing.T) {
	records := [][]string{
		{"rombel", "hari", "jam_mulai", "jam_selesai", "kode_mapel", "email_guru", "ruangan"},
		{"XII RPL 1", "Senin", "4", "2", "BDT", "rendi@demo.sch.id", ""},
	}
	rows, err := parseSlotRows(records, baseLookup())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows[0].Action != "error" {
		t.Fatalf("expected error (jam_mulai > jam_selesai), got action=%q", rows[0].Action)
	}
}

// -- markImportConflicts --

func TestMarkImportConflicts_FlagsRowsThatOverlap(t *testing.T) {
	rows := []slotImportRow{
		{RowNum: 2, ClassID: 100, SubjectID: 200, TeacherID: 300, DayOfWeek: 1, PeriodStart: 1, PeriodEnd: 2, Action: "ok"},
		{RowNum: 3, ClassID: 101, SubjectID: 200, TeacherID: 300, DayOfWeek: 1, PeriodStart: 2, PeriodEnd: 3, Action: "ok"}, // guru sama, beririsan
		{RowNum: 4, ClassID: 102, SubjectID: 200, TeacherID: 301, DayOfWeek: 2, PeriodStart: 1, PeriodEnd: 2, Action: "ok"}, // tidak bentrok
	}
	out := markImportConflicts(rows, nil)
	if out[0].Action != "ok" {
		t.Fatalf("baris pertama harus tetap ok, got %q", out[0].Action)
	}
	if out[1].Action != "error" {
		t.Fatalf("baris kedua harus error (bentrok guru dgn baris pertama), got %q", out[1].Action)
	}
	if out[2].Action != "ok" {
		t.Fatalf("baris ketiga harus tetap ok (tidak bentrok), got %q", out[2].Action)
	}
}

func TestMarkImportConflicts_ConflictsWithExistingDBSlot(t *testing.T) {
	existing := []SlotRecord{
		{ID: 1, ClassID: 100, TeacherID: 300, DayOfWeek: 1, PeriodStart: 1, PeriodEnd: 2},
	}
	rows := []slotImportRow{
		{RowNum: 2, ClassID: 100, SubjectID: 200, TeacherID: 300, DayOfWeek: 1, PeriodStart: 2, PeriodEnd: 3, Action: "ok"},
	}
	out := markImportConflicts(rows, existing)
	if out[0].Action != "error" {
		t.Fatalf("expected error (bentrok dgn slot DB existing), got %q", out[0].Action)
	}
}
