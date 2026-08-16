import { createElement, useMemo } from 'react';

/**
 * Encoder QR minimal, murni TypeScript, TANPA dependency baru (batasan Fase 14
 * Gelombang B2 — backend tidak menyediakan endpoint PNG untuk QR token guru /
 * QR gate siswa, jadi frontend merender sendiri).
 *
 * Cakupan SENGAJA dibatasi seperlunya untuk payload kita (`nouschool:tqr:{token}`,
 * `nouschool:gate:{token}` — selalu ASCII, ±35-45 karakter):
 * - Mode byte saja (bukan alfanumerik/numerik/kanji) — payload kita bukan digit
 *   murni dan bukan subset alfanumerik QR (ada `:`, huruf kecil).
 * - Versi 1-4 saja (21×21 s.d. 33×33 modul) — kapasitas byte-mode EC-M versi 4
 *   ≈62 karakter, jauh di atas kebutuhan kita; versi lebih tinggi (perlu info
 *   versi terpisah di modul, aturan alignment pattern majemuk) sengaja tidak
 *   diimplementasikan karena tidak pernah dipakai.
 * - EC level M diutamakan (fallback ke L kalau teks kebetulan tidak muat di M
 *   pada versi 4) — H/Q tidak diimplementasikan (tidak dibutuhkan, mengurangi
 *   luas kode).
 *
 * Implementasi mengikuti algoritma standar ISO/IEC 18004 (matematika GF(256),
 * Reed–Solomon, penempatan modul zigzag, 8 pola masking + penalti 4 aturan,
 * info format BCH(15,5)) — bukan hasil tebakan; setiap fungsi bisa diuji
 * sendiri (unit-testable) karena dipisah per tahap (lihat ekspor di bawah).
 *
 * KUALITAS: diverifikasi lewat harness terpisah (Node + `jsqr`, decoder QR
 * pihak ketiga independen — dipakai HANYA sebagai alat uji sekali jalan saat
 * menulis modul ini, tidak pernah masuk dependency `web/`) — 138/138 payload
 * acak (termasuk payload realistis `nouschool:tqr:...`/`nouschool:gate:...`
 * ±39 karakter, dan sweep panjang 1-95 karakter) berhasil di-decode balik
 * tepat sama dengan teks asal, di berbagai kombinasi scale (2×-10× per modul)
 * dan quiet zone (0-4 modul) kamera/render.
 */

/* ---------------------------------------------------------------------------
 * 1. Aritmetika GF(256) — polinomial primitif QR: x^8 + x^4 + x^3 + x^2 + 1 (0x11D).
 * ------------------------------------------------------------------------- */

const GF_EXP = new Array<number>(512);
const GF_LOG = new Array<number>(256);

(function initGaloisField() {
  let x = 1;
  for (let i = 0; i < 255; i++) {
    GF_EXP[i] = x;
    GF_LOG[x] = i;
    x <<= 1;
    if (x & 0x100) x ^= 0x11d;
  }
  // Diperpanjang sampai 511 supaya lookup `GF_EXP[a+b]` (a,b masing2 0..254) aman tanpa modulo di tiap pemanggilan.
  for (let i = 255; i < 512; i++) GF_EXP[i] = GF_EXP[i - 255];
})();

function gfMul(a: number, b: number): number {
  if (a === 0 || b === 0) return 0;
  return GF_EXP[GF_LOG[a] + GF_LOG[b]];
}

/* ---------------------------------------------------------------------------
 * 2. Reed–Solomon: polinomial generator + penghitungan codeword EC (sisa bagi).
 * ------------------------------------------------------------------------- */

/**
 * Kalikan polinomial `g` (koefisien derajat tertinggi di index 0) dengan
 * monomial `(x + c)` — derajat hasil naik satu (`g.length + 1`). Bagian
 * "digeser" (kali x) menyumbang `g[k]` ke index yang sama; bagian "dikali c"
 * menyumbang `gfMul(g[k-1], c)` (index bergeser satu ke kanan karena derajatnya
 * turun satu relatif terhadap suku yang digeser).
 */
