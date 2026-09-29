import type { ICellData, IWorkbookData } from '@univerjs/core';
import { LocaleType, mergeLocales } from '@univerjs/core';
import { UniverDocsPlugin } from '@univerjs/docs';
import { UniverDocsUIPlugin } from '@univerjs/docs-ui';
import { UniverFormulaEnginePlugin } from '@univerjs/engine-formula';
import { UniverRenderEnginePlugin } from '@univerjs/engine-render';
import { createUniver } from '@univerjs/presets';
import { UniverSheetsPlugin } from '@univerjs/sheets';
import { UniverSheetsFormulaPlugin } from '@univerjs/sheets-formula';
import { UniverSheetsFormulaUIPlugin } from '@univerjs/sheets-formula-ui';
import { UniverSheetsNumfmtPlugin } from '@univerjs/sheets-numfmt';
import { UniverSheetsNumfmtUIPlugin } from '@univerjs/sheets-numfmt-ui';
import { UniverSheetsUIPlugin } from '@univerjs/sheets-ui';
import { UniverUIPlugin } from '@univerjs/ui';
import UniverPresetSheetsCoreEnUS from '@univerjs/preset-sheets-core/locales/en-US';

import '@univerjs/docs-ui/facade';
import '@univerjs/engine-formula/facade';
import '@univerjs/sheets/facade';
import '@univerjs/sheets-formula/facade';
import '@univerjs/sheets-formula-ui/facade';
import '@univerjs/sheets-numfmt/facade';
import '@univerjs/sheets-ui/facade';
import '@univerjs/ui/facade';

import type {
  CellEdit,
  CellInput,
  RangeEdit,
  SelectionChange,
  WorkbookEvent,
  WorkbookRange,
  WorkbookSnapshot,
  WorkbookOperation,
} from './contracts';
import type { SpreadsheetEngine } from './spreadsheet-engine';

export class UniverSpreadsheetEngine implements SpreadsheetEngine {
  private readonly univer;
  private readonly univerAPI;
  private editListener: ((edit: CellEdit) => void) | undefined;
  private rangeEditListener: ((edit: RangeEdit) => void) | undefined;
  private selectionListener: ((selection: SelectionChange) => void) | undefined;
  private operationListener: ((operation: WorkbookOperation) => void) | undefined;
  private readonly subscriptions: Array<{ dispose(): void }> = [];
  private remoteHighlight: { dispose(): void } | undefined;
  private remoteHighlightTimer: number | undefined;
  private applyingRemote = false;

  constructor(container: HTMLElement) {
    const { univer, univerAPI } = createUniver({
      locale: LocaleType.EN_US,
      locales: {
        [LocaleType.EN_US]: mergeLocales(UniverPresetSheetsCoreEnUS),
      },
      presets: [],
      plugins: [
        UniverDocsPlugin,
        UniverRenderEnginePlugin,
        [UniverUIPlugin, { container, ribbonType: 'simple' }],
        UniverDocsUIPlugin,
        UniverFormulaEnginePlugin,
        UniverSheetsPlugin,
        UniverSheetsUIPlugin,
        UniverSheetsNumfmtPlugin,
        UniverSheetsNumfmtUIPlugin,
        UniverSheetsFormulaPlugin,
        UniverSheetsFormulaUIPlugin,
      ],
    });
    this.univer = univer;
    this.univerAPI = univerAPI;
  }

