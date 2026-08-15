import { useState, type FormEvent } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { Eye, EyeOff } from 'lucide-react';
import { Field, Input } from '../../components/ui/Field';
import { Button } from '../../components/ui/Button';
import { ApiError } from '../../lib/api';
import { useLogin, useMe } from './api';

/** Layar penuh tanpa AppShell, maks 400px, tengah. */
export function LoginPage() {
  const { data: me, isLoading: meLoading } = useMe();
  const [identifier, setIdentifier] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const navigate = useNavigate();
  const login = useLogin();

  // Sudah login — jangan tampilkan form login lagi.
  if (!meLoading && me) {
    const isPlatformAdmin = me.is_super_admin && !me.school;
    return <Navigate to={isPlatformAdmin ? '/admin' : '/'} replace />;
  }

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    login.mutate(
      { identifier, password },
      {
        onSuccess: (loggedInMe) => {
          const isPlatformAdmin = loggedInMe.is_super_admin && !loggedInMe.school;
          navigate(isPlatformAdmin ? '/admin' : '/', { replace: true });
        },
      },
    );
  }

  const errorMessage = login.isError
    ? login.error instanceof ApiError
      ? login.error.message
      : 'Terjadi kesalahan. Coba lagi.'
    : null;

  return (
    <div className="flex min-h-dvh items-center justify-center bg-bg px-5">
      <div className="w-full max-w-[400px]">
        <p className="mb-8 text-center text-[21px] font-semibold text-ink">NouSchool</p>

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <Field label="Email atau username" htmlFor="identifier">
            <Input
              id="identifier"
              name="identifier"
              autoComplete="username"
              value={identifier}
              onChange={(e) => setIdentifier(e.target.value)}
              required
            />
          </Field>

          <Field label="Kata sandi" htmlFor="password">
            <div className="relative">
              <Input
                id="password"
                name="password"
                type={showPassword ? 'text' : 'password'}
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                className="pr-11"
              />
              <button
                type="button"
                onClick={() => setShowPassword((v) => !v)}
                aria-label={showPassword ? 'Sembunyikan kata sandi' : 'Lihat kata sandi'}
                className="absolute inset-y-0 right-0 flex w-11 items-center justify-center text-muted hover:text-ink"
              >
                {showPassword ? (
                  <EyeOff size={20} strokeWidth={2} aria-hidden="true" />
                ) : (
                  <Eye size={20} strokeWidth={2} aria-hidden="true" />
                )}
              </button>
            </div>
          </Field>

          {errorMessage && <p className="text-[12px] text-danger">{errorMessage}</p>}

          <Button type="submit" variant="primary" loading={login.isPending} className="w-full">
            Masuk
          </Button>
        </form>
      </div>
    </div>
  );
}