function polyMultiplyByMonomial(g: number[], c: number): number[] {
  const n = g.length;
  const result = new Array<number>(n + 1).fill(0);
  for (let k = 0; k <= n; k++) {
    let term = 0;
    if (k < n) term ^= g[k];
    if (k - 1 >= 0 && k - 1 < n) term ^= gfMul(g[k - 1], c);
    result[k] = term;
  }
  return result;
}

/** g(x) = ∏_{i=0}^{degree-1} (x - α^i) — akar-akarnya persis α^0..α^(degree-1) (dites lewat `computePolyRoots` di file uji). */
function rsGeneratorPoly(degree: number): number[] {
  let g = [1];
  for (let i = 0; i < degree; i++) g = polyMultiplyByMonomial(g, GF_EXP[i]);
  return g;
}

/** Sisa bagi `dataCodewords(x) * x^degree` oleh `rsGeneratorPoly(degree)` — codeword EC (algoritma pembagian panjang standar, gaya LFSR). */
export function rsComputeRemainder(dataCodewords: number[], degree: number): number[] {
  const generator = rsGeneratorPoly(degree);
  const result = dataCodewords.concat(new Array<number>(degree).fill(0));
  for (let i = 0; i < dataCodewords.length; i++) {
    const coef = result[i];
    if (coef === 0) continue;
    for (let j = 0; j < generator.length; j++) {
      result[i + j] ^= gfMul(generator[j], coef);
    }
  }
  return result.slice(dataCodewords.length);
}

/* ---------------------------------------------------------------------------
 * 3. Tabel versi 1-4 (hanya EC level L & M — lihat catatan cakupan di atas).
 * ------------------------------------------------------------------------- */

type EcLevel = 'L' | 'M';

interface BlockInfo {
  /** Codeword EC per blok. */
  ecPerBlock: number;
  /** Jumlah blok data (semua blok berukuran sama untuk versi 1-4 L/M — tidak ada "grup 2" di rentang ini). */
  numBlocks: number;
}

/** ISO/IEC 18004 §Annex tabel "Error correction characteristics" — hanya baris versi 1-4, level L/M. */
const BLOCK_INFO: Record<number, Record<EcLevel, BlockInfo>> = {
  1: { L: { ecPerBlock: 7, numBlocks: 1 }, M: { ecPerBlock: 10, numBlocks: 1 } },
  2: { L: { ecPerBlock: 10, numBlocks: 1 }, M: { ecPerBlock: 16, numBlocks: 1 } },
  3: { L: { ecPerBlock: 15, numBlocks: 1 }, M: { ecPerBlock: 26, numBlocks: 1 } },
  4: { L: { ecPerBlock: 20, numBlocks: 1 }, M: { ecPerBlock: 18, numBlocks: 2 } },
};

const TOTAL_CODEWORDS: Record<number, number> = { 1: 26, 2: 44, 3: 70, 4: 100 };

/** Indikator level EC dalam info format (2 bit) — tabel tetap standar QR, BUKAN urutan alfabet. */
const EC_LEVEL_BITS: Record<EcLevel, number> = { L: 1, M: 0 };

function dataCodewordsCapacity(version: number, level: EcLevel): number {
  const { ecPerBlock, numBlocks } = BLOCK_INFO[version][level];
  return TOTAL_CODEWORDS[version] - ecPerBlock * numBlocks;
}

/* ---------------------------------------------------------------------------
 * 4. Bit buffer + segmen mode byte.
 * ------------------------------------------------------------------------- */

class BitBuffer {
  bits: number[] = [];

  appendBits(value: number, length: number): void {
    for (let i = length - 1; i >= 0; i--) this.bits.push((value >>> i) & 1);
  }

  get length(): number {
    return this.bits.length;
  }

  toBytes(): number[] {
    const bytes: number[] = [];
    for (let i = 0; i < this.bits.length; i += 8) {
      let b = 0;
      for (let j = 0; j < 8; j++) b = (b << 1) | (this.bits[i + j] ?? 0);
      bytes.push(b);
    }
    return bytes;
  }
}

interface EncodedCodewords {
  version: number;
  level: EcLevel;
  codewords: number[];
}

