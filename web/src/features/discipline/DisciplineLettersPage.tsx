import { useState } from 'react';
import { ExternalLink, FileWarning } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { Tag } from '../../components/ui/Tag';
import { Button } from '../../components/ui/Button';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { SegmentedControl, type SegmentedOption } from '../../components/ui/SegmentedControl';
import { formatDate } from '../../lib/date';
import { disciplineLetterPdfUrl, useDisciplineLetters, type DisciplineLettersFilter } from './api';
import { SP_LEVEL_TAG_VARIANT, spLevelLabel } from './format';

type LevelFilter = '' | '1' | '2' | '3';

const LEVEL_OPTIONS: SegmentedOption<LevelFilter>[] = [
  { value: '', label: 'Semua' },
  { value: '1', label: 'SP1' },
  { value: '2', label: 'SP2' },
  { value: '3', label: 'SP3' },
];

/** Tab "Surat Peringatan" — daftar surat terbit, filter level, lihat PDF/HTML. */
export function DisciplineLettersPage() {
  const [level, setLevel] = useState<LevelFilter>('');
  const filter: DisciplineLettersFilter = level ? { level } : {};

  const { data, isLoading, isError, refetch, hasNextPage, fetchNextPage, isFetchingNextPage } =
    useDisciplineLetters(filter);
  const items = data?.pages.flatMap((p) => p.items) ?? [];

  return (
    <div className="flex flex-col gap-4">
      <SegmentedControl options={LEVEL_OPTIONS} value={level} onChange={setLevel} />

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat surat peringatan." onRetry={() => refetch()} />
      ) : items.length === 0 ? (
        <EmptyState icon={FileWarning} message="Belum ada Surat Peringatan yang terbit." />
      ) : (
        <div className="flex flex-col gap-3">
          <div>
            {items.map((letter) => (
              <ListRow
                key={letter.id}
                title={<span className="num">{letter.number}</span>}
                subtitle={
                  <span className="truncate">
                    {letter.student?.name ?? 'Siswa tidak diketahui'} · {letter.points_snapshot} poin ·{' '}
                    {formatDate(letter.created_at)}
                  </span>
                }
                trailing={
                  <div className="flex items-center gap-1.5">
                    <Tag variant={SP_LEVEL_TAG_VARIANT[letter.level]}>{spLevelLabel(letter.level)}</Tag>
                    <a
                      href={disciplineLetterPdfUrl(letter.id)}
                      target="_blank"
                      rel="noreferrer"
                      aria-label={`Lihat surat ${letter.number}`}
                      className="flex h-9 w-9 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-ink"
                    >
                      <ExternalLink size={16} strokeWidth={2} aria-hidden="true" />
                    </a>
                  </div>
                }
              />
            ))}
          </div>

          {hasNextPage && (
            <Button
              variant="secondary"
              onClick={() => fetchNextPage()}
              loading={isFetchingNextPage}
              className="self-center"
            >
              Muat lebih banyak
            </Button>
          )}
        </div>
      )}
    </div>
  );
}
