import { useState, type FormEvent } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { KeyRound } from 'lucide-react';
import { Field, Input } from '../../components/ui/Field';
import { Button } from '../../components/ui/Button';
import { ApiError } from '../../lib/api';
import { BrandMark } from '../branding/BrandMark';
import { useMe } from '../auth/api';
import { useActivate, useInvitationInfo } from './api';
import type { InvitationInfo } from '../../lib/types';

const ROLE_LABEL: Record<InvitationInfo['role'], string> = {
  siswa: 'siswa',
  orang_tua: 'orang tua',
  guru: 'guru',
};

/** Halaman publik `/aktivasi` — tanpa AppShell, layout senada LoginPage. */
export function ActivationPage() {
  const { data: me, isLoading: meLoading } = useMe();
  const [codeInput, setCodeInput] = useState('');
  const [submittedCode, setSubmittedCode] = useState('');
  const navigate = useNavigate();

  const invitation = useInvitationInfo(submittedCode);

  // Sudah login — tidak perlu aktivasi lagi.
  if (!meLoading && me) {
    return <Navigate to="/" replace />;
  }

  function handleCheckCode(e: FormEvent) {
    e.preventDefault();
    setSubmittedCode(codeInput.trim());
  }

  const codeErrorMessage =
    invitation.isError && submittedCode
      ? invitation.error instanceof ApiError
        ? invitation.error.status === 404
          ? 'Kode aktivasi tidak ditemukan. Periksa kembali kode yang diberikan sekolah.'
          : invitation.error.status === 410 || invitation.error.code === 'invitation_used'
            ? 'Kode aktivasi sudah dipakai. Minta kode baru ke admin sekolah.'
            : invitation.error.message
        : 'Gagal memeriksa kode aktivasi. Coba lagi.'
      : null;

  return (
    <div className="flex min-h-dvh items-center justify-center bg-bg px-5 py-10">
      <div className="w-full max-w-[400px]">
        <BrandMark className="mb-8" />

        {!invitation.data ? (
          <form onSubmit={handleCheckCode} className="flex flex-col gap-4">
            <div className="mb-2 flex flex-col items-center gap-2 text-center">
              <KeyRound size={24} strokeWidth={2} className="text-muted" aria-hidden="true" />
              <p className="text-[14px] text-muted">Masukkan kode aktivasi yang diberikan sekolah Anda.</p>
            </div>
            <Field label="Kode aktivasi" htmlFor="activation-code" error={codeErrorMessage ?? undefined}>
              <Input
                id="activation-code"
                value={codeInput}
                onChange={(e) => setCodeInput(e.target.value)}
                autoCapitalize="characters"
                required
              />
            </Field>
            <Button type="submit" variant="primary" loading={invitation.isFetching} className="w-full">
              Lanjutkan
            </Button>
          </form>
        ) : (
          <ActivationForm code={submittedCode} info={invitation.data} onDone={() => navigate('/', { replace: true })} />
        )}
      </div>
    </div>
  );
}

function ActivationForm({
  code,
  info,
  onDone,
}: {
  code: string;
  info: InvitationInfo;
  onDone: () => void;
}) {
  const [name, setName] = useState('');
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [formError, setFormError] = useState<string | null>(null);
  const activate = useActivate();

  const needsUsername = info.role === 'siswa';
  const needsEmailOrUsername = info.role === 'orang_tua' || info.role === 'guru';

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setFormError(null);

    if (!name.trim()) {
      setFormError('Nama wajib diisi.');
      return;
    }
    if (needsUsername && !username.trim()) {
      setFormError('Username wajib diisi.');
      return;
    }
    if (needsEmailOrUsername && !email.trim() && !username.trim()) {
      setFormError('Isi email atau username untuk masuk nanti.');
      return;
    }
    if (password.length < 8) {
      setFormError('Kata sandi minimal 8 karakter.');
      return;
    }
    if (password !== confirmPassword) {
      setFormError('Konfirmasi kata sandi tidak sama.');
      return;
    }

    activate.mutate(
      {
        code,
        name: name.trim(),
        username: username.trim() || undefined,
        email: email.trim() || undefined,
        password,
      },
      { onSuccess: onDone },
    );
  }

  const errorMessage =
    formError ?? (activate.isError ? (activate.error instanceof ApiError ? activate.error.message : 'Gagal mengaktifkan akun.') : null);

  return (
    <div className="flex flex-col gap-4">
      <p className="text-center text-[14px] text-ink">
        Aktivasi akun {ROLE_LABEL[info.role]} untuk <span className="font-semibold">{info.student_name}</span> —{' '}
        {info.school_name}
      </p>

      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Field label="Nama" htmlFor="activation-name">
          <Input id="activation-name" value={name} onChange={(e) => setName(e.target.value)} required />
        </Field>

        {needsUsername && (
          <Field label="Username" htmlFor="activation-username">
            <Input id="activation-username" value={username} onChange={(e) => setUsername(e.target.value)} required />
          </Field>
        )}

        {needsEmailOrUsername && (
          <>
            <Field label="Email" htmlFor="activation-email" hint="Isi salah satu: email atau username">
              <Input
                id="activation-email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </Field>
            <Field label="Username" htmlFor="activation-username-alt">
              <Input id="activation-username-alt" value={username} onChange={(e) => setUsername(e.target.value)} />
            </Field>
          </>
        )}

        <Field label="Kata sandi" htmlFor="activation-password" hint="Minimal 8 karakter">
          <Input
            id="activation-password"
            type="password"
            autoComplete="new-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        </Field>
        <Field label="Konfirmasi kata sandi" htmlFor="activation-password-confirm">
          <Input
            id="activation-password-confirm"
            type="password"
            autoComplete="new-password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            required
          />
        </Field>

        {errorMessage && <p className="text-[12px] text-danger">{errorMessage}</p>}

        <Button type="submit" variant="primary" loading={activate.isPending} className="w-full">
          Aktifkan Akun
        </Button>
      </form>
    </div>
  );
}
