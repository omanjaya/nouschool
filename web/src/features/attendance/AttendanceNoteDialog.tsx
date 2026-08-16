import { useEffect, useState } from 'react';
import { ShieldAlert } from 'lucide-react';
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
  /**
   * Aksi sekunder "Catat Pelanggaran" (Fase 14 Gelombang A, docs/12
   * §Gelombang A poin 3) — dibuka dari sini supaya guru tidak perlu
   * berpindah ke `/kedisiplinan` saat sedang mengisi absensi. Opsional
   * (kalau tidak diisi, tombolnya tidak tampil) & disembunyikan saat `readOnly`.
   */
  onRecordViolation?: () => void;
}

/** Dialog catatan per siswa (mis. alasan izin/sakit) — dibuka dari icon MessageSquare di baris siswa. */
export function AttendanceNoteDialog({
  open,
  studentName,
  initialNote,
  readOnly,
  onClose,
  onSave,
  onRecordViolation,
}: AttendanceNoteDialogProps) {
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
            {onRecordViolation && (
              <Button variant="ghost" onClick={onRecordViolation} className="mr-auto">
                <ShieldAlert size={16} strokeWidth={2} aria-hidden="true" />
                Catat Pelanggaran
              </Button>
            )}
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
