package student

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul student. Middleware auth/permission
// disuntik dari main.go (diimplementasikan modul identity) — student TIDAK
// mengimpor identity secara langsung (lihat aturan wiring di CLAUDE.md).
func RegisterRoutes(
	mux *http.ServeMux,
	h *Handler,
	requireAuth middleware.Middleware,
	requirePerm func(perm string) middleware.Middleware,
) {
	auth := func(hf http.HandlerFunc) http.Handler { return requireAuth(hf) }
	manage := func(hf http.HandlerFunc) http.Handler { return requireAuth(requirePerm(PermStudentManage)(hf)) }
	read := func(hf http.HandlerFunc) http.Handler { return requireAuth(requirePerm(PermStudentRead)(hf)) }
	scheduleRead := func(hf http.HandlerFunc) http.Handler { return requireAuth(requirePerm(PermScheduleRead)(hf)) }

	// -- siswa --
	mux.Handle("GET /api/students", read(h.ListStudents))
	mux.Handle("POST /api/students", manage(h.CreateStudent))
	// GetStudent sengaja hanya requireAuth: otorisasi (student:read ATAU
	// object-level siswa/orang tua) sepenuhnya di Service.GetStudent.
	mux.Handle("GET /api/students/{id}", auth(h.GetStudent))
	mux.Handle("PATCH /api/students/{id}", manage(h.UpdateStudent))

	// -- rombel --
	mux.Handle("GET /api/classes", read(h.ListClasses))
	mux.Handle("POST /api/classes", manage(h.CreateClass))
	mux.Handle("PATCH /api/classes/{id}", manage(h.UpdateClass))
	mux.Handle("POST /api/classes/{id}/students", manage(h.EnrollStudents))
	mux.Handle("DELETE /api/classes/{id}/students/{studentId}", manage(h.UnenrollStudent))

	// -- orang tua --
	mux.Handle("GET /api/me/children", auth(h.GetMyChildren))

	// -- guru --
	mux.Handle("GET /api/teachers", read(h.ListTeachers))
	mux.Handle("POST /api/teachers", manage(h.CreateTeacher))
	mux.Handle("PATCH /api/teachers/{id}", manage(h.UpdateTeacher))

	// -- mapel --
	mux.Handle("GET /api/subjects", scheduleRead(h.ListSubjects))
	mux.Handle("POST /api/subjects", manage(h.CreateSubject))
	mux.Handle("PATCH /api/subjects/{id}", manage(h.UpdateSubject))

	// -- undangan --
	mux.Handle("POST /api/invitations/generate", manage(h.GenerateInvitations))
	// Publik (host tenant) — dipakai layar aktivasi siswa/orang tua sebelum login.
	mux.HandleFunc("GET /api/auth/invitation/{code}", h.GetInvitationInfo)
	mux.HandleFunc("POST /api/auth/activate", h.Activate)

	// -- import --
	mux.Handle("GET /api/import/students/template", manage(h.StudentImportTemplate))
	mux.Handle("POST /api/import/students", manage(h.PreviewStudentImport))
	mux.Handle("POST /api/import/students/commit", manage(h.CommitStudentImport))
	mux.Handle("GET /api/import/teachers/template", manage(h.TeacherImportTemplate))
	mux.Handle("POST /api/import/teachers", manage(h.PreviewTeacherImport))
	mux.Handle("POST /api/import/teachers/commit", manage(h.CommitTeacherImport))
	// Import Dapodik (Fase 11, docs/03-student.md) — commit memakai handler
	// KHUSUS (CommitDapodikImport) tapi memanggil service yang SAMA
	// (CommitStudentImport) dengan endpoint /api/import/students/commit.
	mux.Handle("POST /api/import/dapodik", manage(h.PreviewDapodikImport))
	mux.Handle("POST /api/import/dapodik/commit", manage(h.CommitDapodikImport))
}
