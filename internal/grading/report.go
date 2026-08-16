package grading

import (
	"context"
	"strings"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
)

// Berkas ini mengurus GAP 1 Fase 15 (rapor lanjutan, docs tugas): pemetaan
// komponen tipe 'tp' -> kode TP, nilai rapor manual/sebelumnya (manual
// MENANG atas nilai akhir dinormalisasi; previous HANYA kolom informasi),
// analisis kelas-mapel, dan export rapor per kelas (report_export.go).
// Dipisah dari service.go supaya file itu tidak makin membengkak.

// -- resolusi nilai rapor (dipakai GetManualScores, ReportAnalysis, export) --

// reportResolution adalah nilai rapor SATU siswa pada SATU (class,subject)
// SUDAH diresolusi: manual (bila ada) MENANG atas computed final; previous
// HANYA disematkan sebagai info, TIDAK PERNAH masuk resolusi/analisis.
type reportResolution struct {
	Previous *float64
	Manual   *ManualScoreDetail
	Computed *float64
	Resolved *float64 // nil bila TIDAK ADA manual maupun computed sama sekali
	Source   string   // "manual" | "computed" | "none"
}

// buildReportResolutions menghitung reportResolution SETIAP siswa roster
// untuk (class,subject) — SATU sumber dipakai ULANG oleh GetManualScores,
// ReportAnalysis, dan report export supaya aturan resolusi TIDAK terduplikasi.
func (s *Service) buildReportResolutions(ctx context.Context, schoolID, academicYearID, classID, subjectID int64, components []ComponentRecord, roster []RosterStudent) (map[int64]reportResolution, error) {
	componentIDs := make([]int64, 0, len(components))
	for _, c := range components {
		componentIDs = append(componentIDs, c.ID)
	}
	gradesByComponent, err := s.repo.ListGradesForComponents(ctx, componentIDs)
	if err != nil {
		return nil, err
	}
	manualRows, err := s.repo.ListManualScoresForClassSubject(ctx, schoolID, academicYearID, classID, subjectID)
	if err != nil {
		return nil, err
	}
	manualByStudent := make(map[int64]map[string]ManualScoreRecord, len(manualRows))
	for _, m := range manualRows {
		if manualByStudent[m.StudentID] == nil {
			manualByStudent[m.StudentID] = map[string]ManualScoreRecord{}
		}
		manualByStudent[m.StudentID][m.Kind] = m
	}

	out := make(map[int64]reportResolution, len(roster))
	for _, st := range roster {
		studentScores := map[int64]float64{}
		for _, c := range components {
			if v, ok := gradesByComponent[c.ID][st.ID]; ok {
				studentScores[c.ID] = v
			}
		}
		computed, _ := computeFinal(components, studentScores)
		res := reportResolution{Computed: computed, Source: "none"}
		if rec, ok := manualByStudent[st.ID][ReportScoreKindPrevious]; ok {
			v := rec.Score
			res.Previous = &v
		}
		if rec, ok := manualByStudent[st.ID][ReportScoreKindManual]; ok {
			res.Manual = &ManualScoreDetail{Score: rec.Score, Note: rec.Note}
			v := rec.Score
			res.Resolved = &v
			res.Source = "manual"
		} else if computed != nil {
			v := *computed
			res.Resolved = &v
			res.Source = "computed"
		}
		out[st.ID] = res
	}
	return out, nil
}

// -- GET/PUT /api/grading/report/tp-mappings --

// GetTPMappings — SEMUA komponen tipe 'tp' pada (class,subject) + mapping-nya
// (tp_code/description "" bila belum dipetakan) — guard SAMA dengan
// components (toggle grading + object-level guru; admin bebas).
func (s *Service) GetTPMappings(ctx context.Context, schoolID, classID, subjectID int64) (TPMappingsView, error) {
	if _, err := s.requireEnabled(ctx, schoolID); err != nil {
		return TPMappingsView{}, err
	}
	if err := s.requireManage(ctx); err != nil {
		return TPMappingsView{}, err
	}
	if classID == 0 || subjectID == 0 {
		return TPMappingsView{}, httpx.Validation("class_id dan subject_id wajib diisi.")
	}
	if err := s.requireClassSubjectAccess(ctx, schoolID, classID, subjectID); err != nil {
		return TPMappingsView{}, err
	}
	return s.tpMappingsView(ctx, schoolID, classID, subjectID)
}