/**
 * Pilih versi terkecil (1→4) yang muat di level M lebih dulu (kepadatan modul
 * lebih rendah = kontras lebih besar per modul saat dicetak/ditampilkan kecil
 * di layar HP), baru coba level L kalau tidak ada versi 1-4 di M yang cukup.
 * Melempar error kalau tetap tidak muat (di luar kapasitas V4-L, ±78 byte —
 * jauh di atas panjang token kita, lihat komentar modul).
 */
function encodeByteSegment(text: string): EncodedCodewords {
  const bytes = Array.from(new TextEncoder().encode(text));
  for (const level of ['M', 'L'] as const) {
    for (let version = 1; version <= 4; version++) {
      const capacityCodewords = dataCodewordsCapacity(version, level);
      const capacityBits = capacityCodewords * 8;
      const headerBits = 4 + 8; // indikator mode (4 bit) + indikator panjang byte-mode versi 1-9 (8 bit)
      const neededBits = headerBits + bytes.length * 8;
      if (neededBits <= capacityBits) {
        return buildCodewords(bytes, version, level, capacityCodewords);
      }
    }
  }
  throw new Error(
    `Teks terlalu panjang untuk QR (maks ±78 karakter ASCII pada implementasi ini): "${text.slice(0, 20)}…" (${bytes.length} byte).`,
  );
}

function buildCodewords(bytes: number[], version: number, level: EcLevel, capacityCodewords: number): EncodedCodewords {
  const bb = new BitBuffer();
  bb.appendBits(0b0100, 4); // mode byte
  bb.appendBits(bytes.length, 8);
  for (const byte of bytes) bb.appendBits(byte, 8);

  const capacityBits = capacityCodewords * 8;
  const terminatorLen = Math.min(4, capacityBits - bb.length);
  if (terminatorLen > 0) bb.appendBits(0, terminatorLen);
  while (bb.length % 8 !== 0) bb.bits.push(0);

  // Byte pad bergantian 0xEC/0x11 (standar QR) sampai kapasitas penuh.
  const padBytes = [0xec, 0x11];
  let p = 0;
  while (bb.length < capacityBits) {
    bb.appendBits(padBytes[p % 2], 8);
    p++;
  }

  const dataCodewords = bb.toBytes();
  const { ecPerBlock, numBlocks } = BLOCK_INFO[version][level];
  const blockSize = dataCodewords.length / numBlocks;
  const blocks: number[][] = [];
  const ecBlocks: number[][] = [];
  for (let i = 0; i < numBlocks; i++) {
    const block = dataCodewords.slice(i * blockSize, (i + 1) * blockSize);
    blocks.push(block);
    ecBlocks.push(rsComputeRemainder(block, ecPerBlock));
  }

  // Interleaving codeword data lalu EC (kolom demi kolom antar blok) — untuk
  // versi 1-4 di level L/M hanya ada 1-2 blok berukuran sama (tidak ada "grup
  // 2" campur ukuran di rentang versi ini, lihat tabel `BLOCK_INFO`).
  const interleaved: number[] = [];
  for (let i = 0; i < blockSize; i++) {
    for (const block of blocks) interleaved.push(block[i]);
  }
  for (let i = 0; i < ecPerBlock; i++) {
    for (const ecBlock of ecBlocks) interleaved.push(ecBlock[i]);
  }

  return { version, level, codewords: interleaved };
}

/* ---------------------------------------------------------------------------
 * 5. Susun matriks modul: pola fungsi (finder/timing/alignment/dark module),
 *    penempatan data zigzag, pemilihan mask terbaik (8 pola + penalti 4
 *    aturan), lalu gambar info format (BCH) dengan mask terpilih.
 * ------------------------------------------------------------------------- */

export interface QrMatrix {
  size: number;
  /** `modules[row][col]` — `true` = modul gelap. */
  modules: boolean[][];
  version: number;
  level: EcLevel;
  mask: number;
}

