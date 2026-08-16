import { useEffect, useRef, useState, type ChangeEvent, type FormEvent } from 'react';
import { Paperclip, X } from 'lucide-react';
import { Dialog } from '../../components/ui/Dialog';
import { Field, Input, Select, Textarea } from '../../components/ui/Field';
import { Button } from '../../components/ui/Button';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { todayISODate } from '../../lib/date';
import { useCreateStudentLeaveRequest } from './api';
import type { StudentLeaveType } from '../../lib/types';

interface StudentLeaveRequestFormDialogProps {
  open: boolean;
  onClose: () => void;
  onSubmitted: () => void;
  /** Terisi saat orang tua mengajukan untuk anak — dikirim sebagai `student_id`; kosong berarti siswa mengajukan untuk dirinya sendiri. */
  studentId?: string;
}

const MAX_ATTACHMENT_BYTES = 5 * 1024 * 1024;
const ALLOWED_ATTACHMENT_TYPES = ['application/pdf', 'image/jpeg', 'image/png'];
const ALLOWED_ATTACHMENT_EXTENSIONS = ['.pdf', '.jpg', '.jpeg', '.png'];

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function validateAttachment(file: File): string | null {
  const ext = file.name.slice(file.name.lastIndexOf('.')).toLowerCase();
  const typeOk = ALLOWED_ATTACHMENT_TYPES.includes(file.type) || ALLOWED_ATTACHMENT_EXTENSIONS.includes(ext);
  if (!typeOk) return 'Lampiran harus berupa PDF, JPG, atau PNG.';
  if (file.size > MAX_ATTACHMENT_BYTES) return 'Ukuran lampiran maksimal 5 MB.';
  return null;
}

/** Dialog "Ajukan Izin" — jenis (Sakit/Izin), rentang tanggal, alasan, lampiran opsional (pola `LeaveRequestFormDialog`). */
export function StudentLeaveRequestFormDialog({
  open,
  onClose,
  onSubmitted,
  studentId,
}: StudentLeaveRequestFormDialogProps) {
  const createRequest = useCreateStudentLeaveRequest();
  const { showToast } = useToast();
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [type, setType] = useState<StudentLeaveType>('sakit');
  const [dateStart, setDateStart] = useState(todayISODate());
  const [dateEnd, setDateEnd] = useState(todayISODate());
  const [reason, setReason] = useState('');
  const [attachment, setAttachment] = useState<File | null>(null);
  const [attachmentError, setAttachmentError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setType('sakit');
      setDateStart(todayISODate());
      setDateEnd(todayISODate());
      setReason('');
      setAttachment(null);
      setAttachmentError(null);
      setFormError(null);
    }
  }, [open]);

  function handleFileChange(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0] ?? null;
    if (!file) {
      setAttachment(null);
      setAttachmentError(null);
      return;
    }
    const error = validateAttachment(file);
    if (error) {
      setAttachmentError(error);
      setAttachment(null);
      if (fileInputRef.current) fileInputRef.current.value = '';
      return;
    }
    setAttachmentError(null);
    setAttachment(file);
  }

  function removeAttachment() {
    setAttachment(null);
    setAttachmentError(null);
    if (fileInputRef.current) fileInputRef.current.value = '';
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setFormError(null);

    if (dateStart > dateEnd) {
      setFormError('Tanggal mulai tidak boleh setelah tanggal selesai.');
      return;
    }
    if (!reason.trim()) {
      setFormError('Alasan izin wajib diisi.');
      return;
    }

    try {
      await createRequest.mutateAsync({
        type,
        date_start: dateStart,
        date_end: dateEnd,
        reason: reason.trim(),
        attachment,
        student_id: studentId,
      });
      showToast('Pengajuan izin terkirim.');
      onSubmitted();
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : 'Gagal mengirim pengajuan izin.');
    }
  }

  return (
    <Dialog open={open} onClose={onClose} title="Ajukan Izin">
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Field label="Jenis izin" htmlFor="student-leave-type">
          <Select
            id="student-leave-type"
            value={type}
            onChange={(e) => setType(e.target.value as StudentLeaveType)}
            required
          >
            <option value="sakit">Sakit</option>
            <option value="izin">Izin</option>
          </Select>
        </Field>

        <div className="grid grid-cols-2 gap-3">
          <Field label="Tanggal mulai" htmlFor="student-leave-date-start">
            <Input
              id="student-leave-date-start"
              type="date"
              value={dateStart}
              onChange={(e) => setDateStart(e.target.value)}
              required
            />
          </Field>
          <Field label="Tanggal selesai" htmlFor="student-leave-date-end">
            <Input
              id="student-leave-date-end"
              type="date"
              value={dateEnd}
              onChange={(e) => setDateEnd(e.target.value)}
              required
            />
          </Field>
        </div>

        <Field label="Alasan" htmlFor="student-leave-reason">
          <Textarea
            id="student-leave-reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder="Jelaskan alasan izin…"
            rows={3}
            required
          />
        </Field>

        <Field
          label="Lampiran (opsional)"
          htmlFor="student-leave-attachment"
          error={attachmentError ?? undefined}
          hint={!attachmentError ? 'PDF, JPG, atau PNG — maksimal 5 MB.' : undefined}
        >
          {attachment ? (
            <div className="flex items-center justify-between gap-2 rounded-lg border border-line px-3 py-2.5">
              <span className="flex min-w-0 items-center gap-2 text-[14px] text-ink">
                <Paperclip size={16} strokeWidth={2} className="shrink-0 text-muted" aria-hidden="true" />
                <span className="truncate">{attachment.name}</span>
                <span className="num shrink-0 text-[12px] text-muted">{formatFileSize(attachment.size)}</span>
              </span>
              <button
                type="button"
                onClick={removeAttachment}
                aria-label="Hapus lampiran"
                className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-ink"
              >
                <X size={16} strokeWidth={2} aria-hidden="true" />
              </button>
            </div>
          ) : (
            <input
              ref={fileInputRef}
              id="student-leave-attachment"
              type="file"
              accept=".pdf,.jpg,.jpeg,.png,application/pdf,image/jpeg,image/png"
              onChange={handleFileChange}
              className="block w-full text-[13px] text-ink file:mr-3 file:min-h-9 file:rounded-lg file:border file:border-line file:bg-surface file:px-3 file:text-[13px] file:font-medium file:text-ink hover:file:bg-surface-2"
            />
          )}
        </Field>

        {formError && <p className="text-[12px] text-danger">{formError}</p>}

        <div className="mt-1 flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" loading={createRequest.isPending}>
            Ajukan Izin
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