func (s *Service) tpMappingsView(ctx context.Context, schoolID, classID, subjectID int64) (TPMappingsView, error) {
	components, err := s.repo.ListComponentsRaw(ctx, schoolID, classID, subjectID)
	if err != nil {
		return TPMappingsView{}, err
	}
	mappings, err := s.repo.ListTPMappingsForClassSubject(ctx, schoolID, classID, subjectID)
	if err != nil {
		return TPMappingsView{}, err
	}
	byComponent := make(map[int64]TPMappingRecord, len(mappings))
	for _, m := range mappings {
		byComponent[m.ComponentID] = m
	}
	items := make([]TPMappingRow, 0)
	for _, c := range components {
		if c.Type != ComponentTP {
			continue
		}
		row := TPMappingRow{ComponentID: c.ID, ComponentName: c.Name}
		if m, ok := byComponent[c.ID]; ok {
			row.TPCode, row.Description = m.TPCode, m.Description
		}
		items = append(items, row)
	}
	return TPMappingsView{Items: items}, nil
}

// PutTPMappings — replace PENUH mapping (class,subject): setiap component_id
// WAJIB komponen bertipe 'tp' milik pasangan (class,subject) itu & TIDAK
// boleh duplikat dalam satu request (docs tugas: "replace utk (class,subject)").
func (s *Service) PutTPMappings(ctx context.Context, actorUserID, schoolID, classID, subjectID int64, in []TPMappingInput) (TPMappingsView, error) {
	if _, err := s.requireEnabled(ctx, schoolID); err != nil {
		return TPMappingsView{}, err
	}
	if err := s.requireManage(ctx); err != nil {
		return TPMappingsView{}, err
	}
	if classID == 0 || subjectID == 0 {
		return TPMappingsView{}, httpx.Validation("class_id dan subject_id wajib diisi.")
	}
	if err := s.requireClassSubjectAccess(ctx, schoolID, classID, subjectID); err != nil {
		return TPMappingsView{}, err
	}
	components, err := s.repo.ListComponentsRaw(ctx, schoolID, classID, subjectID)
	if err != nil {
		return TPMappingsView{}, err
	}
	tpComponents := make(map[int64]bool, len(components))
	for _, c := range components {
		if c.Type == ComponentTP {
			tpComponents[c.ID] = true
		}
	}
	seen := make(map[int64]bool, len(in))
	for i := range in {
		in[i].TPCode = strings.TrimSpace(in[i].TPCode)
		in[i].Description = strings.TrimSpace(in[i].Description)
		if !tpComponents[in[i].ComponentID] {
			return TPMappingsView{}, httpx.Validation("component_id harus komponen bertipe tp pada kelas-mapel ini.")
		}
		if seen[in[i].ComponentID] {
			return TPMappingsView{}, httpx.Validation("component_id tidak boleh duplikat dalam satu permintaan.")
		}
		seen[in[i].ComponentID] = true
	}
	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return TPMappingsView{}, err
	}
	if err := s.repo.ReplaceTPMappings(ctx, schoolID, yearID, classID, subjectID, in); err != nil {
		return TPMappingsView{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "grading.report_tp_mappings_set", "assessment_component", classID, nil,
		map[string]any{"class_id": classID, "subject_id": subjectID, "count": len(in)})
	return s.tpMappingsView(ctx, schoolID, classID, subjectID)
}

// -- GET/PUT /api/grading/report/manual-scores --

// GetManualScores — SATU baris per siswa rombel: previous/manual (bila ada)
// + computed_final (nilai akhir dinormalisasi, SEBELUM resolusi manual) —
// guard SAMA dengan components.
func (s *Service) GetManualScores(ctx context.Context, schoolID, classID, subjectID int64) (ReportScoresView, error) {
	if _, err := s.requireEnabled(ctx, schoolID); err != nil {
		return ReportScoresView{}, err
	}
	if err := s.requireManage(ctx); err != nil {
		return ReportScoresView{}, err
	}
	if classID == 0 || subjectID == 0 {
		return ReportScoresView{}, httpx.Validation("class_id dan subject_id wajib diisi.")
	}
	if err := s.requireClassSubjectAccess(ctx, schoolID, classID, subjectID); err != nil {
		return ReportScoresView{}, err
	}
	return s.manualScoresView(ctx, schoolID, classID, subjectID)
}

