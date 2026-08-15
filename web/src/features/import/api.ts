import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { ImportCommitResult, ImportPreviewResult } from '../../lib/types';

export type ImportEntity = 'students' | 'teachers';

export const IMPORT_ENTITY_LABEL: Record<ImportEntity, string> = {
  students: 'siswa',
  teachers: 'guru',
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
    },
  });
}
