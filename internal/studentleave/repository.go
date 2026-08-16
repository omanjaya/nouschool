package studentleave

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/nouschool/internal/studentleave/studentleavedb"
)

var ErrNotFound = errors.New("studentleave: data tidak ditemukan")

// studentLeaveRepository adalah kontrak yang dibutuhkan Service dari
// repository — dipenuhi *Repository secara struktural, supaya Service bisa
// dites dengan fake repository in-memory, tanpa DB (pola sama internal/leave
// & internal/discipline).
type studentLeaveRepository interface {
	CreateRequest(ctx context.Context, in CreateRequestInput) (int64, error)
	GetRequestDetail(ctx context.Context, schoolID, id int64) (RequestDetail, error)
	ListRequestsForStudents(ctx context.Context, schoolID int64, studentIDs []int64, status string) ([]RequestDetail, error)
	ListRequestsByStatus(ctx context.Context, schoolID int64, status string) ([]RequestDetail, error)
	ListRequestsForHomeroomTeacher(ctx context.Context, schoolID, teacherUserID int64, status string) ([]RequestDetail, error)

	UpdateHomeroomDecision(ctx context.Context, schoolID, id int64, newStatus string, decidedBy int64, comment string) (int64, error)
	UpdateBKDecision(ctx context.Context, schoolID, id int64, newStatus string, decidedBy int64, comment, letterNumber, verifyToken string) (int64, error)
	CancelRequest(ctx context.Context, schoolID, id, submittedBy int64) (int64, error)

	NextLetterSeq(ctx context.Context, schoolID int64, year string) (int32, error)
	GetByVerifyToken(ctx context.Context, schoolID int64, token string) (VerifyRecord, bool, error)

	StudentBasic(ctx context.Context, schoolID, studentID int64) (name, nis string, ok bool, err error)
	HomeroomTeacherUserID(ctx context.Context, schoolID, academicYearID, studentID int64) (userID int64, ok bool, err error)
}

var _ studentLeaveRepository = (*Repository)(nil)

// Repository membungkus akses data modul studentleave (sqlc + pgx).
type Repository struct {
	pool *pgxpool.Pool
	q    *studentleavedb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: studentleavedb.New(pool)}
}

