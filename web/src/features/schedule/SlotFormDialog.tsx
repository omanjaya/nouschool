import { useEffect, useState, type FormEvent } from 'react';
import { Trash2 } from 'lucide-react';
import { Dialog } from '../../components/ui/Dialog';
import { Button } from '../../components/ui/Button';
import { Field, Select } from '../../components/ui/Field';
import { ApiError } from '../../lib/api';
import { useSubjects } from '../subjects/api';
import { useTeachers } from '../teachers/api';
import { DAY_LABELS } from './format';
import { useCreateSlot, useDeleteSlot, useRooms, useUpdateSlot } from './api';
import type { DayOfWeek, Period, ScheduleSlot, ScheduleSlotInput } from '../../lib/types';

interface SlotFormDialogProps {
  open: boolean;
  onClose: () => void;
  classId: string;
  className: string;
  periods: Period[];
  /** Slot yang diedit; kosong = mode tambah. */
  slot?: ScheduleSlot;
  /** Prefill saat dibuka dari sel kosong. */
  defaultDay?: DayOfWeek;
  defaultPeriodNumber?: number;
}

/** Dialog tambah/ubah/hapus satu slot jadwal — hanya dipakai dari mode "Per Kelas" pada builder. */
export function SlotFormDialog({
  open,
  onClose,
  classId,
  className,
  periods,
  slot,
  defaultDay,
  defaultPeriodNumber,
}: SlotFormDialogProps) {
  const isEdit = Boolean(slot);
  const { data: subjects } = useSubjects();
  const { data: teachers } = useTeachers();
  const { data: rooms } = useRooms();
  const createSlot = useCreateSlot();
  const updateSlot = useUpdateSlot(slot?.id ?? '');
  const deleteSlot = useDeleteSlot();
  const mutation = isEdit ? updateSlot : createSlot;

  const kbmPeriods = periods.filter((p) => !p.label).sort((a, b) => a.number - b.number);

  const [form, setForm] = useState<ScheduleSlotInput>(() => emptyForm());
  const [confirmingDelete, setConfirmingDelete] = useState(false);

  function emptyForm(): ScheduleSlotInput {
    const day = defaultDay ?? (1 as DayOfWeek);
    const periodNumber = defaultPeriodNumber ?? kbmPeriods[0]?.number ?? 1;
    return {
      class_id: classId,
      subject_id: '',
      teacher_id: '',
      room_id: undefined,
      day_of_week: day,
      period_start: periodNumber,
      period_end: periodNumber,
    };
  }

  useEffect(() => {
    if (!open) return;
    setConfirmingDelete(false);
    mutation.reset();
    if (slot) {
      setForm({
        class_id: slot.class.id,
        subject_id: slot.subject.id,
        teacher_id: slot.teacher.id,
        room_id: slot.room?.id,
        day_of_week: slot.day_of_week,
        period_start: slot.period_start,
        period_end: slot.period_end,
      });
    } else {
      setForm(emptyForm());
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, slot, defaultDay, defaultPeriodNumber, classId]);

  if (!open) return null;

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    const payload: ScheduleSlotInput = {
      ...form,
      class_id: classId,
      room_id: form.room_id || undefined,
    };
    mutation.mutate(payload, {
      onSuccess: () => onClose(),
    });
  }

  function handleDelete() {
    if (!slot) return;
    deleteSlot.mutate(slot.id, {
      onSuccess: () => onClose(),
    });
  }

  const canSubmit = Boolean(form.subject_id) && Boolean(form.teacher_id) && form.period_end >= form.period_start;

  return (
    <Dialog open={open} onClose={onClose} title={isEdit ? 'Ubah Slot Jadwal' : 'Tambah Slot Jadwal'}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <p className="text-[12px] text-muted">Rombel: {className}</p>

        <div className="grid grid-cols-2 gap-3">
          <Field label="Hari" htmlFor="slot-day">
            <Select
              id="slot-day"
              value={String(form.day_of_week)}
              onChange={(e) => setForm((f) => ({ ...f, day_of_week: Number(e.target.value) as DayOfWeek }))}
            >
              {Object.entries(DAY_LABELS).map(([value, label]) => (
                <option key={value} value={value}>
                  {label}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Mata pelajaran" htmlFor="slot-subject">
            <Select
              id="slot-subject"
              value={form.subject_id}
              onChange={(e) => setForm((f) => ({ ...f, subject_id: e.target.value }))}
            >
              <option value="">Pilih mapel</option>
              {subjects?.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.code} — {s.name}
                </option>
              ))}
            </Select>
          </Field>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <Field label="Jam mulai" htmlFor="slot-period-start">
            <Select
              id="slot-period-start"
              value={String(form.period_start)}
              onChange={(e) => {
                const value = Number(e.target.value);
                setForm((f) => ({ ...f, period_start: value, period_end: Math.max(value, f.period_end) }));
              }}
            >
              {kbmPeriods.map((p) => (
                <option key={p.id} value={p.number}>
                  Ke-{p.number} ({p.starts_at})
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Jam selesai" htmlFor="slot-period-end">
            <Select
              id="slot-period-end"
              value={String(form.period_end)}
              onChange={(e) => setForm((f) => ({ ...f, period_end: Number(e.target.value) }))}
            >
              {kbmPeriods
                .filter((p) => p.number >= form.period_start)
                .map((p) => (
                  <option key={p.id} value={p.number}>
                    Ke-{p.number} ({p.ends_at})
                  </option>
                ))}
            </Select>
          </Field>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <Field label="Guru" htmlFor="slot-teacher">
            <Select
              id="slot-teacher"
              value={form.teacher_id}
              onChange={(e) => setForm((f) => ({ ...f, teacher_id: e.target.value }))}
            >
              <option value="">Pilih guru</option>
              {teachers?.map((t) => (
                <option key={t.id} value={t.id}>
                  {t.name}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Ruangan" htmlFor="slot-room" hint="Opsional">
            <Select
              id="slot-room"
              value={form.room_id ?? ''}
              onChange={(e) => setForm((f) => ({ ...f, room_id: e.target.value || undefined }))}
            >
              <option value="">Tanpa ruangan tetap</option>
              {rooms?.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.name}
                </option>
              ))}
            </Select>
          </Field>
        </div>

        {mutation.isError && (
          <p className="text-[12px] text-danger">
            {mutation.error instanceof ApiError ? mutation.error.message : 'Gagal menyimpan slot jadwal.'}
          </p>
        )}
        {deleteSlot.isError && (
          <p className="text-[12px] text-danger">
            {deleteSlot.error instanceof ApiError ? deleteSlot.error.message : 'Gagal menghapus slot jadwal.'}
          </p>
        )}

        {confirmingDelete ? (
          <div className="flex flex-col gap-3 rounded-lg border border-line bg-surface-2 p-3">
            <p className="text-[13px] text-ink">Hapus slot ini dari jadwal?</p>
            <div className="flex justify-end gap-2">
              <Button type="button" variant="secondary" onClick={() => setConfirmingDelete(false)}>
                Batal
              </Button>
              <Button type="button" variant="danger" loading={deleteSlot.isPending} onClick={handleDelete}>
                Hapus
              </Button>
            </div>
          </div>
        ) : (
          <div className="flex items-center justify-between gap-2 pt-2">
            {isEdit ? (
              <button
                type="button"
                onClick={() => setConfirmingDelete(true)}
                className="inline-flex items-center gap-1.5 text-[13px] font-medium text-danger hover:opacity-80"
              >
                <Trash2 size={16} strokeWidth={2} aria-hidden="true" />
                Hapus
              </button>
            ) : (
              <span />
            )}
            <div className="flex gap-2">
              <Button type="button" variant="secondary" onClick={onClose}>
                Batal
              </Button>
              <Button type="submit" loading={mutation.isPending} disabled={!canSubmit}>
                Simpan
              </Button>
            </div>
          </div>
        )}
      </form>
    </Dialog>
  );
}
