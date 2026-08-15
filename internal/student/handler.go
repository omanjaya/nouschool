package student

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// Handler menerjemahkan HTTP <-> Service. Tidak menyusun SQL, tidak
// memutuskan aturan bisnis — hanya decode/validate bentuk + panggil service.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func pathInt64(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}

func queryInt64(r *http.Request, name string) int64 {
	v, _ := strconv.ParseInt(r.URL.Query().Get(name), 10, 64)
	return v
}

func queryInt(r *http.Request, name string) int {
	v, _ := strconv.Atoi(r.URL.Query().Get(name))
	return v
}

// -- students --

func (h *Handler) ListStudents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	result, err := h.svc.ListStudents(r.Context(), reqctx.SchoolID(r.Context()), ListStudentsQuery{
		Query: q.Get("q"), ClassID: queryInt64(r, "class_id"),
		Page: queryInt(r, "page"), PerPage: queryInt(r, "per_page"),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

type createStudentRequest struct {
	NIS       string `json:"nis"`
	NISN      string `json:"nisn"`
	Name      string `json:"name"`
	Gender    string `json:"gender"`
	BirthDate *Date  `json:"birth_date"`
	ClassID   int64  `json:"class_id"`
}

func (h *Handler) CreateStudent(w http.ResponseWriter, r *http.Request) {
	var req createStudentRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	st, err := h.svc.CreateStudent(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), CreateStudentRequest{
		NIS: req.NIS, NISN: req.NISN, Name: req.Name, Gender: req.Gender, BirthDate: req.BirthDate, ClassID: req.ClassID,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, st)
}

// GetStudent — GET /api/students/{id}. Sengaja TIDAK dipasang requirePerm di
// routes.go: otorisasi (permission student:read ATAU object-level siswa/ortu)
// ditegakkan sepenuhnya di Service.GetStudent (docs/03-student.md).
func (h *Handler) GetStudent(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID siswa tidak valid."))
		return
	}
	ctx := r.Context()
	st, err := h.svc.GetStudent(ctx, reqctx.SchoolID(ctx), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, st)
}

type updateStudentRequest struct {
	NIS       *string `json:"nis"`
	NISN      *string `json:"nisn"`
	Name      *string `json:"name"`
	Gender    *string `json:"gender"`
	BirthDate *Date   `json:"birth_date"`
	Status    *string `json:"status"`
}

func (h *Handler) UpdateStudent(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID siswa tidak valid."))
		return
	}
	var req updateStudentRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	in := UpdateStudentInput{}
	if req.NIS != nil {
		in.NIS = *req.NIS
	}
	if req.NISN != nil {
		in.NISN = *req.NISN
	}
	if req.Name != nil {
		in.Name = *req.Name
	}
	if req.Gender != nil {
		in.Gender = *req.Gender
	}
	if req.BirthDate != nil {
		in.BirthDate = req.BirthDate
	}
	if req.Status != nil {
		in.Status = *req.Status
	}
	ctx := r.Context()
	st, err := h.svc.UpdateStudent(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id, in)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, st)
}

// -- classes --

func (h *Handler) ListClasses(w http.ResponseWriter, r *http.Request) {
	classes, err := h.svc.ListClasses(r.Context(), reqctx.SchoolID(r.Context()), queryInt64(r, "academic_year_id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, classes)
}

type createClassRequest struct {
	Name              string `json:"name"`
	Grade             string `json:"grade"`
	Major             string `json:"major"`
	HomeroomTeacherID int64  `json:"homeroom_teacher_id"`
	AcademicYearID    int64  `json:"academic_year_id"`
}

func (h *Handler) CreateClass(w http.ResponseWriter, r *http.Request) {
	var req createClassRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	cls, err := h.svc.CreateClass(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), CreateClassRequest{
		Name: req.Name, Grade: req.Grade, Major: req.Major,
		HomeroomTeacherID: req.HomeroomTeacherID, AcademicYearID: req.AcademicYearID,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, cls)
}

type updateClassRequest struct {
	Name              *string `json:"name"`
	Grade             *string `json:"grade"`
	Major             *string `json:"major"`
	HomeroomTeacherID *int64  `json:"homeroom_teacher_id"`
}

func (h *Handler) UpdateClass(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID rombel tidak valid."))
		return
	}
	var req updateClassRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	in := UpdateClassInput{}
	if req.Name != nil {
		in.Name = *req.Name
	}
	if req.Grade != nil {
		in.Grade = *req.Grade
	}
	if req.Major != nil {
		in.Major = *req.Major
	}
	if req.HomeroomTeacherID != nil {
		in.HomeroomTeacherID = *req.HomeroomTeacherID
	}
	ctx := r.Context()
	cls, err := h.svc.UpdateClass(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id, in)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, cls)
}

