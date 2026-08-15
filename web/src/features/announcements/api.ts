import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { Announcement, AnnouncementInput } from '../../lib/types';

export const ANNOUNCEMENTS_QUERY_KEY = 'announcements' as const;

/**
 * GET /api/announcements(?active=1) — `active=1` (dipakai TV & kartu ringkas)
 * mengembalikan hanya pengumuman yang rentang tanggalnya mencakup hari ini;
 * tanpa parameter (dipakai `/pengumuman` CRUD) mengembalikan semua.
 */
export function useAnnouncements(activeOnly = false, enabled = true) {
  return useQuery({
    queryKey: [ANNOUNCEMENTS_QUERY_KEY, activeOnly],
    queryFn: () => api.get<Announcement[]>(`/announcements${activeOnly ? '?active=1' : ''}`),
    enabled,
  });
}

export function useCreateAnnouncement() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: AnnouncementInput) => api.post<Announcement>('/announcements', input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [ANNOUNCEMENTS_QUERY_KEY] }),
  });
}

export function useUpdateAnnouncement(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: AnnouncementInput) => api.patch<Announcement>(`/announcements/${id}`, input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [ANNOUNCEMENTS_QUERY_KEY] }),
  });
}

export function useDeleteAnnouncement() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete<undefined>(`/announcements/${id}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [ANNOUNCEMENTS_QUERY_KEY] }),
  });
}