func (s *Service) manualScoresView(ctx context.Context, schoolID, classID, subjectID int64) (ReportScoresView, error) {
	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return ReportScoresView{}, err
	}
	components, err := s.repo.ListComponentsRaw(ctx, schoolID, classID, subjectID)
	if err != nil {
		return ReportScoresView{}, err
	}
	roster, err := s.repo.ListClassRoster(ctx, schoolID, classID)
	if err != nil {
		return ReportScoresView{}, err
	}
	resolutions, err := s.buildReportResolutions(ctx, schoolID, yearID, classID, subjectID, components, roster)
	if err != nil {
		return ReportScoresView{}, err
	}
	items := make([]ReportScoreRow, 0, len(roster))
	for _, st := range roster {
		res := resolutions[st.ID]
		items = append(items, ReportScoreRow{
			StudentID: st.ID, Name: st.Name, NIS: st.NIS,
			Previous: res.Previous, Manual: res.Manual, ComputedFinal: res.Computed,
		})
	}
	return ReportScoresView{Items: items}, nil
}

// PutManualScores — score nil = hapus baris (kind) siswa itu (docs tugas:
// "null = hapus").
func (s *Service) PutManualScores(ctx context.Context, actorUserID, schoolID, classID, subjectID int64, entries []ManualScoreEntryInput) (ReportScoresView, error) {
	if _, err := s.requireEnabled(ctx, schoolID); err != nil {
		return ReportScoresView{}, err
	}
	if err := s.requireManage(ctx); err != nil {
		return ReportScoresView{}, err
	}
	if classID == 0 || subjectID == 0 {
		return ReportScoresView{}, httpx.Validation("class_id dan subject_id wajib diisi.")
	}
	if err := s.requireClassSubjectAccess(ctx, schoolID, classID, subjectID); err != nil {
		return ReportScoresView{}, err
	}
	roster, err := s.repo.ListClassRoster(ctx, schoolID, classID)
	if err != nil {
		return ReportScoresView{}, err
	}
	enrolled := make(map[int64]bool, len(roster))
	for _, st := range roster {
		enrolled[st.ID] = true
	}
	for _, e := range entries {
		if !enrolled[e.StudentID] {
			return ReportScoresView{}, httpx.Validation("Siswa bukan anggota rombel kelas ini.")
		}
		if !validReportScoreKind(e.Kind) {
			return ReportScoresView{}, httpx.Validation("kind harus 'previous' atau 'manual'.")
		}
		if e.Score != nil && (*e.Score < 0 || *e.Score > 100) {
			return ReportScoresView{}, httpx.Validation("Nilai harus di antara 0 dan 100.")
		}
	}
	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return ReportScoresView{}, err
	}
	// Transaksional per-baris (pola sama PutGrades) — validasi SUDAH lolos
	// semua sebelum mutasi pertama dimulai.
	for _, e := range entries {
		if e.Score == nil {
			if err := s.repo.DeleteManualScore(ctx, schoolID, yearID, classID, subjectID, e.StudentID, e.Kind); err != nil {
				return ReportScoresView{}, err
			}
			continue
		}
		if err := s.repo.UpsertManualScore(ctx, schoolID, yearID, classID, subjectID, e.StudentID, e.Kind, *e.Score, strings.TrimSpace(e.Note), actorUserID); err != nil {
			return ReportScoresView{}, err
		}
	}
	s.audit(ctx, schoolID, actorUserID, "grading.report_manual_scores_update", "assessment_component", classID, nil,
		map[string]any{"class_id": classID, "subject_id": subjectID, "count": len(entries)})
	return s.manualScoresView(ctx, schoolID, classID, subjectID)
}

// -- GET /api/grading/report/analysis --

