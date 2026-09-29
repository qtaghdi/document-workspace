import type { ICellData, IWorkbookData } from '@univerjs/core';
import { LocaleType } from '@univerjs/core';

import type { CellInput, WorkbookRange, WorkbookSnapshot } from '../../contracts';
import { parseCellAddress } from '../a1';

export function toCellInput(cell: ICellData | null | undefined | void): CellInput {
  if (!cell) {
    return { value: null };
  }
  if (cell.f) {
    return { formula: cell.f };
  }
  return { value: cell.v ?? null };
}

export function toUniverCellData(value: unknown): ICellData {
  if (value == null) {
    return { v: null };
  }
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
    return { v: value };
  }
  return { v: String(value) };
}

export function normalizeFormula(formula: string): string {
  return formula.startsWith('=') ? formula : `=${formula}`;
}

export function toUniverWorkbook(
  snapshot: WorkbookSnapshot,
  ranges: WorkbookRange[],
): IWorkbookData {
  const sheetOrder = snapshot.sheets.map((_, index) => `sheet-${index + 1}`);
  const sheets = Object.fromEntries(
    snapshot.sheets.map((name, index) => {
      const range = ranges.find((candidate) => candidate.sheet === name);
      const id = sheetOrder[index];
      if (!id) {
        throw new Error(`Missing worksheet ID for ${name}`);
      }
      return [
        id,
        {
          id,
          name,
          rowCount: Math.max(200, range?.rows.length ?? 0),
          columnCount: 50,
          cellData: toCellData(range),
          mergeData: toMergeData(range?.merges ?? []),
        },
      ];
    }),
  );

  return {
    id: snapshot.id,
    name: snapshot.name,
    appVersion: '1.0.2',
    locale: LocaleType.EN_US,
    sheetOrder,
    sheets,
    styles: {},
  };
}

function toCellData(range: WorkbookRange | undefined): Record<number, Record<number, ICellData>> {
  if (!range) {
    return {};
  }
  return Object.fromEntries(
    range.rows.map((row, rowIndex) => [
      rowIndex,
      Object.fromEntries(
        row.map((cell, columnIndex) => {
          const data: ICellData = cell.formula
            ? { f: normalizeFormula(cell.formula) }
            : { v: cell.value };
          if (cell.style) {
            data.s = {
              bl: cell.style.bold ? 1 : undefined,
              it: cell.style.italic ? 1 : undefined,
              ff: cell.style.fontFamily || undefined,
              fs: cell.style.fontSize || undefined,
              cl: cell.style.fontColor ? { rgb: cell.style.fontColor } : undefined,
              bg: cell.style.fillColor ? { rgb: cell.style.fillColor } : undefined,
              n: cell.style.numberFormat ? { pattern: cell.style.numberFormat } : undefined,
            };
          }
          return [columnIndex, data];
        }),
      ),
    ]),
  );
}

function toMergeData(merges: string[]): Array<{
  startRow: number;
  startColumn: number;
  endRow: number;
  endColumn: number;
}> {
  return merges.map((merge) => {
    const [start, end = start] = merge.split(':');
    const first = parseCellAddress(start ?? 'A1');
    const last = parseCellAddress(end ?? start ?? 'A1');
    return {
      startRow: first.row,
      startColumn: first.column,
      endRow: last.row,
      endColumn: last.column,
    };
  });
}
