import { Check, Clock, X } from 'lucide-react';
import { formatDateTime } from '../../lib/date';
import { bkStepDecision, homeroomStepDecision, type StepDecision } from './format';
import type { StudentLeaveRequest, StudentLeaveStepInfo } from '../../lib/types';

function StepIcon({ decision }: { decision: StepDecision }) {
  if (decision === 'approved') {
    return (
      <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full border border-st-hadir-line text-st-hadir">
        <Check size={16} strokeWidth={2} aria-hidden="true" />
      </span>
    );
  }
  if (decision === 'rejected') {
    return (
      <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full border border-st-alpa-line text-st-alpa">
        <X size={16} strokeWidth={2} aria-hidden="true" />
      </span>
    );
  }
  return (
    <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full border border-line text-muted">
      <Clock size={16} strokeWidth={2} aria-hidden="true" />
    </span>
  );
}

function Step({
  label,
  decision,
  info,
  isLast,
}: {
  label: string;
  decision: StepDecision;
  info: StudentLeaveStepInfo | null;
  isLast: boolean;
}) {
  return (
    <div className="flex gap-3">
      <div className="flex flex-col items-center">
        <StepIcon decision={decision} />
        {!isLast && <span className="w-px flex-1 min-h-6 bg-line" aria-hidden="true" />}
      </div>
      <div className={isLast ? 'flex-1' : 'flex-1 pb-4'}>
        <p className="text-[14px] font-medium text-ink">{label}</p>
        {info ? (
          <>
            <p className="text-[12px] text-muted">
              {info.decided_by_name}
              {info.decided_at ? ` · ${formatDateTime(info.decided_at)}` : ''}
            </p>
            {info.comment && <p className="mt-1 text-[13px] text-ink">{info.comment}</p>}
          </>
        ) : (
          <p className="text-[12px] text-muted">Menunggu keputusan</p>
        )}
      </div>
    </div>
  );
}

/** Timeline dua tahap tetap izin siswa: Wali Kelas → BK (berbeda dari `leave` guru yang punya rantai step dinamis). */
export function StudentLeaveApprovalTimeline({ request }: { request: StudentLeaveRequest }) {
  return (
    <div className="flex flex-col">
      <Step label="Wali Kelas" decision={homeroomStepDecision(request)} info={request.homeroom} isLast={false} />
      <Step label="Guru BK" decision={bkStepDecision(request)} info={request.bk} isLast />
    </div>
  );
}
