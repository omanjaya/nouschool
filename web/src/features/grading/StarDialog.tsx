import { useEffect, useState } from 'react';
import { Dialog } from '../../components/ui/Dialog';
import { Button } from '../../components/ui/Button';
import { Field, Select, Textarea } from '../../components/ui/Field';
import { SegmentedControl } from '../../components/ui/SegmentedControl';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { useCreateGradingStar } from './api';
import type { GradingStarVisibility } from '../../lib/types';

export interface StarDialogStudent {
  id: string;
  name: string;
}

interface StarDialogProps {
  open: boolean;
  onClose: () => void;
  student: StarDialogStudent | null;
  /** Preset arah bintang dari tombol yang ditekan (Star = +1, StarOff = -1) — tetap bisa diganti di dialog. */
  initialDelta: 1 | -1;
  onSaved?: () => void;
}

/**
 * Dialog "Beri Bintang" — dipakai dari aksi baris tab Rekap `/nilai` (ikon
 * Star/StarOff). SegmentedControl arah (+1/-1) + catatan opsional +
 * visibility (terlihat siswa/privat guru), POST /api/grading/stars.
 */
export function StarDialog({ open, onClose, student, initialDelta, onSaved }: StarDialogProps) {
  const { showToast } = useToast();
  const [delta, setDelta] = useState<'1' | '-1'>('1');
  const [note, setNote] = useState('');
  const [visibility, setVisibility] = useState<GradingStarVisibility>('student');
  const [formError, setFormError] = useState<string | null>(null);
  const createStar = useCreateGradingStar();

  useEffect(() => {
    if (open) {
      setDelta(initialDelta === -1 ? '-1' : '1');
      setNote('');
      setVisibility('student');
      setFormError(null);
      createStar.reset();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, initialDelta, student?.id]);

  function handleSubmit() {
    if (!student) return;
    setFormError(null);
    createStar.mutate(
      { student_id: student.id, delta: Number(delta), note: note.trim() || undefined, visibility },
      {
        onSuccess: () => {
          showToast('Bintang tercatat.');
          onSaved?.();
          onClose();
        },
        onError: (err) => setFormError(err instanceof ApiError ? err.message : 'Gagal mencatat bintang.'),
      },
    );
  }

  return (
    <Dialog open={open} onClose={onClose} title="Beri Bintang">
      <div className="flex flex-col gap-4">
        {student && (
          <Field label="Siswa">
            <div className="rounded-lg border border-line bg-surface-2 px-3 py-2.5">
              <p className="truncate text-[14px] text-ink">{student.name}</p>
            </div>
          </Field>
        )}

        <Field label="Bintang">
          <SegmentedControl
            options={[
              { value: '1', label: '+1 Bintang' },
              { value: '-1', label: '-1 Bintang' },
            ]}
            value={delta}
            onChange={setDelta}
          />
        </Field>

        <Field label="Catatan (opsional)" htmlFor="star-note">
          <Textarea
            id="star-note"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            rows={3}
            placeholder="Mis. alasan pemberian bintang..."
          />
        </Field>

        <Field label="Terlihat oleh" htmlFor="star-visibility">
          <Select id="star-visibility" value={visibility} onChange={(e) => setVisibility(e.target.value as GradingStarVisibility)}>
            <option value="student">Terlihat siswa</option>
            <option value="private">Privat guru</option>
          </Select>
        </Field>

        {formError && <p className="text-[12px] text-danger">{formError}</p>}

        <div className="flex justify-end gap-2 pt-1">
          <Button type="button" variant="secondary" onClick={onClose}>
            Batal
          </Button>
          <Button type="button" onClick={handleSubmit} loading={createStar.isPending}>
            Simpan
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
