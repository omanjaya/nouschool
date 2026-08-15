import { useEffect, useState } from 'react';
import { Dialog } from '../../components/ui/Dialog';
import { Field, Textarea } from '../../components/ui/Field';
import { Button } from '../../components/ui/Button';
import { Tag } from '../../components/ui/Tag';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { formatDate, formatTimeOfDay } from '../../lib/date';
import { useEndJournal, useUpdateJournal } from './api';
import type { Journal } from '../../lib/types';

interface JournalDetailDialogProps {
  open: boolean;
  journal: Journal | null;
  onClose: () => void;
}

const FLAG_LABEL: Record<string, string> = {
  room_mismatch: 'Ruang beda',
  unscheduled: 'Di luar jadwal',
  manual_pick: 'Dipilih manual',
};

/** Dialog detail jurnal — isi/ubah materi & catatan, akhiri sesi bila masih berlangsung. */
export function JournalDetailDialog({ open, journal, onClose }: JournalDetailDialogProps) {
  const { showToast } = useToast();
  const updateJournal = useUpdateJournal(journal?.id ?? '');
  const endJournal = useEndJournal(journal?.id ?? '');
  const [material, setMaterial] = useState('');
  const [note, setNote] = useState('');
  const [confirmingEnd, setConfirmingEnd] = useState(false);

  useEffect(() => {
    if (open && journal) {
      setMaterial(journal.material ?? '');
      setNote(journal.note ?? '');
      setConfirmingEnd(false);
      updateJournal.reset();
      endJournal.reset();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, journal?.id]);

  if (!journal) return null;

  const timeRange = `${formatTimeOfDay(journal.started_at)}${
    journal.ended_at ? `–${formatTimeOfDay(journal.ended_at)}` : ' – berlangsung'
  }`;

  async function handleSave() {
    if (!journal) return;
    try {
      await updateJournal.mutateAsync({ material: material.trim(), note: note.trim() });
      showToast('Jurnal tersimpan.');
      onClose();
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : 'Gagal menyimpan jurnal.', 'error');
    }
  }

  async function handleEnd() {
    if (!journal) return;
    try {
      await endJournal.mutateAsync();
      showToast('Sesi mengajar diakhiri.');
      setConfirmingEnd(false);
      onClose();
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : 'Gagal mengakhiri sesi.', 'error');
    }
  }

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={`${journal.subject?.name ?? 'Tanpa mapel'} · ${journal.class.name}`}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Tutup
          </Button>
          {journal.status === 'ongoing' && !confirmingEnd && (
            <Button variant="danger" onClick={() => setConfirmingEnd(true)}>
              Akhiri Sesi
            </Button>
          )}
          <Button onClick={handleSave} loading={updateJournal.isPending}>
            Simpan
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-4">
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="text-[12px] text-muted">
            {formatDate(journal.date)} · {timeRange} · {journal.room?.name ?? 'Tanpa ruangan'}
          </span>
          {journal.status === 'ongoing' ? <Tag variant="now">Berlangsung</Tag> : <Tag variant="done">Selesai</Tag>}
          {journal.flags.map((flag) => (
            <Tag key={flag} variant="neutral">
              {FLAG_LABEL[flag] ?? flag}
            </Tag>
          ))}
        </div>

        {confirmingEnd ? (
          <div className="flex flex-col gap-3 rounded-lg border border-line bg-surface-2 p-3">
            <p className="text-[13px] text-ink">Akhiri sesi mengajar ini sekarang? Anda tetap bisa mengisi materi setelahnya.</p>
            <div className="flex justify-end gap-2">
              <Button type="button" variant="secondary" onClick={() => setConfirmingEnd(false)}>
                Batal
              </Button>
              <Button type="button" variant="danger" loading={endJournal.isPending} onClick={handleEnd}>
                Ya, Akhiri
              </Button>
            </div>
          </div>
        ) : null}

        <Field label="Materi yang diajarkan" htmlFor="journal-material">
          <Textarea
            id="journal-material"
            value={material}
            onChange={(e) => setMaterial(e.target.value)}
            placeholder="Mis. Persamaan kuadrat — menyelesaikan dengan faktorisasi"
            rows={3}
          />
        </Field>

        <Field label="Catatan (opsional)" htmlFor="journal-note">
          <Textarea
            id="journal-note"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder="Mis. 3 siswa belum mengumpulkan tugas"
            rows={2}
          />
        </Field>

        {updateJournal.isError && (
          <p className="text-[12px] text-danger">
            {updateJournal.error instanceof ApiError ? updateJournal.error.message : 'Gagal menyimpan jurnal.'}
          </p>
        )}
      </div>
    </Dialog>
  );
}
