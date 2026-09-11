export const QNS_MAX_NAME_BYTES = 255;

const QNS_LABEL_PATTERN = /^[a-z0-9-]+$/;

/**
 * Apply the conservative ASCII QNS profile used by the QNS SDK and ZondScan
 * backend. Only names below .qrl are accepted for explorer resolution.
 */
export function normalizeQnsName(name: string): string | null {
  if (name.length === 0 || name.length > QNS_MAX_NAME_BYTES) return null;

  let lowered = '';
  for (const character of name) {
    const codePoint = character.codePointAt(0);
    if (codePoint === undefined) return null;
    lowered +=
      codePoint >= 0x41 && codePoint <= 0x5a
        ? String.fromCharCode(codePoint + 32)
        : character;
  }

  const labels = lowered.split('.');
  if (labels.length < 2 || labels[labels.length - 1] !== 'qrl') return null;

  for (const label of labels) {
    if (
      label.length === 0 ||
      !QNS_LABEL_PATTERN.test(label) ||
      (label.length >= 4 && label[2] === '-' && label[3] === '-')
    ) {
      return null;
    }
  }

  return labels.join('.');
}

export function hasQnsSuffix(name: string): boolean {
  return name.toLowerCase().endsWith('.qrl');
}
