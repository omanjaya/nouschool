import { Paperclip } from 'lucide-react';
import { Tag } from '../../components/ui/Tag';
import { formatDate, formatDateRange } from '../../lib/date';
import { LEAVE_STATUS_LABEL, LEAVE_STATUS_TAG_VARIANT } from './format';
import { LeaveApprovalTimeline } from './LeaveApprovalTimeline';
import type { LeaveRequest } from '../../lib/types';

/** Info lengkap + timeline persetujuan — dipakai di detail pengajuan (guru) & detail persetujuan (approver). */
export function LeaveRequestDetailBody({ request, showTeacher = false }: { request: LeaveRequest; showTeacher?: boolean }) {
  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-3 border-b border-line pb-5">
        <div className="flex items-start justify-between gap-3">
          <div>
            {showTeacher && <p className="text-[14px] font-semibold text-ink">{request.teacher.name}</p>}
            <p className="text-[16px] font-semibold text-ink">{request.type_label}</p>
            <p className="text-[13px] text-muted">
              {formatDateRange(request.date_start, request.date_end)} · {request.days} hari
            </p>
          </div>
          <Tag variant={LEAVE_STATUS_TAG_VARIANT[request.status]}>{LEAVE_STATUS_LABEL[request.status]}</Tag>
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
        <LeaveApprovalTimeline steps={request.steps} />
      </div>
    </div>
  );
}
