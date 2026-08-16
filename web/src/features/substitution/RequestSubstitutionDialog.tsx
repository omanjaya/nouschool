import { useEffect, useState } from 'react';
import { Dialog } from '../../components/ui/Dialog';
import { Button } from '../../components/ui/Button';
import { Field, Input, Select, Textarea } from '../../components/ui/Field';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { todayISODate } from '../../lib/date';
import { DAY_LABELS } from '../schedule/format';
import { useScheduleSlots } from '../schedule/api';
import { useMe } from '../auth/api';
import { useTeachers } from '../teachers/api';
import { useCreateSubstitution } from './api';

interface RequestSubstitutionDialogProps {
  open: boolean;
  onClose: () => void;
}

/** Dialog "Ajukan Guru Pengganti" — slot dari jadwal mengajar sendiri, tanggal ≥ hari ini, pengganti guru lain. */
export function RequestSubstitutionDialog({ open, onClose }: RequestSubstitutionDialogProps) {
  const { showToast } = useToast();
  const { data: me } = useMe();
  const { data: slots, isLoading: slotsLoading } = useScheduleSlots({}, open);
  const { data: teachers, isLoading: teachersLoading } = useTeachers();
  const createSubstitution = useCreateSubstitution();

  const [slotId, setSlotId] = useState('');
  const [date, setDate] = useState(todayISODate());
  const [substituteId, setSubstituteId] = useState('');
  const [reason, setReason] = useState('');
  const [formError, setFormError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setSlotId('');
      setDate(todayISODate());
      setSubstituteId('');
      setReason('');
      setFormError(null);
      createSubstitution.reset();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const sortedSlots = (slots ?? [])
    .slice()
    .sort((a, b) => a.day_of_week - b.day_of_week || a.period_start - b.period_start);
  const otherTeachers = (teachers ?? []).filter((t) => t.user_id !== me?.id);

  function handleSubmit() {
    setFormError(null);
    if (!slotId) {
      setFormError('Pilih jam mengajar yang akan digantikan.');
      return;
    }
    if (!date) {
      setFormError('Tanggal wajib diisi.');
      return;
    }
    if (date < todayISODate()) {
      setFormError('Tanggal tidak boleh sebelum hari ini.');
      return;
    }
    if (!substituteId) {
      setFormError('Pilih guru pengganti.');
      return;
    }
    if (!reason.trim()) {
      setFormError('Alasan wajib diisi.');
      return;
    }

    createSubstitution.mutate(
      { schedule_slot_id: slotId, date, substitute_user_id: substituteId, reason: reason.trim() },
      {
        onSuccess: () => {
          showToast('Pengajuan guru pengganti terkirim.');
          onClose();
        },
        onError: (err) => {
          setFormError(err instanceof ApiError ? err.message : 'Gagal mengajukan guru pengganti.');
        },
      },
    );
  }

  return (
    <Dialog open={open} onClose={onClose} title="Ajukan Guru Pengganti">
      <div className="flex flex-col gap-4">
        <Field label="Jam mengajar" htmlFor="substitution-slot">
          <Select id="substitution-slot" value={slotId} onChange={(e) => setSlotId(e.target.value)} disabled={slotsLoading}>
            <option value="">{slotsLoading ? 'Memuat jadwal...' : 'Pilih jam...'}</option>
            {sortedSlots.map((slot) => (
              <option key={slot.id} value={slot.id}>
                {DAY_LABELS[slot.day_of_week]} jam {slot.period_start}
                {slot.period_end !== slot.period_start ? `-${slot.period_end}` : ''} · {slot.subject.name} ·{' '}
                {slot.class.name}
              </option>
            ))}
          </Select>
        </Field>

        <Field label="Tanggal" htmlFor="substitution-date">
          <Input
            id="substitution-date"
            type="date"
            min={todayISODate()}
            value={date}
            onChange={(e) => setDate(e.target.value)}
          />
        </Field>

        <Field label="Guru pengganti" htmlFor="substitution-teacher">
          <Select
            id="substitution-teacher"
            value={substituteId}
            onChange={(e) => setSubstituteId(e.target.value)}
            disabled={teachersLoading}
          >
            <option value="">{teachersLoading ? 'Memuat guru...' : 'Pilih guru...'}</option>
            {otherTeachers.map((t) => (
              <option key={t.id} value={t.user_id}>
                {t.name}
              </option>
            ))}
          </Select>
        </Field>

        <Field label="Alasan" htmlFor="substitution-reason">
          <Textarea
            id="substitution-reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            rows={3}
            placeholder="Mis. dinas luar, sakit..."
          />
        </Field>

        {formError && <p className="text-[12px] text-danger">{formError}</p>}

        <div className="flex justify-end gap-2 pt-1">
          <Button type="button" variant="secondary" onClick={onClose}>
            Batal
          </Button>
          <Button type="button" onClick={handleSubmit} loading={createSubstitution.isPending}>
            Ajukan
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
