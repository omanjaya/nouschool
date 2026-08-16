// Package teacherqr mengurus QR token guru — kebalikan QR kartu siswa/
// ruangan (docs/12-sion-parity.md Gelombang B2): guru menampilkan QR
// berumur pendek (60 detik) SEKALI PAKAI di layarnya sendiri, siswa
// memindainya untuk approval alur dispensasi keluar/terlambat
// (internal/exitpermit, internal/latearrival). Modul ini SENGAJA kecil:
// hanya generate + consume, tanpa state machine sendiri.
package teacherqr

import "time"

// RoleGuru — nilai HARUS sama persis dengan internal/identity.RoleGuru
// (didefinisikan ulang di sini karena teacherqr TIDAK boleh mengimpor
// identity, lihat CLAUDE.md).
const RoleGuru = "guru"

// tokenPrefix adalah awalan isi QR ditampilkan guru (frontend yang menyusun
// string QR lengkap "nouschool:tqr:{token}" — endpoint HANYA mengembalikan
// token mentah). Sisi consume menerima token DENGAN ATAU TANPA awalan ini.
const tokenPrefix = "nouschool:tqr:"

// tokenChars/tokenLength — 24 karakter alfanumerik acak (pola sama
// internal/studentleave.verifyTokenChars).
const tokenChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const tokenLength = 24

// tokenTTL — token berlaku 60 detik sejak dibuat (docs tugas Fase 14
// Gelombang B2).
const tokenTTL = 60 * time.Second

// TokenView adalah shape response POST /api/teacher-qr.
type TokenView struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}
