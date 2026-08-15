import { useNavigate } from 'react-router-dom';
import { Bell } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Button } from '../../components/ui/Button';
import { formatRelativeTime } from '../../lib/date';
import { useMarkNotificationsRead, useNotifications } from './api';
import type { NotificationItem } from '../../lib/types';

/** /notifikasi — inbox notifikasi in-app (docs/08-notification.md), sama untuk semua peran non-display. */
export function NotificationsPage() {
  const navigate = useNavigate();
  const { data, isLoading, isError, refetch, hasNextPage, fetchNextPage, isFetchingNextPage } = useNotifications();
  const markRead = useMarkNotificationsRead();

  const items = data?.pages.flatMap((p) => p.items) ?? [];
  const unreadCount = data?.pages[0]?.unread_count ?? 0;

  function handleItemClick(item: NotificationItem) {
    if (!item.read_at) {
      markRead.mutate({ ids: [item.id] });
    }
    if (item.link) {
      navigate(item.link);
    }
  }

  function handleMarkAllRead() {
    markRead.mutate({ all: true });
  }

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Akun</p>
          <h1 className="text-[21px] font-semibold text-ink">Notifikasi</h1>
        </div>
        {unreadCount > 0 && (
          <Button variant="secondary" onClick={handleMarkAllRead} loading={markRead.isPending}>
            Tandai semua dibaca
          </Button>
        )}
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat notifikasi." onRetry={() => refetch()} />
      ) : items.length === 0 ? (
        <EmptyState icon={Bell} message="Belum ada notifikasi." />
      ) : (
        <div className="flex flex-col gap-4">
          <div>
            {items.map((item) => (
              <ListRow
                key={item.id}
                className="min-h-[64px] items-start"
                onClick={() => handleItemClick(item)}
                leading={
                  !item.read_at ? (
                    <span aria-hidden="true" className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-primary" />
                  ) : (
                    <span aria-hidden="true" className="mt-1.5 h-1.5 w-1.5 shrink-0" />
                  )
                }
                title={<span className={!item.read_at ? 'font-semibold' : ''}>{item.title}</span>}
                subtitle={
                  <span className="flex flex-col gap-0.5 py-0.5">
                    <span className="line-clamp-2">{item.body}</span>
                    <span className="text-[11px] text-muted">{formatRelativeTime(item.created_at)}</span>
                  </span>
                }
                subtitleClassName="block text-[12px] text-muted"
              />
            ))}
          </div>

          {hasNextPage && (
            <Button variant="secondary" onClick={() => fetchNextPage()} loading={isFetchingNextPage} className="self-center">
              Muat lebih banyak
            </Button>
          )}
        </div>
      )}
    </div>
  );
}
