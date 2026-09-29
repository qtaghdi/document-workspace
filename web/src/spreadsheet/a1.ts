export function toA1Range(
  startRow: number,
  startColumn: number,
  endRow: number,
  endColumn: number,
): string {
  const start = `${columnName(startColumn)}${startRow + 1}`;
  const end = `${columnName(endColumn)}${endRow + 1}`;
  return start === end ? start : `${start}:${end}`;
}

export function parseCellAddress(address: string): { row: number; column: number } {
  const match = /^([A-Z]+)([1-9][0-9]*)$/i.exec(address);
  if (!match) {
    throw new Error(`Invalid cell address ${address}`);
  }
  let column = 0;
  for (const character of match[1]!.toUpperCase()) {
    column = column * 26 + character.charCodeAt(0) - 64;
  }
  return { row: Number(match[2]) - 1, column: column - 1 };
}

function columnName(column: number): string {
  let value = column + 1;
  let name = '';
  while (value > 0) {
    value -= 1;
    name = String.fromCharCode(65 + (value % 26)) + name;
    value = Math.floor(value / 26);
  }
  return name;
}
