import { useEffect, useRef, useState, type ChangeEvent, type FormEvent } from 'react';
import { Paperclip, X } from 'lucide-react';
import { Dialog } from '../../components/ui/Dialog';
import { Field } from '../../components/ui/Field';
import { Button } from '../../components/ui/Button';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { useUploadInvoiceProof } from './api';
import type { Invoice } from '../../lib/types';

interface UploadProofDialogProps {
  invoice: Invoice | null;
  onClose: () => void;
}

const MAX_PROOF_BYTES = 5 * 1024 * 1024;
const ALLOWED_TYPES = ['application/pdf', 'image/jpeg', 'image/png'];
const ALLOWED_EXTENSIONS = ['.pdf', '.jpg', '.jpeg', '.png'];

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function validateProof(file: File): string | null {
  const ext = file.name.slice(file.name.lastIndexOf('.')).toLowerCase();
  const typeOk = ALLOWED_TYPES.includes(file.type) || ALLOWED_EXTENSIONS.includes(ext);
  if (!typeOk) return 'Bukti transfer harus berupa PDF, JPG, atau PNG.';
  if (file.size > MAX_PROOF_BYTES) return 'Ukuran berkas maksimal 5 MB.';
  return null;
}

/** Dialog "Upload Bukti Transfer" per invoice — docs/09-billing.md "Transfer manual". */
export function UploadProofDialog({ invoice, onClose }: UploadProofDialogProps) {
  const upload = useUploadInvoiceProof();
  const { showToast } = useToast();
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [file, setFile] = useState<File | null>(null);
  const [fileError, setFileError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);

  useEffect(() => {
    if (invoice) {
      setFile(null);
      setFileError(null);
      setFormError(null);
    }
  }, [invoice]);

  function handleFileChange(e: ChangeEvent<HTMLInputElement>) {
    const picked = e.target.files?.[0] ?? null;
    if (!picked) {
      setFile(null);
      setFileError(null);
      return;
    }
    const error = validateProof(picked);
    if (error) {
      setFileError(error);
      setFile(null);
      if (fileInputRef.current) fileInputRef.current.value = '';
      return;
    }
    setFileError(null);
    setFile(picked);
  }

  function removeFile() {
    setFile(null);
    setFileError(null);
    if (fileInputRef.current) fileInputRef.current.value = '';
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setFormError(null);
    if (!invoice) return;
    if (!file) {
      setFormError('Pilih berkas bukti transfer terlebih dulu.');
      return;
    }
    try {
      await upload.mutateAsync({ invoiceId: invoice.id, file });
      showToast('Bukti transfer terkirim. Menunggu verifikasi.');
      onClose();
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : 'Gagal mengunggah bukti transfer.');
    }
  }

  return (
    <Dialog open={invoice !== null} onClose={onClose} title="Upload Bukti Transfer">
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        {invoice && (
          <p className="text-[14px] text-ink">
            Tagihan <span className="num font-medium">{invoice.number}</span> — akan berstatus &ldquo;Menunggu
            Verifikasi&rdquo; sampai diperiksa oleh NouSchool.
          </p>
        )}

        <Field label="Bukti transfer" htmlFor="proof-file" error={fileError ?? undefined} hint={!fileError ? 'PDF, JPG, atau PNG — maksimal 5 MB.' : undefined}>
          {file ? (
            <div className="flex items-center justify-between gap-2 rounded-lg border border-line px-3 py-2.5">
              <span className="flex min-w-0 items-center gap-2 text-[14px] text-ink">
                <Paperclip size={16} strokeWidth={2} className="shrink-0 text-muted" aria-hidden="true" />
                <span className="truncate">{file.name}</span>
                <span className="num shrink-0 text-[12px] text-muted">{formatFileSize(file.size)}</span>
              </span>
              <button
                type="button"
                onClick={removeFile}
                aria-label="Hapus berkas"
                className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-ink"
              >
                <X size={16} strokeWidth={2} aria-hidden="true" />
              </button>
            </div>
          ) : (
            <input
              ref={fileInputRef}
              id="proof-file"
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
          <Button type="submit" loading={upload.isPending}>
            Kirim Bukti Transfer
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
