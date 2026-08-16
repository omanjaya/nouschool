# React + TypeScript + Vite

This template provides a minimal setup to get React working in Vite with HMR and some Oxlint rules.

Currently, two official plugins are available:

- [@vitejs/plugin-react](https://github.com/vitejs/vite-plugin-react/blob/main/packages/plugin-react) uses [Oxc](https://oxc.rs)
- [@vitejs/plugin-react-swc](https://github.com/vitejs/vite-plugin-react/blob/main/packages/plugin-react-swc) uses [SWC](https://swc.rs/)

## React Compiler

The React Compiler is not enabled on this template because of its impact on dev & build performances. To add it, see [this documentation](https://react.dev/learn/react-compiler/installation).

## Expanding the Oxlint configuration

If you are developing a production application, we recommend enabling type-aware lint rules by installing `oxlint-tsgolint` and editing `.oxlintrc.json`:

```json
{
  "$schema": "./node_modules/oxlint/configuration_schema.json",
  "plugins": ["react", "typescript", "oxc"],
  "options": {
    "typeAware": true
  },
  "rules": {
    "react/rules-of-hooks": "error",
    "react/only-export-components": ["warn", { "allowConstantExport": true }]
  }
}
```

See the [Oxlint rules documentation](https://oxc.rs/docs/guide/usage/linter/rules) for the full list of rules and categories.

## Frontend blocks (shadcnblocks)

`components.json` mendaftarkan registry pihak ketiga `@shadcnblocks`
selain registry shadcn/ui bawaan, supaya block premium bisa diambil sebagai
BAHAN lalu diadaptasi ke design system "Rapor" (lihat
`src/components/blocks/README.md` untuk aturan wajib adaptasinya — jangan
skip, itu bukan opsional).

Ambil block:

```sh
npx shadcn@latest add @shadcnblocks/<nama-block>
```

Ambil primitif shadcn/ui biasa (mis. untuk melengkapi block di atas):

```sh
npx shadcn@latest add button
```

### Set API key registry

Registry `@shadcnblocks` butuh `Authorization: Bearer ${SHADCNBLOCKS_API_KEY}`
(lihat `components.json`). **Jangan pernah commit nilai key ke repo.**
CLI shadcn (lewat `@dotenvx/dotenvx`) membaca env var dari, berurutan sesuai
prioritas — file lebih awal menang, dan variabel yang sudah ada di
`process.env` selalu menang atas isi file: `.env.local` →
`.env.development.local` → `.env.development` → `.env` di `web/` (cwd saat
menjalankan CLI), digabung ke `process.env` proses CLI.

Dua cara pakai, pilih salah satu:

1. **Env var proses** (PowerShell), tidak perlu file apa pun:
   ```powershell
   $env:SHADCNBLOCKS_API_KEY = "isi-key-di-sini"
   npx shadcn@latest add @shadcnblocks/<nama-block>
   ```
2. **File `web/.env.local`** (sudah ada dengan placeholder kosong, dan
   ter-gitignore lewat pola `*.local` — verifikasi dengan
   `git check-ignore -v web/.env.local`): isi barisnya jadi
   `SHADCNBLOCKS_API_KEY=isi-key-di-sini`, lalu jalankan CLI seperti biasa.

Tanpa key, `add @shadcnblocks/...` akan gagal dengan 401 dari registry —
itu wajar; registry shadcn/ui resmi (`button`, `card`, dst) tidak butuh key
sama sekali.
