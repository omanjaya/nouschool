import type { InputHTMLAttributes, ReactNode, Ref, SelectHTMLAttributes, TextareaHTMLAttributes } from 'react';

interface FieldProps {
  label: string;
  htmlFor?: string;
  error?: string;
  hint?: string;
  children: ReactNode;
  className?: string;
}

/** Bungkus Input/Select/Textarea: label 12px, pesan error --danger di bawah. */
export function Field({ label, htmlFor, error, hint, children, className = '' }: FieldProps) {
  return (
    <div className={`flex flex-col gap-1.5 ${className}`}>
      <label htmlFor={htmlFor} className="text-[12px] font-medium text-muted">
        {label}
      </label>
      {children}
      {error ? (
        <p className="text-[12px] text-danger">{error}</p>
      ) : hint ? (
        <p className="text-[12px] text-muted">{hint}</p>
      ) : null}
    </div>
  );
}

const FIELD_CLASS =
  'h-11 w-full rounded-lg border border-line bg-surface px-3 text-[14px] text-ink placeholder:text-muted transition-colors duration-150 focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary disabled:cursor-not-allowed disabled:opacity-50';

/**
 * `ref` diteruskan sebagai prop biasa (React 19 — tanpa `forwardRef`), dipakai
 * `features/grading/GradingInputPage` untuk navigasi Enter ke baris berikut.
 */
export function Input({
  className = '',
  ref,
  ...rest
}: InputHTMLAttributes<HTMLInputElement> & { ref?: Ref<HTMLInputElement> }) {
  return <input ref={ref} className={`${FIELD_CLASS} ${className}`} {...rest} />;
}

export function Textarea({ className = '', ...rest }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea className={`${FIELD_CLASS} min-h-24 resize-y py-2 ${className}`} {...rest} />;
}

export function Select({ className = '', children, ...rest }: SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select className={`${FIELD_CLASS} ${className}`} {...rest}>
      {children}
    </select>
  );
}

interface CheckboxProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  label: string;
}

/** Checkbox baris tunggal — label 14px di kanan kotak, aksen `--primary` (docs/10 token). */
export function Checkbox({ label, className = '', id, disabled, ...rest }: CheckboxProps) {
  return (
    <label
      htmlFor={id}
      className={`flex min-h-6 items-center gap-2 text-[14px] text-ink ${disabled ? 'opacity-50' : ''} ${className}`}
    >
      <input
        id={id}
        type="checkbox"
        disabled={disabled}
        className="h-4 w-4 shrink-0 rounded border border-line accent-primary focus:outline-none focus:ring-2 focus:ring-primary disabled:cursor-not-allowed"
        {...rest}
      />
      {label}
    </label>
  );
}
