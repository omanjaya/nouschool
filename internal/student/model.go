// Package student mengurus siswa, rombel, enrollment, wali, guru (profil),
// mapel, import Excel/CSV, dan undangan akun siswa/wali (lihat docs/03-student.md).
package student

import (
	"fmt"
	"strings"
	"time"
)

// Date adalah tanggal murni (tanpa jam), dikodekan JSON sebagai "YYYY-MM-DD"
// atau null. Sama seperti internal/tenant.Date — didefinisikan ulang di sini
// supaya student TIDAK mengimpor package tenant (aturan modular monolith:
// antar-modul lewat interface kecil, bukan import langsung).
type Date struct{ time.Time }

func NewDate(t time.Time) Date { return Date{t} }

func (d Date) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", d.Format("2006-01-02"))), nil
}

func (d *Date) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return fmt.Errorf("format tanggal harus YYYY-MM-DD: %w", err)
	}
	d.Time = t
	return nil
}

// Role — nilai kanonik role membership yang relevan bagi modul student
// (harus sama persis dengan konstanta di internal/identity, tapi
// didefinisikan ulang di sini karena student tidak boleh mengimpor identity).
const (
	RoleGuru     = "guru"
	RoleSiswa    = "siswa"
	RoleOrangTua = "orang_tua"
)

// Relasi wali (guardians.relation).
const (
	RelationAyah = "ayah"
	RelationIbu  = "ibu"
	RelationWali = "wali"
)

// ClassRef adalah potongan info rombel disematkan pada Student.
type ClassRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Student adalah shape response publik siswa.
type Student struct {
	ID        int64     `json:"id"`
	NIS       string    `json:"nis"`
	NISN      string    `json:"nisn"`
	Name      string    `json:"name"`
	Gender    string    `json:"gender"`
	BirthDate *Date     `json:"birth_date"`
	Status    string    `json:"status"`
	Class     *ClassRef `json:"class"`
	UserID    int64     `json:"user_id"` // 0 = belum aktivasi akun (dipakai UI tag status akun)
}

// StudentRecord adalah representasi domain internal (dipakai service &
// object-level check) — menyertakan UserID/SchoolID yang TIDAK pernah
// diserialisasi langsung ke klien.
type StudentRecord struct {
	ID        int64
	SchoolID  int64
	NIS       string
	NISN      string
	Name      string
	Gender    string
	BirthDate *Date
	UserID    int64 // 0 = belum aktivasi akun siswa
	Status    string
	ClassID   int64 // 0 = tidak terdaftar di rombel pada tahun ajaran ini
	ClassName string
}

// View mengonversi record internal menjadi shape response publik.
func (r StudentRecord) View() Student {
	v := Student{
		ID: r.ID, NIS: r.NIS, NISN: r.NISN, Name: r.Name, Gender: r.Gender,
		BirthDate: r.BirthDate, Status: r.Status, UserID: r.UserID,
	}
	if r.ClassID != 0 {
		v.Class = &ClassRef{ID: r.ClassID, Name: r.ClassName}
	}
	return v
}

// TeacherRef adalah potongan info guru disematkan pada Class (wali kelas).
type TeacherRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Class adalah shape response publik rombel.
type Class struct {
	ID              int64       `json:"id"`
	Name            string      `json:"name"`
	Grade           string      `json:"grade"`
	Major           string      `json:"major"`
	HomeroomTeacher *TeacherRef `json:"homeroom_teacher"`
	StudentCount    int64       `json:"student_count"`
}

// ClassRecord adalah representasi domain internal rombel.
type ClassRecord struct {
	ID                int64
	SchoolID          int64
	AcademicYearID    int64
	Name              string
	Grade             string
	Major             string
	HomeroomTeacherID int64
}

// Teacher adalah shape response publik guru.
type Teacher struct {
	ID     int64  `json:"id"`
	UserID int64  `json:"user_id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	NIP    string `json:"nip"`
}

// Subject adalah shape response publik mapel.
type Subject struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// AcademicYearOption adalah shape response GET /api/academic-years (filter UI).
type AcademicYearOption struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

// InvitationCodes adalah shape response POST /api/invitations/generate per siswa.
type InvitationCodes struct {
	StudentID    int64   `json:"student_id"`
	StudentName  string  `json:"student_name"`
	StudentCode  *string `json:"student_code"`
	GuardianCode *string `json:"guardian_code"`
}

// InvitationInfo adalah shape response GET /api/auth/invitation/{code}.
type InvitationInfo struct {
	Role        string `json:"role"`
	StudentName string `json:"student_name"`
	SchoolName  string `json:"school_name"`
}
