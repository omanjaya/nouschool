import { useEffect, useState } from 'react';
import { Dialog } from '../../components/ui/Dialog';
import { Textarea } from '../../components/ui/Field';
import { Button } from '../../components/ui/Button';

interface AttendanceNoteDialogProps {
  open: boolean;
  studentName: string;
  initialNote: string;
  readOnly?: boolean;
  onClose: () => void;
  onSave: (note: string) => void;
}

/** Dialog catatan per siswa (mis. alasan izin/sakit) — dibuka dari icon MessageSquare di baris siswa. */
export function AttendanceNoteDialog({ open, studentName, initialNote, readOnly, onClose, onSave }: AttendanceNoteDialogProps) {
  const [note, setNote] = useState(initialNote);

  useEffect(() => {
    if (open) setNote(initialNote);
  }, [open, initialNote]);

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={`Catatan — ${studentName}`}
      footer={
        readOnly ? (
          <Button variant="secondary" onClick={onClose}>
            Tutup
          </Button>
        ) : (
          <>
            <Button variant="secondary" onClick={onClose}>
              Batal
            </Button>
            <Button onClick={() => onSave(note.trim())}>Simpan Catatan</Button>
          </>
        )
      }
    >
      <Textarea
        value={note}
        onChange={(e) => setNote(e.target.value)}
        placeholder="Mis. izin acara keluarga, terlambat karena hujan…"
        rows={4}
        disabled={readOnly}
        autoFocus
      />
    </Dialog>
  );
}