type enrollStudentsRequest struct {
	StudentIDs []int64 `json:"student_ids"`
}

func (h *Handler) EnrollStudents(w http.ResponseWriter, r *http.Request) {
	classID, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID rombel tidak valid."))
		return
	}
	var req enrollStudentsRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	if err := h.svc.EnrollStudents(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), classID, req.StudentIDs); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) UnenrollStudent(w http.ResponseWriter, r *http.Request) {
	classID, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID rombel tidak valid."))
		return
	}
	studentID, err := pathInt64(r, "studentId")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID siswa tidak valid."))
		return
	}
	ctx := r.Context()
	if err := h.svc.UnenrollStudent(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), classID, studentID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// -- teachers --

func (h *Handler) ListTeachers(w http.ResponseWriter, r *http.Request) {
	teachers, err := h.svc.ListTeachers(r.Context(), reqctx.SchoolID(r.Context()))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, teachers)
}

type createTeacherRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	NIP   string `json:"nip"`
}

func (h *Handler) CreateTeacher(w http.ResponseWriter, r *http.Request) {
	var req createTeacherRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	t, err := h.svc.CreateTeacher(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), CreateTeacherRequest{
		Name: req.Name, Email: req.Email, NIP: req.NIP,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, t)
}

type updateTeacherRequest struct {
	NIP *string `json:"nip"`
}

func (h *Handler) UpdateTeacher(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID guru tidak valid."))
		return
	}
	var req updateTeacherRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	nip := ""
	if req.NIP != nil {
		nip = *req.NIP
	}
	ctx := r.Context()
	t, err := h.svc.UpdateTeacher(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id, nip)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}

// -- subjects --

func (h *Handler) ListSubjects(w http.ResponseWriter, r *http.Request) {
	subjects, err := h.svc.ListSubjects(r.Context(), reqctx.SchoolID(r.Context()))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, subjects)
}

type subjectRequest struct {
	Code *string `json:"code"`
	Name *string `json:"name"`
}

func (h *Handler) CreateSubject(w http.ResponseWriter, r *http.Request) {
	var req subjectRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	code, name := "", ""
	if req.Code != nil {
		code = *req.Code
	}
	if req.Name != nil {
		name = *req.Name
	}
	ctx := r.Context()
	subj, err := h.svc.CreateSubject(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), CreateSubjectRequest{Code: code, Name: name})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, subj)
}

func (h *Handler) UpdateSubject(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID mapel tidak valid."))
		return
	}
	var req subjectRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	code, name := "", ""
	if req.Code != nil {
		code = *req.Code
	}
	if req.Name != nil {
		name = *req.Name
	}
	ctx := r.Context()
	subj, err := h.svc.UpdateSubject(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id, code, name)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, subj)
}

// GetMyChildren — GET /api/me/children (role orang_tua).
func (h *Handler) GetMyChildren(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	children, err := h.svc.ListMyChildren(ctx, reqctx.SchoolID(ctx), reqctx.UserID(ctx))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, children)
}

// -- undangan --

type generateInvitationsRequest struct {
	ClassID int64 `json:"class_id"`
}

