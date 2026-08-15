/**
 * Fetch wrapper tipis untuk memanggil backend NouSchool di `/api`.
 * - Selalu mengirim cookie sesi (`credentials: 'include'`).
 * - Error dari backend berbentuk JSON `{ error: { code, message } }`
 *   (lihat CLAUDE.md — konvensi error terpusat di `platform/httpx`).
 */

export class ApiError extends Error {
  code: string;
  status: number;

  constructor(code: string, message: string, status: number) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
    this.status = status;
  }
}

interface ApiErrorBody {
  error?: {
    code?: string;
    message?: string;
  };
}

interface ApiSuccessBody<T> {
  data: T;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  // FormData (upload multipart) mengatur Content-Type + boundary sendiri —
  // jangan pernah menimpanya dengan 'application/json'.
  const isFormData = init?.body instanceof FormData;

  const res = await fetch(`/api${path}`, {
    credentials: 'include',
    headers: {
      ...(isFormData ? {} : { 'Content-Type': 'application/json' }),
      ...(init?.headers ?? {}),
    },
    ...init,
  });

  if (!res.ok) {
    let code = 'unknown_error';
    let message = 'Terjadi kesalahan. Coba lagi.';

    try {
      const body = (await res.json()) as ApiErrorBody;
      if (body.error?.code) code = body.error.code;
      if (body.error?.message) message = body.error.message;
    } catch {
      // Body bukan JSON valid — pakai pesan default di atas.
    }

    throw new ApiError(code, message, res.status);
  }

  if (res.status === 204) {
    return undefined as T;
  }

  // Semua response sukses dibungkus `{ data: ... }` — unwrap di sini supaya
  // pemanggil cukup bekerja dengan tipe datanya langsung.
  const body = (await res.json()) as ApiSuccessBody<T>;
  return body.data;
}

function withBody(method: string) {
  return <T>(path: string, body?: unknown) =>
    request<T>(path, {
      method,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
}

export const api = {
  get: <T>(path: string) => request<T>(path, { method: 'GET' }),
  post: withBody('POST'),
  put: withBody('PUT'),
  patch: withBody('PATCH'),
  delete: <T>(path: string) => request<T>(path, { method: 'DELETE' }),
  /** Upload multipart (field `file`) — dipakai import Excel/CSV. */
  upload: <T>(path: string, formData: FormData) => request<T>(path, { method: 'POST', body: formData }),
};
