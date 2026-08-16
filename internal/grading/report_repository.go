package grading

import (
	"context"

	"github.com/omanjaya/nouschool/internal/grading/gradingdb"
)

// Berkas ini mengurus akses data GAP 1 (rapor lanjutan, Fase 15): pemetaan
// komponen tipe 'tp' -> kode TP + nilai rapor manual/sebelumnya. Dipisah dari
// repository.go supaya file itu tidak makin membengkak — Repository (struct)
// tetap SATU, method-nya saja disebar antar file (pola Go biasa).

// TPMappingRecord adalah satu baris pemetaan TP + nama komponennya — dipakai
// GET /api/grading/report/tp-mappings (join dengan daftar komponen tipe 'tp'
// dilakukan di Service, bukan di sini, supaya komponen TANPA mapping tetap
// muncul di response).
type TPMappingRecord struct {
	ComponentID int64
	TPCode      string
	Description string
}

// TPMappingWithSubject adalah satu baris pemetaan TP + info mapel/komponen —
// dipakai sheet "TP" pada GET /api/grading/report/export.
type TPMappingWithSubject struct {
	ComponentID   int64
	SubjectID     int64
	SubjectName   string
	ComponentName string
	TPCode        string
	Description   string
}

// ManualScoreRecord adalah satu baris report_manual_scores mentah.
type ManualScoreRecord struct {
	StudentID int64
	Kind      string
	Score     float64
	Note      string
}

func (r *Repository) ListSubjectsWithComponentsForClass(ctx context.Context, schoolID, classID int64) ([]SubjectRef, error) {
	rows, err := r.q.ListSubjectsWithComponentsForClass(ctx, gradingdb.ListSubjectsWithComponentsForClassParams{SchoolID: schoolID, ClassID: classID})
	if err != nil {
		return nil, err
	}
	out := make([]SubjectRef, 0, len(rows))
	for _, row := range rows {
		out = append(out, SubjectRef{ID: row.ID, Code: row.Code, Name: row.Name})
	}
	return out, nil
}

// ReplaceTPMappings menghapus SELURUH mapping (class,subject) lalu insert
// ulang daftar baru — semantik "replace penuh" (docs tugas GAP 1: "PUT ...
// replace utk (class,subject)"). Pemanggil (Service) SUDAH memvalidasi
// component_id unik dalam request & milik pasangan (class,subject) itu
// bertipe 'tp' SEBELUM baris pertama dihapus.
func (r *Repository) ReplaceTPMappings(ctx context.Context, schoolID, academicYearID, classID, subjectID int64, mappings []TPMappingInput) error {
	if err := r.q.DeleteTPMappingsForClassSubject(ctx, gradingdb.DeleteTPMappingsForClassSubjectParams{SchoolID: schoolID, ClassID: classID, SubjectID: subjectID}); err != nil {
		return err
	}
	for _, m := range mappings {
		if err := r.q.CreateTPMapping(ctx, gradingdb.CreateTPMappingParams{
			SchoolID: schoolID, AcademicYearID: academicYearID, ClassID: classID, SubjectID: subjectID,
			ComponentID: m.ComponentID, TpCode: m.TPCode, Description: m.Description,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) ListTPMappingsForClassSubject(ctx context.Context, schoolID, classID, subjectID int64) ([]TPMappingRecord, error) {
	rows, err := r.q.ListTPMappingsForClassSubject(ctx, gradingdb.ListTPMappingsForClassSubjectParams{SchoolID: schoolID, ClassID: classID, SubjectID: subjectID})
	if err != nil {
		return nil, err
	}
	out := make([]TPMappingRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, TPMappingRecord{ComponentID: row.ComponentID, TPCode: row.TpCode, Description: row.Description})
	}
	return out, nil
}

func (r *Repository) ListTPMappingsForClass(ctx context.Context, schoolID, classID int64) ([]TPMappingWithSubject, error) {
	rows, err := r.q.ListTPMappingsForClass(ctx, gradingdb.ListTPMappingsForClassParams{SchoolID: schoolID, ClassID: classID})
	if err != nil {
		return nil, err
	}
	out := make([]TPMappingWithSubject, 0, len(rows))
	for _, row := range rows {
		out = append(out, TPMappingWithSubject{
			ComponentID: row.ComponentID, SubjectID: row.SubjectID, SubjectName: row.SubjectName,
			ComponentName: row.ComponentName, TPCode: row.TpCode, Description: row.Description,
		})
	}
	return out, nil
}

func (r *Repository) UpsertManualScore(ctx context.Context, schoolID, academicYearID, classID, subjectID, studentID int64, kind string, score float64, note string, setBy int64) error {
	return r.q.UpsertManualScore(ctx, gradingdb.UpsertManualScoreParams{
		SchoolID: schoolID, AcademicYearID: academicYearID, ClassID: classID, SubjectID: subjectID, StudentID: studentID,
		Kind: kind, Score: score, Note: note, SetBy: int8OrNil(setBy),
	})
}

func (r *Repository) DeleteManualScore(ctx context.Context, schoolID, academicYearID, classID, subjectID, studentID int64, kind string) error {
	return r.q.DeleteManualScore(ctx, gradingdb.DeleteManualScoreParams{
		SchoolID: schoolID, AcademicYearID: academicYearID, ClassID: classID, SubjectID: subjectID, StudentID: studentID, Kind: kind,
	})
}

func (r *Repository) ListManualScoresForClassSubject(ctx context.Context, schoolID, academicYearID, classID, subjectID int64) ([]ManualScoreRecord, error) {
	rows, err := r.q.ListManualScoresForClassSubject(ctx, gradingdb.ListManualScoresForClassSubjectParams{
		SchoolID: schoolID, AcademicYearID: academicYearID, ClassID: classID, SubjectID: subjectID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]ManualScoreRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, ManualScoreRecord{StudentID: row.StudentID, Kind: row.Kind, Score: row.Score, Note: row.Note})
	}
	return out, nil
}
