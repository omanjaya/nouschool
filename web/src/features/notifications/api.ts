import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { NotificationListResult, NotificationsReadInput } from '../../lib/types';

export const NOTIFICATIONS_QUERY_KEY = 'notifications' as const;
/** Query key khusus ringkasan halaman 1 — sumber badge unread di AppShell (docs/08). */
export const NOTIFICATIONS_UNREAD_QUERY_KEY = [NOTIFICATIONS_QUERY_KEY, 'unread-summary'] as const;

/** Daftar notifikasi berhalaman untuk `/notifikasi`, dimuat bertahap lewat "Muat lebih banyak". */
export function useNotifications() {
  return useInfiniteQuery({
    queryKey: [NOTIFICATIONS_QUERY_KEY, 'list'],
    queryFn: ({ pageParam }) => api.get<NotificationListResult>(`/notifications?page=${pageParam}`),
    initialPageParam: 1,
    getNextPageParam: (lastPage, allPages) => (lastPage.items.length > 0 ? allPages.length + 1 : undefined),
    staleTime: 30 * 1000,
  });
}

/**
 * Ringkasan unread untuk badge nav (AppShell) — halaman pertama saja.
 * staleTime 30 dtk, refresh saat window focus + polling 60 dtk (spesifikasi Fase 9).
 */
export function useUnreadNotificationCount(enabled: boolean) {
  return useQuery({
    queryKey: NOTIFICATIONS_UNREAD_QUERY_KEY,
    queryFn: () => api.get<NotificationListResult>('/notifications?page=1'),
    enabled,
    staleTime: 30 * 1000,
    refetchOnWindowFocus: true,
    refetchInterval: 60 * 1000,
    select: (data) => data.unread_count,
  });
}

export function useMarkNotificationsRead() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: NotificationsReadInput) => api.post<undefined>('/notifications/read', input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [NOTIFICATIONS_QUERY_KEY] });
    },
  });
}
