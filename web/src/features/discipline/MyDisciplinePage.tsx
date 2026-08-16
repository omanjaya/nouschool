import { useEffect, useState } from 'react';
import { Navigate } from 'react-router-dom';
import { ShieldCheck } from 'lucide-react';
import { StatTile } from '../../components/ui/StatTile';
import { Tag } from '../../components/ui/Tag';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Field, Select } from '../../components/ui/Field';
import { formatDate } from '../../lib/date';
import { useMe } from '../auth/api';
import { disciplineLetterPdfUrl, useDisciplineChildren, useStudentDiscipline } from './api';
import { SP_LEVEL_TAG_VARIANT, spLevelLabel } from './format';
import type { DisciplineLetterLevel } from '../../lib/types';

function MyDisciplineBody({ studentId }: { studentId: string }) {
  const { data, isLoading, isError, refetch } = useStudentDiscipline(studentId);

  if (isLoading) {
    return (
      <div className="flex flex-col gap-3">
        <Skeleton className="h-20 w-full" />
        <Skeleton className="h-14 w-full" />
      </div>
    );
  }

  if (isError || !data) {
    return <ErrorState message="Gagal memuat data kedisiplinan." onRetry={() => refetch()} />;
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center gap-4">
        {/* Ambang SP tidak tersedia untuk siswa/ortu (hanya admin, GET /discipline/sp-settings) —
            angka ditampilkan netral, bukan berwarna seperti Rekap admin. */}
        <StatTile label="Total Poin" value={data.total_points} />
        {data.sp_level_reached > 0 && (
          <Tag variant={SP_LEVEL_TAG_VARIANT[data.sp_level_reached as DisciplineLetterLevel]}>
            {spLevelLabel(data.sp_level_reached)}
          </Tag>
        )}
      </div>

      <div>
        <p className="mb-1 text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Riwayat Pelanggaran</p>
        {data.records.length === 0 ? (
          <EmptyState icon={ShieldCheck} message="Tidak ada catatan pelanggaran. Pertahankan!" />
        ) : (
          <div>
            {data.records.map((r) => (
              <ListRow
                key={r.id}
                title={r.violation.name}
                subtitle={
                  <span className="flex flex-col gap-0.5">
                    <span>{formatDate(r.occurred_on)}</span>
                    {r.note && <span className="truncate">{r.note}</span>}
                  </span>
                }
                trailing={<Tag variant="danger">+{r.violation.points} poin</Tag>}
              />
            ))}
          </div>
        )}
      </div>

      {data.letters.length > 0 && (
        <div>
          <p className="mb-1 text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Surat Peringatan</p>
          <div>
            {data.letters.map((letter) => (
              <ListRow
                key={letter.id}
                title={<span className="num">{letter.number}</span>}
                subtitle={formatDate(letter.created_at)}
                trailing={
                  <div className="flex items-center gap-2">
                    <Tag variant={SP_LEVEL_TAG_VARIANT[letter.level]}>{spLevelLabel(letter.level)}</Tag>
                    <a
                      href={disciplineLetterPdfUrl(letter.id)}
                      target="_blank"
                      rel="noreferrer"
                      className="text-[12px] font-semibold text-primary hover:underline"
                    >
                      Lihat
                    </a>
                  </div>
                }
              />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function SiswaMyDiscipline({ studentId }: { studentId?: string | null }) {
  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Kedisiplinan</p>
        <h1 className="text-[21px] font-semibold text-ink">Poin Kedisiplinan</h1>
      </div>
      {studentId ? (
        <MyDisciplineBody studentId={studentId} />
      ) : (
        <ErrorState message="Data siswa untuk akun ini tidak ditemukan." />
      )}
    </div>
  );
}

function OrangTuaMyDiscipline() {
  const { data: children, isLoading, isError, refetch } = useDisciplineChildren();
  const [selectedId, setSelectedId] = useState<string | undefined>(undefined);

  useEffect(() => {
    if (children && children.length > 0 && !selectedId) {
      setSelectedId(children[0].student_id);
    }
  }, [children, selectedId]);

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Kedisiplinan</p>
        <h1 className="text-[21px] font-semibold text-ink">Poin Kedisiplinan Anak</h1>
      </div>

      {isLoading ? (
        <Skeleton className="h-20 w-full" />
      ) : isError ? (
        <ErrorState message="Gagal memuat data anak." onRetry={() => refetch()} />
      ) : !children || children.length === 0 ? (
        <EmptyState message="Belum ada anak yang terhubung ke akun ini." />
      ) : (
        <>
          {children.length > 1 && (
            <Field label="Pilih anak">
              <Select value={selectedId} onChange={(e) => setSelectedId(e.target.value)}>
                {children.map((c) => (
                  <option key={c.student_id} value={c.student_id}>
                    {c.name} — {c.class_name}
                  </option>
                ))}
              </Select>
            </Field>
          )}
          {selectedId && <MyDisciplineBody studentId={selectedId} />}
        </>
      )}
    </div>
  );
}

/** /pelanggaran-saya — poin kedisiplinan untuk siswa (dirinya) & orang tua (pilih anak). */
export function MyDisciplinePage() {
  const { data: me } = useMe();

  if (!me) return null;
  if (me.role === 'siswa') return <SiswaMyDiscipline studentId={me.student_id} />;
  if (me.role === 'orang_tua') return <OrangTuaMyDiscipline />;
  return <Navigate to="/" replace />;
}
