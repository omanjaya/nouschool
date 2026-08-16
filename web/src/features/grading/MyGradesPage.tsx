import { useEffect, useState } from 'react';
import { Navigate } from 'react-router-dom';
import { ChevronDown, ChevronUp, GraduationCap } from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { Tag } from '../../components/ui/Tag';
import { Field, Select } from '../../components/ui/Field';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { useMe } from '../auth/api';
import { useGradingChildren, useGradingStatus, useMyGrades } from './api';
import { GRADING_TYPE_LABEL } from './format';
import type { MyGradesSubject } from '../../lib/types';

function SubjectCard({ item }: { item: MyGradesSubject }) {
  const [open, setOpen] = useState(false);

  return (
    <Card className="flex flex-col gap-3">
      <button type="button" onClick={() => setOpen((v) => !v)} className="flex items-center justify-between gap-3 text-left">
        <div className="min-w-0">
          <p className="truncate text-[14px] font-semibold text-ink">{item.subject.name}</p>
          <p className="truncate text-[12px] text-muted">{item.subject.code}</p>
        </div>
        <div className="flex shrink-0 items-center gap-3">
          <span className="num text-[28px] font-semibold text-ink">{item.final ?? '-'}</span>
          {item.label && <Tag variant="neutral">{item.label}</Tag>}
          {open ? (
            <ChevronUp size={18} strokeWidth={2} className="text-muted" aria-hidden="true" />
          ) : (
            <ChevronDown size={18} strokeWidth={2} className="text-muted" aria-hidden="true" />
          )}
        </div>
      </button>

      {open && (
        <div className="overflow-x-auto border-t border-line pt-3">
          <table className="w-full min-w-[420px] border-collapse text-[13px]">
            <thead>
              <tr className="text-left text-[11px] font-semibold uppercase tracking-[0.05em] text-muted">
                <th className="px-2 py-1.5">Komponen</th>
                <th className="px-2 py-1.5">Tipe</th>
                <th className="px-2 py-1.5 text-right">Bobot</th>
                <th className="px-2 py-1.5 text-right">KKTP</th>
                <th className="px-2 py-1.5 text-right">Skor</th>
              </tr>
            </thead>
            <tbody>
              {item.components.map((c, i) => {
                const below = c.score !== null && c.score < c.kktp;
                return (
                  <tr key={i} className="border-t border-line">
                    <td className="px-2 py-1.5 text-ink">{c.name}</td>
                    <td className="px-2 py-1.5">
                      <Tag variant="neutral">{GRADING_TYPE_LABEL[c.type]}</Tag>
                    </td>
                    <td className="num px-2 py-1.5 text-right text-muted">{c.weight}</td>
                    <td className="num px-2 py-1.5 text-right text-muted">{c.kktp}</td>
                    <td className={`num px-2 py-1.5 text-right ${below ? 'text-danger' : 'text-ink'}`}>{c.score ?? '-'}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </Card>
  );
}

function MyGradesBody({ studentId }: { studentId?: string }) {
  const { data, isLoading, isError, refetch } = useMyGrades(studentId);

  if (isLoading) {
    return (
      <div className="flex flex-col gap-3">
        <Skeleton className="h-20 w-full" />
        <Skeleton className="h-20 w-full" />
      </div>
    );
  }

  if (isError || !data) {
    return <ErrorState message="Gagal memuat nilai." onRetry={() => refetch()} />;
  }

  if (data.subjects.length === 0) {
    return <EmptyState icon={GraduationCap} message="Belum ada nilai yang dipublikasikan." />;
  }

  return (
    <div className="flex flex-col gap-3">
      {data.subjects.map((item) => (
        <SubjectCard key={item.subject.id} item={item} />
      ))}
    </div>
  );
}

/** /nilai-saya — nilai siswa (dirinya) & orang tua (pilih anak), pola sama `MyDisciplinePage`. */
export function MyGradesPage() {
  const { data: me } = useMe();
  const { data: status, isLoading: statusLoading, isError: statusError, refetch: refetchStatus } = useGradingStatus();
  const { data: children, isLoading: childrenLoading, isError: childrenError, refetch: refetchChildren } = useGradingChildren(
    me?.role === 'orang_tua',
  );
  const [selectedChild, setSelectedChild] = useState<string | undefined>();

  useEffect(() => {
    if (children && children.length > 0 && !selectedChild) setSelectedChild(children[0].student_id);
  }, [children, selectedChild]);

  if (!me) return null;
  if (me.role !== 'siswa' && me.role !== 'orang_tua') return <Navigate to="/" replace />;

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Penilaian</p>
        <h1 className="text-[21px] font-semibold text-ink">{me.role === 'siswa' ? 'Nilai Saya' : 'Nilai Anak'}</h1>
      </div>

      {statusLoading ? (
        <Skeleton className="h-40 w-full" />
      ) : statusError ? (
        <ErrorState message="Gagal memuat status modul penilaian." onRetry={() => refetchStatus()} />
      ) : !status?.enabled ? (
        <EmptyState icon={GraduationCap} message="Modul penilaian tidak aktif." />
      ) : me.role === 'siswa' ? (
        <MyGradesBody />
      ) : childrenLoading ? (
        <Skeleton className="h-20 w-full" />
      ) : childrenError ? (
        <ErrorState message="Gagal memuat data anak." onRetry={() => refetchChildren()} />
      ) : !children || children.length === 0 ? (
        <EmptyState message="Belum ada anak yang terhubung ke akun ini." />
      ) : (
        <>
          {children.length > 1 && (
            <Field label="Pilih anak">
              <Select value={selectedChild} onChange={(e) => setSelectedChild(e.target.value)}>
                {children.map((c) => (
                  <option key={c.student_id} value={c.student_id}>
                    {c.name} — {c.class_name}
                  </option>
                ))}
              </Select>
            </Field>
          )}
          {selectedChild && <MyGradesBody studentId={selectedChild} />}
        </>
      )}
    </div>
  );
}
