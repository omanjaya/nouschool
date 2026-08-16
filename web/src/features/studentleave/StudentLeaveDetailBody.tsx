import { Copy, Paperclip } from 'lucide-react';
import { Tag } from '../../components/ui/Tag';
import { Button } from '../../components/ui/Button';
import { useToast } from '../../components/ui/Toast';
import { formatDate, formatDateRange } from '../../lib/date';
import { STUDENT_LEAVE_STATUS_LABEL, STUDENT_LEAVE_STATUS_TAG_VARIANT, STUDENT_LEAVE_TYPE_LABEL } from './format';
import { studentLeaveLetterPdfUrl, studentLeaveVerifyPageUrl } from './api';
import { StudentLeaveApprovalTimeline } from './StudentLeaveApprovalTimeline';
import type { StudentLeaveRequest } from '../../lib/types';

/** Info lengkap + timeline persetujuan — dipakai di detail izin siswa (siswa/ortu) & detail review (guru). */
export function StudentLeaveDetailBody({
  request,
  showStudent = false,
}: {
  request: StudentLeaveRequest;
  showStudent?: boolean;
}) {
  const { showToast } = useToast();

  async function handleCopyVerifyLink() {
    if (!request.verify_token) return;
    try {
      await navigator.clipboard.writeText(studentLeaveVerifyPageUrl(request.verify_token));
      showToast('Tautan verifikasi disalin.');
    } catch {
      showToast('Gagal menyalin tautan. Salin manual.', 'error');
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-3 border-b border-line pb-5">
        <div className="flex items-start justify-between gap-3">
          <div>
            {showStudent && (
              <p className="text-[14px] font-semibold text-ink">
                {request.student.name} · {request.student.class_name}
              </p>
            )}
            <p className="text-[16px] font-semibold text-ink">{STUDENT_LEAVE_TYPE_LABEL[request.type]}</p>
            <p className="text-[13px] text-muted num">{formatDateRange(request.date_start, request.date_end)}</p>
          </div>
          <Tag variant={STUDENT_LEAVE_STATUS_TAG_VARIANT[request.status]}>
            {STUDENT_LEAVE_STATUS_LABEL[request.status]}
          </Tag>
        </div>

        <p className="text-[14px] text-ink">{request.reason}</p>

        {request.attachment_url && (
          <a
            href={request.attachment_url}
            target="_blank"
            rel="noreferrer"
            className="inline-flex w-fit items-center gap-1.5 text-[13px] font-medium text-primary hover:underline"
          >
            <Paperclip size={16} strokeWidth={2} aria-hidden="true" />
            Lihat lampiran
          </a>
        )}

        <p className="text-[12px] text-muted">Diajukan {formatDate(request.created_at)}</p>
      </div>

      <div>
        <p className="mb-3 text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Langkah Persetujuan</p>
        <StudentLeaveApprovalTimeline request={request} />
      </div>

      {request.status === 'issued' && request.letter_number && (
        <div className="flex flex-col gap-3 rounded-lg border border-line bg-surface-2 px-4 py-4">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Nomor Surat</p>
            <p className="num text-[16px] font-semibold text-ink">{request.letter_number}</p>
          </div>
          <div className="flex flex-wrap gap-2">
            <a
              href={studentLeaveLetterPdfUrl(request.id)}
              target="_blank"
              rel="noreferrer"
              className="inline-flex min-h-11 items-center justify-center gap-2 rounded-lg border border-line bg-surface px-4 text-[14px] font-semibold text-ink transition-colors duration-150 hover:bg-surface-2"
            >
              Unduh Surat (PDF)
            </a>
            {request.verify_token && (
              <Button type="button" variant="secondary" onClick={handleCopyVerifyLink}>
                <Copy size={16} strokeWidth={2} aria-hidden="true" />
                Salin Tautan Verifikasi
              </Button>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
