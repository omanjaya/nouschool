import { Check, Clock, X } from 'lucide-react';
import { formatDateTime } from '../../lib/date';
import { stepApproverLabel } from './format';
import type { LeaveApprovalStep } from '../../lib/types';

function StepIcon({ decision }: { decision: LeaveApprovalStep['decision'] }) {
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

/** Timeline vertikal langkah persetujuan izin — ikon lingkaran + garis hairline penghubung. */
export function LeaveApprovalTimeline({ steps }: { steps: LeaveApprovalStep[] }) {
  const ordered = [...steps].sort((a, b) => a.step_order - b.step_order);

  return (
    <div className="flex flex-col">
      {ordered.map((step, idx) => (
        <div key={step.id} className="flex gap-3">
          <div className="flex flex-col items-center">
            <StepIcon decision={step.decision} />
            {idx < ordered.length - 1 && <span className="w-px flex-1 min-h-6 bg-line" aria-hidden="true" />}
          </div>
          <div className={`flex-1 ${idx < ordered.length - 1 ? 'pb-4' : ''}`}>
            <p className="text-[14px] font-medium text-ink">{stepApproverLabel(step)}</p>
            {step.decision ? (
              <>
                <p className="text-[12px] text-muted">
                  {step.decision === 'approved' ? 'Disetujui' : 'Ditolak'}
                  {step.decided_at ? ` · ${formatDateTime(step.decided_at)}` : ''}
                </p>
                {step.comment && <p className="mt-1 text-[13px] text-ink">{step.comment}</p>}
              </>
            ) : (
              <p className="text-[12px] text-muted">Menunggu keputusan</p>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}