  initialize(snapshot: WorkbookSnapshot, ranges: WorkbookRange[]): void {
    this.univerAPI.createWorkbook(toUniverWorkbook(snapshot, ranges));
    this.subscriptions.push(this.univerAPI.addEvent(
      this.univerAPI.Event.SheetEditEnded,
      ({ worksheet, row, column, isConfirm }) => {
        if (this.applyingRemote || !isConfirm || !this.editListener) {
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
    ));
    this.subscriptions.push(this.univerAPI.addEvent(
      this.univerAPI.Event.CommandExecuted,
      ({ id }) => {
        if (this.applyingRemote || !this.operationListener) {
          return;
        }
        const worksheet = this.univerAPI.getActiveWorkbook()?.getActiveSheet();
        const range = worksheet?.getActiveRange();
        if (!worksheet || !range) {
          return;
        }
        if (id === 'sheet.command.add-worksheet-merge') {
          this.operationListener({ type: 'merge_cells', sheet: worksheet.getSheetName(), range: range.getA1Notation() });
          return;
        }
        if (id === 'sheet.command.remove-worksheet-merge') {
          this.operationListener({ type: 'unmerge_cells', sheet: worksheet.getSheetName(), range: range.getA1Notation() });
          return;
        }
        const format = formatAfterCommand(id, range);
        if (format) {
          this.operationListener({
            type: 'set_format',
            sheet: worksheet.getSheetName(),
            range: range.getA1Notation(),
            format,
          });
        }
      },
    ));
    this.subscriptions.push(this.univerAPI.addEvent(
      this.univerAPI.Event.SelectionChanged,
      ({ worksheet, selections }) => {
        const selection = selections.at(-1);
        if (!selection || !this.selectionListener) {
          return;
        }
        this.selectionListener({
          sheet: worksheet.getSheetName(),
          range: toA1Range(selection.startRow, selection.startColumn, selection.endRow, selection.endColumn),
          state: 'selecting',
        });
      },
    ));
    this.subscriptions.push(this.univerAPI.addEvent(
      this.univerAPI.Event.ClipboardPasted,
      ({ worksheet }) => {
        const range = worksheet.getActiveRange();
        if (this.applyingRemote || !range || !this.rangeEditListener) {
          return;
        }
        const cellData = range.getCellDataGrid();
        this.rangeEditListener({
          sheet: worksheet.getSheetName(),
          range: range.getA1Notation(),
          cells: cellData.map((row) => row.map(toCellInput)),
        });
      },
    ));
  }

  onCellEdit(listener: (edit: CellEdit) => void): void {
    this.editListener = listener;
  }

  onRangeEdit(listener: (edit: RangeEdit) => void): void {
    this.rangeEditListener = listener;
  }

  onSelectionChange(listener: (selection: SelectionChange) => void): void {
    this.selectionListener = listener;
  }

  onOperation(listener: (operation: WorkbookOperation) => void): void {
    this.operationListener = listener;
  }

  applyRemoteEvent(event: WorkbookEvent): void {
    if (!event.sheet) {
      return;
    }
    const workbook = this.univerAPI.getActiveWorkbook();
    const worksheet = workbook?.getSheetByName(event.sheet);
    if (!worksheet) {
      return;
    }
    if (event.type === 'presence.update' && event.range) {
      const editing = event.state === 'editing';
      window.clearTimeout(this.remoteHighlightTimer);
      this.remoteHighlight?.dispose();
      this.remoteHighlight = worksheet.getRange(event.range).highlight({
        stroke: '#7c3aed',
        strokeWidth: editing ? 3 : 2,
        strokeDash: editing ? 6 : 0,
        isAnimationDash: editing,
        fill: 'rgba(124, 58, 237, 0.10)',
      });
      this.remoteHighlightTimer = window.setTimeout(() => {
        this.remoteHighlight?.dispose();
        this.remoteHighlight = undefined;
      }, 5000);
      return;
    }
    if (event.type === 'range.format' && event.range && event.format) {
      const range = worksheet.getRange(event.range);
      this.applyingRemote = true;
      try {
        if (event.format.bold !== undefined) range.setFontWeight(event.format.bold ? 'bold' : 'normal');
        if (event.format.italic !== undefined) range.setFontStyle(event.format.italic ? 'italic' : 'normal');
        if (event.format.fontFamily !== undefined) range.setFontFamily(event.format.fontFamily);
        if (event.format.fontSize !== undefined) range.setFontSize(event.format.fontSize);
        if (event.format.fontColor !== undefined) range.setFontColor(event.format.fontColor);
        if (event.format.fillColor !== undefined) range.setBackgroundColor(event.format.fillColor);
        if (event.format.numberFormat !== undefined) range.setNumberFormat(event.format.numberFormat);
      } finally {
        this.applyingRemote = false;
      }
      return;
    }
    if (event.type === 'range.merge_cells' && event.range) {
      this.applyingRemote = true;
      try {
        worksheet.getRange(event.range).merge({ defaultMerge: true, isForceMerge: true });
      } finally {
        this.applyingRemote = false;
      }
      return;
    }
    if (event.type === 'range.unmerge_cells' && event.range) {
      this.applyingRemote = true;
      try {
        worksheet.getRange(event.range).breakApart();
      } finally {
        this.applyingRemote = false;
      }
      return;
    }
    if (event.type === 'cell.commit' && event.cell) {
      const change = event.cells?.[0];
      const range = worksheet.getRange(event.cell);
      this.applyingRemote = true;
      try {
        if (change?.formula) {
          range.setFormula(normalizeFormula(change.formula));
        } else {
          range.setValueForCell(toUniverCellData(change ? change.value : event.text ?? ''));
        }
      } finally {
        this.applyingRemote = false;
      }
      return;
    }
    if (event.type === 'range.commit' && event.cells) {
      this.applyingRemote = true;
      try {
        for (const change of event.cells) {
          const range = worksheet.getRange(change.cell);
          if (change.formula) {
            range.setFormula(normalizeFormula(change.formula));
          } else {
            range.setValueForCell(toUniverCellData(change.value));
          }
        }
      } finally {
        this.applyingRemote = false;
      }
    }
  }

  dispose(): void {
    window.clearTimeout(this.remoteHighlightTimer);
    this.remoteHighlight?.dispose();
    for (const subscription of this.subscriptions) {
      subscription.dispose();
    }
    this.univer.dispose();
  }

}

function toCellInput(cell: ICellData | null | undefined | void): CellInput {
  if (!cell) {
    return { value: null };
  }
  if (cell.f) {
    return { formula: cell.f };
  }
  return { value: cell.v ?? null };
}

function toUniverCellData(value: unknown): ICellData {
  if (value == null) {
    return { v: null };
  }
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
    return { v: value };
  }
  return { v: String(value) };
}

function toA1Range(startRow: number, startColumn: number, endRow: number, endColumn: number): string {
  const start = `${columnName(startColumn)}${startRow + 1}`;
  const end = `${columnName(endColumn)}${endRow + 1}`;
  return start === end ? start : `${start}:${end}`;
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

function parseCellAddress(address: string): { row: number; column: number } {
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

function normalizeFormula(formula: string): string {
  return formula.startsWith('=') ? formula : `=${formula}`;
}

function formatAfterCommand(
  commandID: string,
  range: {
    getCellStyle(): {
      bold: boolean;
      italic: boolean;
      fontFamily?: string | null | void;
      fontSize?: number;
      color?: { rgb?: string | null | void } | null | void;
      background?: { rgb?: string | null | void } | null | void;
      numberFormat?: { pattern: string } | null | void;
    } | null;
  },
): import('./contracts').CellFormat | undefined {
  const style = range.getCellStyle();
  if (!style) return undefined;
  switch (commandID) {
    case 'sheet.command.set-bold': return { bold: style.bold };
    case 'sheet.command.set-italic': return { italic: style.italic };
    case 'sheet.command.set-font-family': return { fontFamily: style.fontFamily ?? '' };
    case 'sheet.command.set-font-size': return { fontSize: style.fontSize ?? 11 };
    case 'sheet.command.set-text-color':
    case 'sheet.command.reset-text-color': return { fontColor: style.color?.rgb ?? '' };
    case 'sheet.command.set-background-color':
    case 'sheet.command.reset-background-color': return { fillColor: style.background?.rgb ?? '' };
    default:
      if (commandID.startsWith('sheet.command.numfmt.')) {
        return { numberFormat: style.numberFormat?.pattern ?? '' };
      }
      return undefined;
  }
}