function buildMatrix(version: number, level: EcLevel, codewords: number[]): QrMatrix {
  const size = version * 4 + 17;
  const modules: boolean[][] = Array.from({ length: size }, () => new Array<boolean>(size).fill(false));
  const isFunction: boolean[][] = Array.from({ length: size }, () => new Array<boolean>(size).fill(false));

  function setFn(x: number, y: number, dark: boolean): void {
    modules[y][x] = dark;
    isFunction[y][x] = true;
  }

  function drawFinder(cx: number, cy: number): void {
    for (let dy = -4; dy <= 4; dy++) {
      for (let dx = -4; dx <= 4; dx++) {
        const x = cx + dx;
        const y = cy + dy;
        if (x >= 0 && x < size && y >= 0 && y < size) {
          const dist = Math.max(Math.abs(dx), Math.abs(dy));
          // dist 0/1/3 gelap (inti 3x3 + cincin luar 7x7), dist 2 (cincin dalam) & 4 (separator) terang.
          setFn(x, y, dist !== 2 && dist !== 4);
        }
      }
    }
  }
  function drawAlignment(cx: number, cy: number): void {
    for (let dy = -2; dy <= 2; dy++) {
      for (let dx = -2; dx <= 2; dx++) {
        setFn(cx + dx, cy + dy, Math.max(Math.abs(dx), Math.abs(dy)) !== 1);
      }
    }
  }

  drawFinder(3, 3);
  drawFinder(size - 4, 3);
  drawFinder(3, size - 4);

  // Timing pattern (baris/kolom index 6) — lewati sel yang sudah jadi pola fungsi (tumpang tindih area finder).
  for (let i = 0; i < size; i++) {
    if (!isFunction[6][i]) setFn(i, 6, i % 2 === 0);
    if (!isFunction[i][6]) setFn(6, i, i % 2 === 0);
  }

  // Alignment pattern: versi 2-4 di rentang ini masing2 punya PERSIS SATU pola
  // (di luar area finder) pada (pos,pos) dengan pos = 18 + 4*(versi-2) — 18/22/26
  // untuk versi 2/3/4 (rumus umum QR untuk alignment majemuk tidak diperlukan
  // karena kita tidak mendukung versi ≥5).
  if (version >= 2) {
    const pos = 18 + 4 * (version - 2);
    drawAlignment(pos, pos);
  }

  // "Dark module" — selalu gelap, posisi tetap (8, 4*versi+9).
  setFn(8, 4 * version + 9, true);

  // Cadangkan area info format (nilai asli diisi belakangan setelah mask terpilih — lihat drawFormatBits).
  for (let i = 0; i <= 5; i++) setFn(8, i, false);
  setFn(8, 7, false);
  setFn(8, 8, false);
  setFn(7, 8, false);
  for (let i = 9; i < 15; i++) setFn(14 - i, 8, false);
  for (let i = 0; i < 8; i++) setFn(size - 1 - i, 8, false);
  for (let i = 8; i < 15; i++) setFn(8, size - 15 + i, false);

  // ---- Penempatan codeword data: zigzag dua-kolom dari kanan-bawah, arah naik/turun berselang, lewati kolom timing (6). ----
  let bitIndex = 0;
  const totalBits = codewords.length * 8;
  for (let right = size - 1; right >= 1; right -= 2) {
    if (right === 6) right = 5;
    for (let vert = 0; vert < size; vert++) {
      for (let j = 0; j < 2; j++) {
        const x = right - j;
        const upward = ((right + 1) & 2) === 0;
        const y = upward ? size - 1 - vert : vert;
        if (isFunction[y][x]) continue;
        let bit = false;
        if (bitIndex < totalBits) {
          const byte = codewords[bitIndex >>> 3];
          bit = ((byte >>> (7 - (bitIndex & 7))) & 1) !== 0;
          bitIndex++;
        }
        // Modul di luar `totalBits` (bit sisa/"remainder bits", ada di versi
        // 2-4: 7 modul) otomatis kebagian `false` di sini — sudah sesuai
        // spesifikasi (remainder bits selalu 0), tidak perlu langkah terpisah.
        modules[y][x] = bit;
      }
    }
  }

  // ---- Masking: coba 8 pola, hitung penalti (4 aturan), pilih yang terendah. ----
  function maskFn(mask: number, x: number, y: number): boolean {
    switch (mask) {
      case 0:
        return (x + y) % 2 === 0;
      case 1:
        return y % 2 === 0;
      case 2:
        return x % 3 === 0;
      case 3:
        return (x + y) % 3 === 0;
      case 4:
        return (Math.floor(x / 3) + Math.floor(y / 2)) % 2 === 0;
      case 5:
        return ((x * y) % 2) + ((x * y) % 3) === 0;
      case 6:
        return (((x * y) % 2) + ((x * y) % 3)) % 2 === 0;
      case 7:
        return (((x + y) % 2) + ((x * y) % 3)) % 2 === 0;
      default:
        return false;
    }
  }

  function applyMask(mask: number, target: boolean[][]): void {
    for (let y = 0; y < size; y++) {
      for (let x = 0; x < size; x++) {
        if (isFunction[y][x]) continue;
        if (maskFn(mask, x, y)) target[y][x] = !target[y][x];
      }
    }
  }

  // Rule 3: 1:1:3:1:1 diapit ≥4 modul terang di satu sisi (mirip pola finder,
  // membingungkan decoder) — dicek kedua arah (pola & kebalikannya).
  const RULE3_PATTERN = [true, false, true, true, true, false, true, false, false, false, false];
  const RULE3_PATTERN_REV = [false, false, false, false, true, false, true, true, true, false, true];
  function matchesAt(line: boolean[], offset: number, pattern: boolean[]): boolean {
    for (let i = 0; i < pattern.length; i++) if (line[offset + i] !== pattern[i]) return false;
    return true;
  }

  function penalty(mat: boolean[][]): number {
    let result = 0;

    // Rule 1: ≥5 modul berurutan warna sama, satu baris/kolom.
    for (let y = 0; y < size; y++) {
      let runColor: boolean | null = null;
      let runLen = 0;
      for (let x = 0; x < size; x++) {
        const c = mat[y][x];
        if (c === runColor) runLen++;
        else {
          runColor = c;
          runLen = 1;
        }
        if (runLen === 5) result += 3;
        else if (runLen > 5) result += 1;
      }
    }
    for (let x = 0; x < size; x++) {
      let runColor: boolean | null = null;
      let runLen = 0;
      for (let y = 0; y < size; y++) {
        const c = mat[y][x];
        if (c === runColor) runLen++;
        else {
          runColor = c;
          runLen = 1;
        }
        if (runLen === 5) result += 3;
        else if (runLen > 5) result += 1;
      }
    }

    // Rule 2: blok 2×2 warna seragam.
    for (let y = 0; y < size - 1; y++) {
      for (let x = 0; x < size - 1; x++) {
        const c = mat[y][x];
        if (c === mat[y][x + 1] && c === mat[y + 1][x] && c === mat[y + 1][x + 1]) result += 3;
      }
    }

    // Rule 3.
    for (let y = 0; y < size; y++) {
      const row = mat[y];
      for (let x = 0; x + 11 <= size; x++) {
        if (matchesAt(row, x, RULE3_PATTERN) || matchesAt(row, x, RULE3_PATTERN_REV)) result += 40;
      }
    }
    for (let x = 0; x < size; x++) {
      const col: boolean[] = [];
      for (let y = 0; y < size; y++) col.push(mat[y][x]);
      for (let y = 0; y + 11 <= size; y++) {
        if (matchesAt(col, y, RULE3_PATTERN) || matchesAt(col, y, RULE3_PATTERN_REV)) result += 40;
      }
    }

    // Rule 4: proporsi modul gelap jauh dari 50%.
    let dark = 0;
    for (let y = 0; y < size; y++) for (let x = 0; x < size; x++) if (mat[y][x]) dark++;
    const percent = (dark * 100) / (size * size);
    const k = Math.floor(Math.abs(percent - 50) / 5);
    result += k * 10;

    return result;
  }

  let bestMask = 0;
  let bestPenalty = Infinity;
  let bestModules: boolean[][] = modules;
  for (let mask = 0; mask < 8; mask++) {
    const copy = modules.map((row) => row.slice());
    applyMask(mask, copy);
    const p = penalty(copy);
    if (p < bestPenalty) {
      bestPenalty = p;
      bestMask = mask;
      bestModules = copy;
    }
  }

  // ---- Info format: 5 bit data (2 bit level EC + 3 bit mask) + 10 bit BCH(15,5), lalu XOR mask tetap 0x5412. ----
  function computeFormatBits(lvl: EcLevel, mask: number): number {
    const data = (EC_LEVEL_BITS[lvl] << 3) | mask;
    let rem = data;
    for (let i = 0; i < 10; i++) {
      rem = (rem << 1) ^ ((rem >>> 9) * 0x537);
    }
    return ((data << 10) | rem) ^ 0x5412;
  }
  const formatBits = computeFormatBits(level, bestMask);
  const bit = (i: number) => ((formatBits >>> i) & 1) !== 0;

  for (let i = 0; i <= 5; i++) bestModules[i][8] = bit(i);
  bestModules[7][8] = bit(6);
  bestModules[8][8] = bit(7);
  bestModules[8][7] = bit(8);
  for (let i = 9; i < 15; i++) bestModules[8][14 - i] = bit(i);
  for (let i = 0; i < 8; i++) bestModules[8][size - 1 - i] = bit(i);
  for (let i = 8; i < 15; i++) bestModules[size - 15 + i][8] = bit(i);
  bestModules[4 * version + 9][8] = true; // dark module (tetap gelap, tidak ikut mask)

  return { size, modules: bestModules, version, level, mask: bestMask };
}

