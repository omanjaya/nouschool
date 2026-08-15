import { useEffect, useState } from 'react';
import { Dialog } from '../../components/ui/Dialog';
import { Button } from '../../components/ui/Button';
import { Field, Select } from '../../components/ui/Field';
import { ApiError } from '../../lib/api';
import { useClasses } from '../classes/api';
import { useCopySchedule } from './api';
import type { SchoolClass } from '../../lib/types';

interface CopyScheduleDialogProps {
  open: boolean;
  onClose: () => void;
  targetClassId: string;
  targetClassName: string;
}

/** Dialog "Salin dari kelas...": pilih kelas sumber → copy → tampilkan hasil copied/skipped. */
export function CopyScheduleDialog({ open, onClose, targetClassId, targetClassName }: CopyScheduleDialogProps) {
  const { data: classes } = useClasses();
  const [fromClassId, setFromClassId] = useState('');
  const copySchedule = useCopySchedule();

  useEffect(() => {
    if (open) {
      setFromClassId('');
      copySchedule.reset();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const sourceOptions = (classes ?? []).filter((c: SchoolClass) => c.id !== targetClassId);

  function handleCopy() {
    if (!fromClassId) return;
    copySchedule.mutate({ from_class_id: fromClassId, to_class_id: targetClassId });
  }

  return (
    <Dialog open={open} onClose={onClose} title="Salin Jadwal dari Kelas Lain">
      <div className="flex flex-col gap-4">
        <p className="text-[12px] text-muted">Menyalin ke: {targetClassName}</p>

        <Field label="Kelas sumber" htmlFor="copy-from-class">
          <Select id="copy-from-class" value={fromClassId} onChange={(e) => setFromClassId(e.target.value)}>
            <option value="">Pilih kelas sumber</option>
            {sourceOptions.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </Select>
        </Field>

        {copySchedule.isError && (
          <p className="text-[12px] text-danger">
            {copySchedule.error instanceof ApiError ? copySchedule.error.message : 'Gagal menyalin jadwal.'}
          </p>
        )}

        {copySchedule.data && (
          <div className="rounded-lg border border-line bg-surface-2 p-3 text-[13px] text-ink">
            <p className="font-semibold">{copySchedule.data.copied} slot berhasil disalin.</p>
            {copySchedule.data.skipped.length > 0 && (
              <div className="mt-2">
                <p className="text-muted">{copySchedule.data.skipped.length} slot dilewati:</p>
                <ul className="mt-1 list-disc pl-4 text-muted">
                  {copySchedule.data.skipped.map((s, i) => (
                    <li key={i}>{s.reason}</li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        )}

        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose}>
            {copySchedule.isSuccess ? 'Tutup' : 'Batal'}
          </Button>
          {!copySchedule.isSuccess && (
            <Button type="button" onClick={handleCopy} loading={copySchedule.isPending} disabled={!fromClassId}>
              Salin Jadwal
            </Button>
          )}
        </div>
      </div>
    </Dialog>
  );
}
