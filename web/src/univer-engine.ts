import type { ICellData, IWorkbookData } from '@univerjs/presets';
import { createUniver, LocaleType, mergeLocales } from '@univerjs/presets';
import { UniverSheetsCorePreset } from '@univerjs/preset-sheets-core';
import UniverPresetSheetsCoreEnUS from '@univerjs/preset-sheets-core/locales/en-US';

import type {
  CellEdit,
  WorkbookEvent,
  WorkbookRange,
  WorkbookSnapshot,
} from './contracts';
import type { SpreadsheetEngine } from './spreadsheet-engine';

export class UniverSpreadsheetEngine implements SpreadsheetEngine {
  private readonly univer;
  private readonly univerAPI;
  private editListener: ((edit: CellEdit) => void) | undefined;
  private editSubscription: { dispose(): void } | undefined;

  constructor(container: HTMLElement) {
    const { univer, univerAPI } = createUniver({
      locale: LocaleType.EN_US,
      locales: {
        [LocaleType.EN_US]: mergeLocales(UniverPresetSheetsCoreEnUS),
      },
      presets: [
        UniverSheetsCorePreset({
          container,
          ribbonType: 'simple',
        }),
      ],
    });
    this.univer = univer;
    this.univerAPI = univerAPI;
  }

  initialize(snapshot: WorkbookSnapshot, ranges: WorkbookRange[]): void {
    this.univerAPI.createWorkbook(toUniverWorkbook(snapshot, ranges));
    this.editSubscription = this.univerAPI.addEvent(
      this.univerAPI.Event.SheetEditEnded,
      ({ worksheet, row, column, isConfirm }) => {
        if (!isConfirm || !this.editListener) {
          return;
        }
        const range = worksheet.getRange(row, column);
        const formula = range.getFormula();
        this.editListener({
          sheet: worksheet.getSheetName(),
          cell: range.getA1Notation(),
          value: range.getRawValue(),
          formula: formula || undefined,
        });
      },
    );
  }

  onCellEdit(listener: (edit: CellEdit) => void): void {
    this.editListener = listener;
  }

  applyRemoteEvent(event: WorkbookEvent): void {
    if (event.type !== 'cell.commit' || !event.sheet || !event.cell) {
      return;
    }
    const workbook = this.univerAPI.getActiveWorkbook();
    const worksheet = workbook?.getSheetByName(event.sheet);
    if (!worksheet) {
      return;
    }
    worksheet.getRange(event.cell).setValueForCell(event.text ?? '');
  }

  dispose(): void {
    this.editSubscription?.dispose();
    this.univer.dispose();
  }
}

function toUniverWorkbook(
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
        row.map((cell, columnIndex) => [
          columnIndex,
          cell.formula ? { f: normalizeFormula(cell.formula) } : { v: cell.value },
        ]),
      ),
    ]),
  );
}

function normalizeFormula(formula: string): string {
  return formula.startsWith('=') ? formula : `=${formula}`;
}
