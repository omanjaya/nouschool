import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { ImportCommitResult, ImportPreviewResult } from '../../lib/types';

/** `dapodik` = import peserta didik dari file export aplikasi Dapodik (docs/03-student.md), bukan template NouSchool sendiri. */
export type ImportEntity = 'students' | 'teachers' | 'schedule' | 'dapodik';

export const IMPORT_ENTITY_LABEL: Record<ImportEntity, string> = {
  students: 'siswa',
  teachers: 'guru',
  schedule: 'jadwal',
  dapodik: 'Dapodik',
};

/** Entity dengan template Excel NouSchool sendiri — `dapodik` memakai format export bawaan Dapodik, jadi tidak ada template untuk diunduh. */
export const IMPORT_ENTITY_HAS_TEMPLATE: Record<ImportEntity, boolean> = {
  students: true,
  teachers: true,
  schedule: true,
  dapodik: false,
};

/** URL template CSV — dibuka langsung lewat <a href>, bukan fetch (lihat lib/api.ts). */
export function importTemplateUrl(entity: ImportEntity): string {
  return `/api/import/${entity}/template`;
}

/** POST /api/import/{entity} multipart — upload lalu terima preview & upload_id. */
export function useImportPreview(entity: ImportEntity) {
  return useMutation({
    mutationFn: (file: File) => {
      const formData = new FormData();
      formData.append('file', file);
      return api.upload<ImportPreviewResult>(`/import/${entity}`, formData);
    },
  });
}

/** POST /api/import/{entity}/commit — commit transaksional dari upload_id hasil preview. */
export function useImportCommit(entity: ImportEntity) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (uploadId: string) =>
      api.post<ImportCommitResult>(`/import/${entity}/commit`, { upload_id: uploadId }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [entity] });
      // Slot jadwal disimpan di bawah query key 'schedule-slots' (features/schedule/api.ts),
      // bukan 'schedule' — invalidasi tambahan supaya builder & grid ikut refresh.
      if (entity === 'schedule') {
        queryClient.invalidateQueries({ queryKey: ['schedule-slots'] });
        queryClient.invalidateQueries({ queryKey: ['schedule-today'] });
      }
      // Import Dapodik menulis ke tabel `students` yang sama dengan import
      // biasa (docs/03-student.md) — query key-nya 'students', bukan 'dapodik'.
      if (entity === 'dapodik') {
        queryClient.invalidateQueries({ queryKey: ['students'] });
      }
    },
  });
}