func mapNoRows(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// -- domain records --

// RequestDetail adalah representasi domain LENGKAP satu baris pengajuan izin
// siswa (join siswa+kelas TA request+pengaju+pemutus) — sumber tunggal shape
// RequestView (pola sama internal/discipline RecordDetail).
type RequestDetail struct {
	ID                    int64
	SchoolID              int64
	AcademicYearID        int64
	StudentID             int64
	SubmittedBy           int64
	Type                  string
	DateStart             time.Time
	DateEnd               time.Time
	Reason                string
	Attachment            string
	AttachmentName        string
	AttachmentMime        string
	Status                string
	LetterNumber          string
	VerifyToken           string
	HomeroomDecidedBy     int64
	HomeroomDecidedAt     time.Time
	HomeroomComment       string
	HomeroomDecidedByName string
	BKDecidedBy           int64
	BKDecidedAt           time.Time
	BKComment             string
	BKDecidedByName       string
	CreatedAt             time.Time
	StudentName           string
	StudentNIS            string
	ClassName             string
	SubmittedByName       string
}

func (d RequestDetail) View() RequestView {
	v := RequestView{
		ID:        d.ID,
		Student:   StudentRef{ID: d.StudentID, Name: d.StudentName, NIS: d.StudentNIS, ClassName: d.ClassName},
		Type:      d.Type,
		DateStart: NewDate(d.DateStart),
		DateEnd:   NewDate(d.DateEnd),
		Reason:    d.Reason,
		Status:    d.Status,
		CreatedAt: d.CreatedAt,
	}
	if d.Attachment != "" {
		u := "/api/student-leave/" + itoa(d.ID) + "/attachment"
		v.AttachmentURL = &u
	}
	if d.LetterNumber != "" {
		v.LetterNumber = &d.LetterNumber
	}
	if d.Status == StatusIssued && d.VerifyToken != "" {
		v.VerifyToken = &d.VerifyToken
	}
	if d.HomeroomDecidedBy != 0 {
		at := d.HomeroomDecidedAt
		v.Homeroom = &DecisionInfo{DecidedByName: d.HomeroomDecidedByName, DecidedAt: &at, Comment: d.HomeroomComment}
	}
	if d.BKDecidedBy != 0 {
		at := d.BKDecidedAt
		v.BK = &DecisionInfo{DecidedByName: d.BKDecidedByName, DecidedAt: &at, Comment: d.BKComment}
	}
	return v
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

// VerifyRecord adalah hasil GetByVerifyToken (PUBLIK).
type VerifyRecord struct {
	LetterNumber string
	Type         string
	DateStart    time.Time
	DateEnd      time.Time
	IssuedAt     time.Time
	StudentName  string
	ClassName    string
}

// CreateRequestInput adalah parameter Repository.CreateRequest.
type CreateRequestInput struct {
	SchoolID       int64
	AcademicYearID int64
	StudentID      int64
	SubmittedBy    int64
	Type           string
	DateStart      time.Time
	DateEnd        time.Time
	Reason         string
	Attachment     string
	AttachmentName string
	AttachmentMime string
}

func (r *Repository) CreateRequest(ctx context.Context, in CreateRequestInput) (int64, error) {
	return r.q.CreateRequest(ctx, studentleavedb.CreateRequestParams{
		SchoolID: in.SchoolID, AcademicYearID: in.AcademicYearID, StudentID: in.StudentID, SubmittedBy: in.SubmittedBy,
		Type: in.Type, DateStart: dateOf(in.DateStart), DateEnd: dateOf(in.DateEnd), Reason: in.Reason,
		Attachment: in.Attachment, AttachmentName: in.AttachmentName, AttachmentMime: in.AttachmentMime,
	})
}

func detailFromDetailRow(row studentleavedb.GetRequestDetailRow) RequestDetail {
	return RequestDetail{
		ID: row.ID, SchoolID: row.SchoolID, AcademicYearID: row.AcademicYearID, StudentID: row.StudentID, SubmittedBy: row.SubmittedBy,
		Type: row.Type, DateStart: dateToTime(row.DateStart), DateEnd: dateToTime(row.DateEnd), Reason: row.Reason,
		Attachment: row.Attachment, AttachmentName: row.AttachmentName, AttachmentMime: row.AttachmentMime, Status: row.Status,
		LetterNumber: row.LetterNumber.String, VerifyToken: row.VerifyToken.String,
		HomeroomDecidedBy: int8ToInt64(row.HomeroomDecidedBy), HomeroomDecidedAt: tsToTime(row.HomeroomDecidedAt), HomeroomComment: row.HomeroomComment,
		HomeroomDecidedByName: row.HomeroomDecidedByName.String,
		BKDecidedBy:           int8ToInt64(row.BkDecidedBy), BKDecidedAt: tsToTime(row.BkDecidedAt), BKComment: row.BkComment,
		BKDecidedByName: row.BkDecidedByName.String,
		CreatedAt:       tsToTime(row.CreatedAt),
		StudentName:     row.StudentName, StudentNIS: row.StudentNis, ClassName: row.ClassName, SubmittedByName: row.SubmittedByName,
	}
}

func (r *Repository) GetRequestDetail(ctx context.Context, schoolID, id int64) (RequestDetail, error) {
	row, err := r.q.GetRequestDetail(ctx, studentleavedb.GetRequestDetailParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return RequestDetail{}, mapNoRows(err)
	}
	return detailFromDetailRow(row), nil
}

func (r *Repository) ListRequestsForStudents(ctx context.Context, schoolID int64, studentIDs []int64, status string) ([]RequestDetail, error) {
	rows, err := r.q.ListRequestsForStudents(ctx, studentleavedb.ListRequestsForStudentsParams{
		SchoolID: schoolID, StudentIds: studentIDs, Status: textOrNilFrom(status),
	})
	if err != nil {
		return nil, err
	}
	out := make([]RequestDetail, 0, len(rows))
	for _, row := range rows {
		out = append(out, detailFromDetailRow(studentleavedb.GetRequestDetailRow(row)))
	}
	return out, nil
}

func (r *Repository) ListRequestsByStatus(ctx context.Context, schoolID int64, status string) ([]RequestDetail, error) {
	rows, err := r.q.ListRequestsByStatus(ctx, studentleavedb.ListRequestsByStatusParams{
		SchoolID: schoolID, Status: textOrNilFrom(status),
	})
	if err != nil {
		return nil, err
	}
	out := make([]RequestDetail, 0, len(rows))
	for _, row := range rows {
		out = append(out, detailFromDetailRow(studentleavedb.GetRequestDetailRow(row)))
	}
	return out, nil
}

func (r *Repository) ListRequestsForHomeroomTeacher(ctx context.Context, schoolID, teacherUserID int64, status string) ([]RequestDetail, error) {
	rows, err := r.q.ListRequestsForHomeroomTeacher(ctx, studentleavedb.ListRequestsForHomeroomTeacherParams{
		SchoolID: schoolID, UserID: teacherUserID, Status: status,
	})
	if err != nil {
		return nil, err
	}
	out := make([]RequestDetail, 0, len(rows))
	for _, row := range rows {
		out = append(out, detailFromDetailRow(studentleavedb.GetRequestDetailRow(row)))
	}
	return out, nil
}

func (r *Repository) UpdateHomeroomDecision(ctx context.Context, schoolID, id int64, newStatus string, decidedBy int64, comment string) (int64, error) {
	return r.q.UpdateHomeroomDecision(ctx, studentleavedb.UpdateHomeroomDecisionParams{
		ID: id, SchoolID: schoolID, Status: newStatus, HomeroomDecidedBy: int8OrNil(decidedBy), HomeroomComment: comment,
	})
}

func (r *Repository) UpdateBKDecision(ctx context.Context, schoolID, id int64, newStatus string, decidedBy int64, comment, letterNumber, verifyToken string) (int64, error) {
	return r.q.UpdateBKDecision(ctx, studentleavedb.UpdateBKDecisionParams{
		ID: id, SchoolID: schoolID, Status: newStatus, BkDecidedBy: int8OrNil(decidedBy), BkComment: comment,
		LetterNumber: textOrNilFrom(letterNumber), VerifyToken: textOrNilFrom(verifyToken),
	})
}

func (r *Repository) CancelRequest(ctx context.Context, schoolID, id, submittedBy int64) (int64, error) {
	return r.q.CancelRequestExec(ctx, studentleavedb.CancelRequestExecParams{ID: id, SchoolID: schoolID, SubmittedBy: submittedBy})
}

func (r *Repository) NextLetterSeq(ctx context.Context, schoolID int64, year string) (int32, error) {
	return r.q.NextLetterSeq(ctx, studentleavedb.NextLetterSeqParams{SchoolID: schoolID, Year: year})
}

func (r *Repository) GetByVerifyToken(ctx context.Context, schoolID int64, token string) (VerifyRecord, bool, error) {
	row, err := r.q.GetByVerifyToken(ctx, studentleavedb.GetByVerifyTokenParams{VerifyToken: textOrNilFrom(token), SchoolID: schoolID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VerifyRecord{}, false, nil
		}
		return VerifyRecord{}, false, err
	}
	return VerifyRecord{
		LetterNumber: row.LetterNumber.String, Type: row.Type, DateStart: dateToTime(row.DateStart), DateEnd: dateToTime(row.DateEnd),
		IssuedAt: tsToTime(row.BkDecidedAt), StudentName: row.StudentName, ClassName: row.ClassName,
	}, true, nil
}

func (r *Repository) StudentBasic(ctx context.Context, schoolID, studentID int64) (string, string, bool, error) {
	row, err := r.q.StudentBasic(ctx, studentleavedb.StudentBasicParams{ID: studentID, SchoolID: schoolID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", false, nil
		}
		return "", "", false, err
	}
	return row.Name, row.Nis, true, nil
}

func (r *Repository) HomeroomTeacherUserID(ctx context.Context, schoolID, academicYearID, studentID int64) (int64, bool, error) {
	id, err := r.q.HomeroomTeacherUserID(ctx, studentleavedb.HomeroomTeacherUserIDParams{
		StudentID: studentID, SchoolID: schoolID, AcademicYearID: academicYearID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return id, true, nil
}