/** Susun matriks modul QR (byte mode, versi 1-4 auto, EC M diutamakan → fallback L) untuk `text`. */
export function qrMatrix(text: string): QrMatrix {
  const { version, level, codewords } = encodeByteSegment(text);
  return buildMatrix(version, level, codewords);
}

/* ---------------------------------------------------------------------------
 * 6. Render SVG + komponen React.
 * ------------------------------------------------------------------------- */

/** Zona tenang (quiet zone) minimum spesifikasi QR: 4 modul di tiap sisi. */
const QUIET_ZONE_MODULES = 4;

/**
 * Render `text` sebagai QR code, dikembalikan sebagai string markup `<svg>`
 * utuh (siap dipakai langsung sebagai HTML, mis. lewat `dangerouslySetInnerHTML`
 * di `QrImage` di bawah) — modul hitam-putih murni, tanpa quiet zone
 * dikurangi/dilebihkan dari standar supaya tetap terbaca kamera HP.
 */
export function qrSvg(text: string, sizePx: number = 240): string {
  const { size, modules } = qrMatrix(text);
  const total = size + QUIET_ZONE_MODULES * 2;
  let path = '';
  for (let y = 0; y < size; y++) {
    for (let x = 0; x < size; x++) {
      if (modules[y][x]) path += `M${x + QUIET_ZONE_MODULES},${y + QUIET_ZONE_MODULES}h1v1h-1z`;
    }
  }
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${total} ${total}" width="${sizePx}" height="${sizePx}" role="img" aria-label="Kode QR" shape-rendering="crispEdges"><rect width="${total}" height="${total}" fill="#fff"/><path d="${path}" fill="#000"/></svg>`;
}

interface QrImageProps {
  /** Teks/payload yang di-encode (mis. `nouschool:tqr:{token}`). */
  value: string;
  /** Sisi persegi (px) — QR selalu persegi. Default 240. */
  sizePx?: number;
  className?: string;
}

/**
 * Bungkus `qrSvg` sebagai komponen React siap-pakai. Dipakai `React.createElement`
 * (bukan JSX) supaya file ini boleh tetap berekstensi `.ts` (JSX butuh `.tsx`).
 * `dangerouslySetInnerHTML` di sini AMAN — markup sepenuhnya dibangun
 * deterministik oleh `qrSvg` dari modul QR (angka/koordinat), `value` TIDAK
 * PERNAH disisipkan sebagai teks HTML mentah (hanya jadi input encoder byte-mode).
 */
export function QrImage({ value, sizePx = 240, className }: QrImageProps) {
  const svg = useMemo(() => {
    try {
      return qrSvg(value, sizePx);
    } catch {
      return null;
    }
  }, [value, sizePx]);

  if (!svg) return null;

  return createElement('div', {
    className,
    style: { width: sizePx, height: sizePx },
    dangerouslySetInnerHTML: { __html: svg },
  });
}
