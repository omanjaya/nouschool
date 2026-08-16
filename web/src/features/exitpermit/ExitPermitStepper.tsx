import { Check, Clock, X } from 'lucide-react';
import { Tag } from '../../components/ui/Tag';
import { formatDateTime } from '../../lib/date';
import {
  EXIT_PERMIT_STAGE_LABEL,
  EXIT_PERMIT_STAGE_ORDER,
  exitPermitStageDecision,
  type ExitPermitStageField,
  type StepDecision,
} from './format';
import type { ExitPermit, ExitPermitStatus } from '../../lib/types';

/** Status di luar rantai persetujuan — tidak ada lagi "tahap aktif" untuk disorot. */
const TERMINAL_STATUSES: ExitPermitStatus[] = ['issued', 'exited', 'rejected', 'canceled'];

function currentStageField(permit: ExitPermit): ExitPermitStageField | null {
  if (TERMINAL_STATUSES.includes(permit.status)) return null;
  return EXIT_PERMIT_STAGE_ORDER.find((f) => !permit[f]) ?? null;
}

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

function Step({ field, permit, isLast }: { field: ExitPermitStageField; permit: ExitPermit; isLast: boolean }) {
  const decision = exitPermitStageDecision(permit, field);
  const isCurrent = decision === null && currentStageField(permit) === field;
  const info = permit[field];

  return (
    <div className="flex gap-3">
      <div className="flex flex-col items-center">
        <StepIcon decision={decision} />
        {!isLast && <span className="w-px flex-1 min-h-6 bg-line" aria-hidden="true" />}
      </div>
      <div className={isLast ? 'flex-1' : 'flex-1 pb-4'}>
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-[14px] font-medium text-ink">{EXIT_PERMIT_STAGE_LABEL[field]}</p>
          {isCurrent && <Tag variant="now">Sedang diproses</Tag>}
        </div>
        {decision === 'approved' && info ? (
          <p className="text-[12px] text-muted">
            {info.by_name} · {formatDateTime(info.at)}
          </p>
        ) : decision === 'rejected' ? (
          <p className="text-[12px] text-danger">Ditolak</p>
        ) : !isCurrent ? (
          <p className="text-[12px] text-muted">Menunggu tahap sebelumnya</p>
        ) : null}
      </div>
    </div>
  );
}

/** Stepper 4 tahap rantai dispensasi keluar: Piket → Guru Pengajar → BK → Pimpinan (docs/12-sion-parity.md Gelombang B alur 2). */
export function ExitPermitStepper({ permit }: { permit: ExitPermit }) {
  return (
    <div className="flex flex-col">
      {EXIT_PERMIT_STAGE_ORDER.map((field, i) => (
        <Step key={field} field={field} permit={permit} isLast={i === EXIT_PERMIT_STAGE_ORDER.length - 1} />
      ))}
    </div>
  );
}
