package attendance

import (
	"bytes"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestBuildMonthlyMatricesGroupsByClassAndStudent(t *testing.T) {
	rows := []MonthlyRecordRow{
		{ClassID: 1, ClassName: "XII RPL 1", StudentID: 10, StudentName: "Ahmad", StudentNIS: "1001", Date: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC), Status: StatusHadir},
		{ClassID: 1, ClassName: "XII RPL 1", StudentID: 10, StudentName: "Ahmad", StudentNIS: "1001", Date: time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC), Status: StatusSakit},
		{ClassID: 1, ClassName: "XII RPL 1", StudentID: 11, StudentName: "Budi", StudentNIS: "1002", Date: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC), Status: StatusAlpa},
		{ClassID: 2, ClassName: "XI RPL 2", StudentID: 20, StudentName: "Citra", StudentNIS: "2001"}, // tanpa sesi sama sekali (LEFT JOIN kosong)
	}

	matrices := buildMonthlyMatrices(rows)
	if len(matrices) != 2 {
		t.Fatalf("expected 2 kelas, got %d", len(matrices))
	}
	if matrices[0].ClassName != "XII RPL 1" || len(matrices[0].Students) != 2 {
		t.Fatalf("kelas pertama salah: %+v", matrices[0])
	}
	ahmad := matrices[0].Students[0]
	if ahmad.Name != "Ahmad" || ahmad.ByDay[3] != "H" || ahmad.ByDay[4] != "S" {
		t.Fatalf("data Ahmad salah: %+v", ahmad)
	}
	budi := matrices[0].Students[1]
	if budi.ByDay[3] != "A" {
		t.Fatalf("data Budi salah: %+v", budi)
	}

	if matrices[1].ClassName != "XI RPL 2" || len(matrices[1].Students) != 1 {
		t.Fatalf("kelas kedua salah: %+v", matrices[1])
	}
	citra := matrices[1].Students[0]
	if len(citra.ByDay) != 0 {
		t.Fatalf("siswa tanpa sesi seharusnya ByDay kosong: %+v", citra)
	}
}

// TestRenderMonthlyXLSXCellsMatchData membuka kembali workbook hasil
// renderMonthlyXLSX lewat excelize dan memverifikasi isi sel benar dari data
// palsu (docs tugas: "boleh test level service menyusun matrix").
func TestRenderMonthlyXLSXCellsMatchData(t *testing.T) {
	month := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	matrices := []classMonthlyMatrix{
		{
			ClassID: 1, ClassName: "XII RPL 1",
			Students: []monthlyStudentRow{
				{StudentID: 10, Name: "Ahmad", NIS: "1001", ByDay: map[int]string{1: "H", 2: "S", 3: "H"}},
				{StudentID: 11, Name: "Budi", NIS: "1002", ByDay: map[int]string{1: "A"}},
			},
		},
	}

	data, err := renderMonthlyXLSX(matrices, month)
	if err != nil {
		t.Fatalf("renderMonthlyXLSX error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected data non-kosong")
	}

	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("gagal buka hasil xlsx: %v", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) != 1 || sheets[0] != "XII RPL 1" {
		t.Fatalf("nama sheet salah: %v", sheets)
	}

	// Header: A1=NIS, B1=Nama, C1=1 (hari ke-1) ... ujung kanan kolom ringkasan.
	if v, _ := f.GetCellValue("XII RPL 1", "A1"); v != "NIS" {
		t.Fatalf("header A1 salah: %q", v)
	}
	if v, _ := f.GetCellValue("XII RPL 1", "C1"); v != "1" {
		t.Fatalf("header C1 (hari ke-1) salah: %q", v)
	}

	// Baris Ahmad (row 2): NIS, Nama, lalu kolom hari 1-3 berisi H/S/H.
	if v, _ := f.GetCellValue("XII RPL 1", "A2"); v != "1001" {
		t.Fatalf("A2 (NIS Ahmad) salah: %q", v)
	}
	if v, _ := f.GetCellValue("XII RPL 1", "B2"); v != "Ahmad" {
		t.Fatalf("B2 (nama Ahmad) salah: %q", v)
	}
	if v, _ := f.GetCellValue("XII RPL 1", "C2"); v != "H" {
		t.Fatalf("C2 (hari-1 Ahmad) salah: %q", v)
	}
	if v, _ := f.GetCellValue("XII RPL 1", "D2"); v != "S" {
		t.Fatalf("D2 (hari-2 Ahmad) salah: %q", v)
	}
	if v, _ := f.GetCellValue("XII RPL 1", "E2"); v != "H" {
		t.Fatalf("E2 (hari-3 Ahmad) salah: %q", v)
	}

	// Baris Budi (row 3): hari-1 alpa.
	if v, _ := f.GetCellValue("XII RPL 1", "C3"); v != "A" {
		t.Fatalf("C3 (hari-1 Budi) salah: %q", v)
	}

	// Kolom ringkasan: daysInMonth Agustus 2026 = 31, header mulai kolom C
	// (index 3) sampai C+31-1=AG (index 33), lalu 5 kolom ringkasan (Hadir..Alpa).
	daysInMonth := 31
	summaryStartCol := 2 + daysInMonth + 1 // 1-based: NIS(1) Nama(2) hari(3..33) -> ringkasan mulai 34
	hadirCell, _ := excelize.CoordinatesToCellName(summaryStartCol, 2)
	if v, _ := f.GetCellValue("XII RPL 1", hadirCell); v != "2" {
		t.Fatalf("kolom ringkasan Hadir Ahmad (2x H) salah, cell %s = %q", hadirCell, v)
	}
	sakitCell, _ := excelize.CoordinatesToCellName(summaryStartCol+3, 2) // statusOrder = Hadir,Terlambat,Izin,Sakit,Alpa
	if v, _ := f.GetCellValue("XII RPL 1", sakitCell); v != "1" {
		t.Fatalf("kolom ringkasan Sakit Ahmad salah, cell %s = %q", sakitCell, v)
	}
}

func TestRenderMonthlyXLSXEmptyClassStillProducesWorkbook(t *testing.T) {
	data, err := renderMonthlyXLSX(nil, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("renderMonthlyXLSX error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected data non-kosong walau tanpa kelas")
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("gagal buka hasil xlsx: %v", err)
	}
	defer f.Close()
	if len(f.GetSheetList()) != 1 {
		t.Fatalf("expected 1 sheet placeholder, got %v", f.GetSheetList())
	}
}

func TestMonthLabelID(t *testing.T) {
	got := monthLabelID(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if got != "Agustus 2026" {
		t.Fatalf("monthLabelID salah: %q", got)
	}
}
