import { useEffect, useState } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { CalendarDays, Plus } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Tag } from '../../components/ui/Tag';
import { Button } from '../../components/ui/Button';
import { Field, Select } from '../../components/ui/Field';
import { formatDateRange } from '../../lib/date';
import { useMe } from '../auth/api';
import { useStudentLeaveChildren, useStudentLeaveRequests } from './api';
import { StudentLeaveRequestFormDialog } from './StudentLeaveRequestFormDialog';
import { STUDENT_LEAVE_STATUS_LABEL, STUDENT_LEAVE_STATUS_TAG_VARIANT, STUDENT_LEAVE_TYPE_LABEL } from './format';
import type { StudentLeaveRequest } from '../../lib/types';

function StudentLeaveList({
  items,
  isLoading,
  isError,
  onRetry,
  onOpenDetail,
  emptyMessage,
}: {
  items: StudentLeaveRequest[] | undefined;
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  onOpenDetail: (id: string) => void;
  emptyMessage: string;
}) {
  if (isLoading) {
    return (
      <div className="flex flex-col gap-2">
        <Skeleton className="h-16 w-full" />
        <Skeleton className="h-16 w-full" />
      </div>
    );
  }
  if (isError) {
    return <ErrorState message="Gagal memuat pengajuan izin." onRetry={onRetry} />;
  }
  if (!items || items.length === 0) {
    return <EmptyState icon={CalendarDays} message={emptyMessage} />;
  }
  return (
    <div>
      {items.map((item) => (
        <ListRow
          key={item.id}
          className="min-h-[56px]"
          title={STUDENT_LEAVE_TYPE_LABEL[item.type]}
          subtitle={
            <span className="flex flex-col gap-0.5">
              <span className="num">{formatDateRange(item.date_start, item.date_end)}</span>
              <span className="truncate">{item.reason}</span>
            </span>
          }
          trailing={
            <Tag variant={STUDENT_LEAVE_STATUS_TAG_VARIANT[item.status]}>
              {item.status === 'issued' && item.letter_number
                ? `Terbit · ${item.letter_number}`
                : STUDENT_LEAVE_STATUS_LABEL[item.status]}
            </Tag>
          }
          onClick={() => onOpenDetail(item.id)}
        />
      ))}
    </div>
  );
}

function SiswaMyLeave({ studentId }: { studentId?: string | null }) {
  const navigate = useNavigate();
  const [formOpen, setFormOpen] = useState(false);
  const { data: items, isLoading, isError, refetch } = useStudentLeaveRequests('mine');

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Izin</p>
        <h1 className="text-[21px] font-semibold text-ink">Izin Saya</h1>
      </div>

      <Button onClick={() => setFormOpen(true)} className="self-start">
        <Plus size={16} strokeWidth={2} aria-hidden="true" />
        Ajukan Izin
      </Button>

      <div className="flex flex-col gap-3">
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Pengajuan Saya</p>
        <StudentLeaveList
          items={items}
          isLoading={isLoading}
          isError={isError}
          onRetry={() => refetch()}
          onOpenDetail={(id) => navigate(`/izin-saya/${id}`)}
          emptyMessage="Belum ada pengajuan izin."
        />
      </div>

      <StudentLeaveRequestFormDialog
        open={formOpen}
        onClose={() => setFormOpen(false)}
        onSubmitted={() => setFormOpen(false)}
        studentId={studentId ?? undefined}
      />
    </div>
  );
}

function OrangTuaMyLeave() {
  const navigate = useNavigate();
  const [formOpen, setFormOpen] = useState(false);
  const { data: children, isLoading: childrenLoading, isError: childrenError, refetch: refetchChildren } =
    useStudentLeaveChildren();
  const [selectedId, setSelectedId] = useState<string | undefined>(undefined);
  // `scope=mine` untuk ortu mengembalikan gabungan pengajuan SEMUA anaknya (tiap
  // baris sudah punya `student` ref) — filter per anak dilakukan di client lewat
  // Select anak, karena kontrak GET tidak punya parameter `student_id`.
  const { data: items, isLoading: itemsLoading, isError: itemsError, refetch: refetchItems } = useStudentLeaveRequests(
    'mine',
    undefined,
    Boolean(children && children.length > 0),
  );

  useEffect(() => {
    if (children && children.length > 0 && !selectedId) {
      setSelectedId(children[0].student_id);
    }
  }, [children, selectedId]);

  const filteredItems = items?.filter((i) => i.student.id === selectedId);

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Izin</p>
        <h1 className="text-[21px] font-semibold text-ink">Izin Anak</h1>
      </div>

      {childrenLoading ? (
        <Skeleton className="h-20 w-full" />
      ) : childrenError ? (
        <ErrorState message="Gagal memuat data anak." onRetry={() => refetchChildren()} />
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

          <Button onClick={() => setFormOpen(true)} className="self-start">
            <Plus size={16} strokeWidth={2} aria-hidden="true" />
            Ajukan Izin
          </Button>

          <div className="flex flex-col gap-3">
            <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Pengajuan</p>
            <StudentLeaveList
              items={filteredItems}
              isLoading={itemsLoading}
              isError={itemsError}
              onRetry={() => refetchItems()}
              onOpenDetail={(id) => navigate(`/izin-saya/${id}`)}
              emptyMessage="Belum ada pengajuan izin untuk anak ini."
            />
          </div>

          <StudentLeaveRequestFormDialog
            open={formOpen}
            onClose={() => setFormOpen(false)}
            onSubmitted={() => setFormOpen(false)}
            studentId={selectedId}
          />
        </>
      )}
    </div>
  );
}

/** /izin-saya — pengajuan izin terencana: siswa (dirinya) & orang tua (pilih anak). */
export function MyStudentLeavePage() {
  const { data: me } = useMe();

  if (!me) return null;
  if (me.role === 'siswa') return <SiswaMyLeave studentId={me.student_id} />;
  if (me.role === 'orang_tua') return <OrangTuaMyLeave />;
  return <Navigate to="/" replace />;
}