// ReportAnalysis — guard SAMA dengan Recap (Fase 15 GAP 5: grading:manage
// ATAU grading:read; canManage=false/kepsek melewati object-level).
func (s *Service) ReportAnalysis(ctx context.Context, schoolID, classID, subjectID int64) (ReportAnalysisView, error) {
	if _, err := s.requireEnabled(ctx, schoolID); err != nil {
		return ReportAnalysisView{}, err
	}
	canManage, err := s.requireReadAccess(ctx)
	if err != nil {
		return ReportAnalysisView{}, err
	}
	if classID == 0 || subjectID == 0 {
		return ReportAnalysisView{}, httpx.Validation("class_id dan subject_id wajib diisi.")
	}
	if canManage {
		if err := s.requireClassSubjectAccess(ctx, schoolID, classID, subjectID); err != nil {
			return ReportAnalysisView{}, err
		}
	}
	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return ReportAnalysisView{}, err
	}
	components, err := s.repo.ListComponentsRaw(ctx, schoolID, classID, subjectID)
	if err != nil {
		return ReportAnalysisView{}, err
	}
	roster, err := s.repo.ListClassRoster(ctx, schoolID, classID)
	if err != nil {
		return ReportAnalysisView{}, err
	}
	componentIDs := make([]int64, 0, len(components))
	for _, c := range components {
		componentIDs = append(componentIDs, c.ID)
	}
	gradesByComponent, err := s.repo.ListGradesForComponents(ctx, componentIDs)
	if err != nil {
		return ReportAnalysisView{}, err
	}
	resolutions, err := s.buildReportResolutions(ctx, schoolID, yearID, classID, subjectID, components, roster)
	if err != nil {
		return ReportAnalysisView{}, err
	}
	settings, err := s.loadSettings(ctx, schoolID)
	if err != nil {
		return ReportAnalysisView{}, err
	}
	return computeReportAnalysis(components, roster, gradesByComponent, resolutions, settings.Ranges), nil
}

// computeReportAnalysis adalah fungsi MURNI (tanpa I/O) — mudah dites tanpa
// DB. avg/min/max/count dihitung HANYA atas siswa yang punya nilai
// ter-resolusi (manual atau computed, source != "none"). below_kktp_per_component
// memakai nilai RAW per komponen (BUKAN nilai ter-resolusi).
func computeReportAnalysis(components []ComponentRecord, roster []RosterStudent, gradesByComponent map[int64]map[int64]float64, resolutions map[int64]reportResolution, ranges []GradeRange) ReportAnalysisView {
	var sum, min, max float64
	count := 0
	labelCounts := map[string]int{}
	var src ResolvedSourceCount
	for _, st := range roster {
		res := resolutions[st.ID]
		switch res.Source {
		case "manual":
			src.Manual++
		case "computed":
			src.Computed++
		default:
			src.None++
		}
		if res.Resolved == nil {
			continue
		}
		v := *res.Resolved
		if count == 0 || v < min {
			min = v
		}
		if count == 0 || v > max {
			max = v
		}
		sum += v
		count++
		if label := labelForScore(ranges, roundHalfUp(v)); label != nil {
			labelCounts[*label]++
		}
	}
	avg := 0.0
	if count > 0 {
		avg = sum / float64(count)
	}

	labelNames := make([]string, 0, len(ranges))
	seenLabel := map[string]bool{}
	for _, r := range ranges {
		if !seenLabel[r.Label] {
			seenLabel[r.Label] = true
			labelNames = append(labelNames, r.Label)
		}
	}
	perLabel := make([]LabelCount, 0, len(labelNames))
	for _, l := range labelNames {
		perLabel = append(perLabel, LabelCount{Label: l, Count: labelCounts[l]})
	}

	belowCounts := make([]BelowKktpComponentCount, 0, len(components))
	for _, c := range components {
		cnt := 0
		for _, st := range roster {
			if v, ok := gradesByComponent[c.ID][st.ID]; ok && v < float64(c.Kktp) {
				cnt++
			}
		}
		belowCounts = append(belowCounts, BelowKktpComponentCount{ComponentID: c.ID, Name: c.Name, Count: cnt})
	}

	return ReportAnalysisView{
		Avg: avg, Min: min, Max: max, Count: count,
		PerLabel: perLabel, BelowKktpPerComponent: belowCounts, ResolvedSource: src,
	}
}