func (h *Handler) GenerateInvitations(w http.ResponseWriter, r *http.Request) {
	var req generateInvitationsRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	codes, err := h.svc.GenerateInvitations(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), req.ClassID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, codes)
}

// GetInvitationInfo — GET /api/auth/invitation/{code} (PUBLIK, host tenant).
func (h *Handler) GetInvitationInfo(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	ctx := r.Context()
	sch, _ := reqctx.SchoolFromContext(ctx)
	info, err := h.svc.GetInvitationInfo(ctx, sch.ID, sch.Name, code)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, info)
}

type activateRequest struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// clientIP mengambil IP klien dari RemoteAddr (host:port).
func clientIP(r *http.Request) string {
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx > 0 {
		return r.RemoteAddr[:idx]
	}
	return r.RemoteAddr
}

// Activate — POST /api/auth/activate (PUBLIK, host tenant).
func (h *Handler) Activate(w http.ResponseWriter, r *http.Request) {
	var req activateRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	sch, _ := reqctx.SchoolFromContext(ctx)
	result, err := h.svc.Activate(ctx, sch.ID, SchoolRef{ID: sch.ID, Name: sch.Name, Slug: sch.Slug}, ActivateInput{
		Code: req.Code, Name: req.Name, Username: req.Username, Email: req.Email, Password: req.Password,
		IP: clientIP(r), UserAgent: r.UserAgent(),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.svc.SetSessionCookie(w, result.Token, result.ExpiresAt)
	httpx.JSON(w, http.StatusOK, map[string]any{"user": result.View})
}

// -- import --

const maxImportFileSize = 10 << 20 // 10 MB

// StudentImportTemplate — GET /api/import/students/template.
func (h *Handler) StudentImportTemplate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="template-siswa.csv"`)
	_, _ = w.Write([]byte("nis,nisn,nama,jenis_kelamin,tanggal_lahir,rombel\n"))
}

// TeacherImportTemplate — GET /api/import/teachers/template.
func (h *Handler) TeacherImportTemplate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="template-guru.csv"`)
	_, _ = w.Write([]byte("nip,nama,email\n"))
}

func readUploadedFile(r *http.Request) (filename string, content []byte, err error) {
	if err := r.ParseMultipartForm(maxImportFileSize); err != nil {
		return "", nil, httpx.Validation("Gagal membaca form upload.")
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return "", nil, httpx.Validation("File wajib diunggah (field 'file').")
	}
	defer file.Close()
	content, err = io.ReadAll(io.LimitReader(file, maxImportFileSize))
	if err != nil {
		return "", nil, httpx.Validation("Gagal membaca isi file.")
	}
	return header.Filename, content, nil
}

// PreviewStudentImport — POST /api/import/students.
func (h *Handler) PreviewStudentImport(w http.ResponseWriter, r *http.Request) {
	filename, content, err := readUploadedFile(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	preview, err := h.svc.PreviewStudentImport(r.Context(), reqctx.SchoolID(r.Context()), filename, content)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, preview)
}

type importCommitRequest struct {
	UploadID string `json:"upload_id"`
}

// CommitStudentImport — POST /api/import/students/commit.
func (h *Handler) CommitStudentImport(w http.ResponseWriter, r *http.Request) {
	var req importCommitRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	created, updated, skipped, err := h.svc.CommitStudentImport(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), req.UploadID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"created": created, "updated": updated, "skipped": skipped})
}

// PreviewTeacherImport — POST /api/import/teachers.
func (h *Handler) PreviewTeacherImport(w http.ResponseWriter, r *http.Request) {
	filename, content, err := readUploadedFile(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	preview, err := h.svc.PreviewTeacherImport(r.Context(), reqctx.SchoolID(r.Context()), filename, content)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, preview)
}

// CommitTeacherImport — POST /api/import/teachers/commit.
func (h *Handler) CommitTeacherImport(w http.ResponseWriter, r *http.Request) {
	var req importCommitRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	created, updated, skipped, err := h.svc.CommitTeacherImport(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), req.UploadID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"created": created, "updated": updated, "skipped": skipped})
}
