// Client-side photo preparation: phone cameras produce 4000+ px images of several MB, so we
// downscale and re-encode before uploading. Re-encoding through a canvas also drops all EXIF
// metadata, including GPS location, which keeps where the photo was taken off the server.

export const MAX_EDGE = 2560;
const JPEG_QUALITY = 0.85;

/** The browser can't decode the file, e.g. HEIC on Chrome for Android. */
export class UnsupportedImageError extends Error {
  constructor() {
    super("This photo's format isn't supported. Try a JPEG, or set your camera to save JPEG.");
    this.name = "UnsupportedImageError";
  }
}

/** Scales to fit `maxEdge` on the long side, keeping the aspect ratio. Never upscales. */
export function targetSize(width: number, height: number, maxEdge = MAX_EDGE): { width: number; height: number } {
  const scale = Math.min(1, maxEdge / Math.max(width, height));
  return { width: Math.max(1, Math.round(width * scale)), height: Math.max(1, Math.round(height * scale)) };
}

export interface PreparedPhoto {
  blob: Blob;
  /** The file's modified time, the best guess at when it was taken once EXIF is gone. */
  takenAt: string;
}

export async function preparePhoto(file: File): Promise<PreparedPhoto> {
  let bitmap: ImageBitmap;
  try {
    // "from-image" applies the EXIF orientation, so portrait photos stay upright.
    bitmap = await createImageBitmap(file, { imageOrientation: "from-image" });
  } catch {
    throw new UnsupportedImageError();
  }
  try {
    const { width, height } = targetSize(bitmap.width, bitmap.height);
    const blob = await encodeJpeg(bitmap, width, height);
    return { blob, takenAt: new Date(file.lastModified).toISOString() };
  } finally {
    bitmap.close();
  }
}

async function encodeJpeg(bitmap: ImageBitmap, width: number, height: number): Promise<Blob> {
  if (typeof OffscreenCanvas !== "undefined") {
    const canvas = new OffscreenCanvas(width, height);
    draw(canvas.getContext("2d"), bitmap, width, height);
    return canvas.convertToBlob({ type: "image/jpeg", quality: JPEG_QUALITY });
  }
  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;
  draw(canvas.getContext("2d"), bitmap, width, height);
  return new Promise((resolve, reject) =>
    canvas.toBlob(
      (blob) => (blob ? resolve(blob) : reject(new Error("Couldn't encode the photo."))),
      "image/jpeg",
      JPEG_QUALITY,
    ),
  );
}

function draw(
  ctx: OffscreenCanvasRenderingContext2D | CanvasRenderingContext2D | null,
  bitmap: ImageBitmap,
  width: number,
  height: number,
) {
  if (!ctx) throw new Error("Canvas 2D isn't available.");
  ctx.imageSmoothingQuality = "high";
  ctx.drawImage(bitmap, 0, 0, width, height);
}
