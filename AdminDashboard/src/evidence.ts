/** Neutral names never disclose the original customer filename or identity. */
export function evidenceFilename(mime: string): string {
  const extensions: Record<string, string> = {
    'image/png': 'png',
    'image/jpeg': 'jpg',
    'application/pdf': 'pdf',
  };
  const type = mime.split(';', 1)[0].trim().toLowerCase();
  if (!Object.hasOwn(extensions, type)) throw new Error('Unsupported private evidence type. Do not open this response.');
  return `private-evidence.${extensions[type]}`;
}
