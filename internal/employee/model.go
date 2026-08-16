// Package employee mengurus profil pegawai (staff non-guru, mis. security/
// tata usaha) — pola yang SAMA dengan modul teachers (student.Teacher):
// profil tipis di atas users+membership. Fase 14 Gelombang B1
// (docs/12-sion-parity.md): fondasi role `pegawai`.
package employee

// RolePegawai — role kanonik dipakai modul employee (nilai HARUS sama persis
// dengan internal/identity — didefinisikan ulang di sini karena employee
// TIDAK boleh mengimpor identity, lihat CLAUDE.md).
const RolePegawai = "pegawai"

// Permission dipakai modul employee — SENGAJA `student:manage` (BUKAN
// permission baru), sesuai instruksi tugas: "GET/POST/PATCH /api/employees
// (perm student:manage)".
const PermManage = "student:manage"

// Employee adalah shape response publik satu pegawai.
type Employee struct {
	ID       int64  `json:"id"`
	UserID   int64  `json:"user_id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Username string `json:"username"`
	NIP      string `json:"nip"`
}

// CreatedEmployee adalah shape response POST /api/employees — SATU KALI
// menampilkan temp_password (pola internal/identity/admin.go P4.2/P2.1).
type CreatedEmployee struct {
	Employee
	TempPassword string `json:"temp_password"`
}
