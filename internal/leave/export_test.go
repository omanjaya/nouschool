package leave

import (
	"bytes"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func strPtr(s string) *string { return &s }

func TestApproverNameForPicksLastDecidedStep(t *testing.T) {
	req := Request{
		Steps: []StepView{
			{StepOrder: 1, ApproverRole: "guru", Decision: strPtr("approved"), ApproverName: strPtr("Rendi")},
			{StepOrder: 2, ApproverRole: "kepala_sekolah"}, // belum diputuskan
		},
	}
	if got := approverNameFor(req); got != "Rendi" {
		t.Fatalf("approverNameFor salah: got %q, want %q", got, "Rendi")
	}
}

func TestApproverNameForNoDecisionYet(t *testing.T) {
	req := Request{Steps: []StepView{{StepOrder: 1, ApproverRole: "kepala_sekolah"}}}
	if got := approverNameFor(req); got != "-" {
		t.Fatalf("approverNameFor tanpa keputusan seharusnya \"-\", dapat %q", got)
	}
}

func TestApproverNameForAutoApprovedNoSteps(t *testing.T) {
	req := Request{Steps: nil}
	if got := approverNameFor(req); got != "-" {
		t.Fatalf("approverNameFor tanpa step seharusnya \"-\", dapat %q", got)
	}
}

// TestRenderLeaveXLSXCellsMatchData — docs tugas: export leave, isi sel benar
// dari data palsu.
func TestRenderLeaveXLSXCellsMatchData(t *testing.T) {
	reqs := []Request{
		{
			Teacher: TeacherView{Name: "Rendi Saputra"}, TypeLabel: "Sakit",
			DateStart: NewDate(mustDate("2026-08-03")), DateEnd: NewDate(mustDate("2026-08-04")), Days: 2,
			Status: StatusApproved,
			Steps:  []StepView{{StepOrder: 1, ApproverRole: "kepala_sekolah", Decision: strPtr("approved"), ApproverName: strPtr("Kepsek Uji")}},
		},
		{
			Teacher: TeacherView{Name: "Sari Dewi"}, TypeLabel: "Izin",
			DateStart: NewDate(mustDate("2026-08-10")), DateEnd: NewDate(mustDate("2026-08-10")), Days: 1,
			Status: StatusPending,
		},
	}

	data, err := renderLeaveXLSX(reqs)
	if err != nil {
		t.Fatalf("renderLeaveXLSX error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected data non-kosong")
	}

	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("gagal buka hasil xlsx: %v", err)
	}
	defer f.Close()

	if v, _ := f.GetCellValue("Izin", "A1"); v != "Guru" {
		t.Fatalf("header A1 salah: %q", v)
	}
	if v, _ := f.GetCellValue("Izin", "A2"); v != "Rendi Saputra" {
		t.Fatalf("A2 (guru baris 1) salah: %q", v)
	}
	if v, _ := f.GetCellValue("Izin", "B2"); v != "Sakit" {
		t.Fatalf("B2 (jenis baris 1) salah: %q", v)
	}
	if v, _ := f.GetCellValue("Izin", "C2"); v != "2026-08-03" {
		t.Fatalf("C2 (tanggal mulai baris 1) salah: %q", v)
	}
	if v, _ := f.GetCellValue("Izin", "E2"); v != "2" {
		t.Fatalf("E2 (jumlah hari baris 1) salah: %q", v)
	}
	if v, _ := f.GetCellValue("Izin", "F2"); v != "Disetujui" {
		t.Fatalf("F2 (status baris 1) salah: %q", v)
	}
	if v, _ := f.GetCellValue("Izin", "G2"); v != "Kepsek Uji" {
		t.Fatalf("G2 (approver baris 1) salah: %q", v)
	}

	if v, _ := f.GetCellValue("Izin", "A3"); v != "Sari Dewi" {
		t.Fatalf("A3 (guru baris 2) salah: %q", v)
	}
	if v, _ := f.GetCellValue("Izin", "F3"); v != "Menunggu" {
		t.Fatalf("F3 (status baris 2) salah: %q", v)
	}
	if v, _ := f.GetCellValue("Izin", "G3"); v != "-" {
		t.Fatalf("G3 (approver baris 2, belum diputuskan) salah: %q", v)
	}
}

func mustDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}
