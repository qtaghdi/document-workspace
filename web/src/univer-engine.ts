import { LocaleType, mergeLocales } from '@univerjs/core';
import type { IRange } from '@univerjs/core';
import { UniverDocsPlugin } from '@univerjs/docs';
import { UniverDocsDrawingPlugin } from '@univerjs/docs-drawing';
import { UniverDocsUIPlugin } from '@univerjs/docs-ui';
import { UniverDrawingPlugin } from '@univerjs/drawing';
import { UniverDrawingUIPlugin } from '@univerjs/drawing-ui';
import DrawingUIEnUS from '@univerjs/drawing-ui/locale/en-US';
import { UniverFormulaEnginePlugin } from '@univerjs/engine-formula';
import { UniverRenderEnginePlugin } from '@univerjs/engine-render';
import { createUniver } from '@univerjs/presets';
import { UniverSheetsPlugin } from '@univerjs/sheets';
import type { FWorksheet } from '@univerjs/sheets/facade';
import { UniverSheetsFormulaPlugin } from '@univerjs/sheets-formula';
import { UniverSheetsFormulaUIPlugin } from '@univerjs/sheets-formula-ui';
import { UniverSheetsNumfmtPlugin } from '@univerjs/sheets-numfmt';
import { UniverSheetsNumfmtUIPlugin } from '@univerjs/sheets-numfmt-ui';
import { UniverSheetsUIPlugin } from '@univerjs/sheets-ui';
import { UniverSheetsDrawingPlugin } from '@univerjs/sheets-drawing';
import { UniverSheetsDrawingUIPlugin } from '@univerjs/sheets-drawing-ui';
import SheetsDrawingUIEnUS from '@univerjs/sheets-drawing-ui/locale/en-US';
import { UniverSheetsDataValidationPlugin } from '@univerjs/sheets-data-validation';
import { UniverSheetsDataValidationUIPlugin } from '@univerjs/sheets-data-validation-ui';
import type { FDataValidationBuilder } from '@univerjs/sheets-data-validation/facade';
import { UniverSheetsConditionalFormattingPlugin } from '@univerjs/sheets-conditional-formatting';
import { CFNumberOperator, CFTimePeriodOperator, CFValueType } from '@univerjs/sheets-conditional-formatting';
import type { IConditionFormattingRule, IValueConfig } from '@univerjs/sheets-conditional-formatting';
import type { FConditionalFormattingBuilder } from '@univerjs/sheets-conditional-formatting/facade';
import { UniverUIPlugin } from '@univerjs/ui';
import UniverPresetSheetsCoreEnUS from '@univerjs/preset-sheets-core/locales/en-US';

import '@univerjs/docs-ui/facade';
import '@univerjs/docs-drawing/facade';
import '@univerjs/engine-formula/facade';
import '@univerjs/sheets/facade';
import '@univerjs/sheets-formula/facade';
import '@univerjs/sheets-formula-ui/facade';
import '@univerjs/sheets-numfmt/facade';
import '@univerjs/sheets-ui/facade';
import '@univerjs/sheets-drawing/facade';
import '@univerjs/sheets-drawing-ui/facade';
import '@univerjs/sheets-data-validation/facade';
import '@univerjs/sheets-conditional-formatting/facade';
import '@univerjs/ui/facade';

import type {
  CellEdit,
  RangeEdit,
  SelectionChange,
  WorkbookEvent,
  WorkbookRange,
  WorkbookSnapshot,
  WorkbookOperation,
  ViewportChange,
  HistoryDirection,
  SheetObjects,
  SheetChart,
  SheetImage,
  DataValidation,
  ConditionalFormat,
} from './contracts';
import { parseCellAddress, toA1Range } from './spreadsheet/a1';
import { formatAfterCommand } from './spreadsheet/univer/command-mapper';
import {
  normalizeFormula,
  toCellInput,
  toUniverCellData,
  toUniverWorkbook,
} from './spreadsheet/univer/workbook-mapper';
import type { SpreadsheetEngine } from './spreadsheet-engine';

export class UniverSpreadsheetEngine implements SpreadsheetEngine {
  private readonly univer;
  private readonly univerAPI;
  private editListener: ((edit: CellEdit) => void) | undefined;
  private rangeEditListener: ((edit: RangeEdit) => void) | undefined;
  private selectionListener: ((selection: SelectionChange) => void) | undefined;
  private operationListener: ((operation: WorkbookOperation) => void) | undefined;
  private viewportListener: ((viewport: ViewportChange) => void) | undefined;
  private historyListener: ((direction: HistoryDirection) => void) | undefined;
  private readonly subscriptions: Array<{ dispose(): void }> = [];
  private remoteHighlight: { dispose(): void } | undefined;
  private remoteHighlightTimer: number | undefined;
  private applyingRemote = false;
  private readonly appliedValidations = new Set<string>();
  private readonly appliedConditionalFormats = new Set<string>();
  private readonly objectBindings = new Map<string, SheetObjectBinding>();

  constructor(container: HTMLElement) {
    const { univer, univerAPI } = createUniver({
      locale: LocaleType.EN_US,
      locales: {
        [LocaleType.EN_US]: mergeLocales(
          UniverPresetSheetsCoreEnUS,
          DrawingUIEnUS,
          SheetsDrawingUIEnUS,
        ),
      },
      presets: [],
      plugins: [
        UniverRenderEnginePlugin,
        [UniverUIPlugin, { container, ribbonType: 'simple' }],
        UniverDocsPlugin,
        UniverDocsUIPlugin,
        UniverFormulaEnginePlugin,
        UniverSheetsPlugin,
        UniverSheetsUIPlugin,
        UniverDrawingPlugin,
        UniverDrawingUIPlugin,
        UniverDocsDrawingPlugin,
        UniverSheetsDrawingPlugin,
        UniverSheetsDrawingUIPlugin,
        UniverSheetsNumfmtPlugin,
        UniverSheetsNumfmtUIPlugin,
        UniverSheetsFormulaPlugin,
        UniverSheetsFormulaUIPlugin,
        UniverSheetsDataValidationPlugin,
        UniverSheetsDataValidationUIPlugin,
        UniverSheetsConditionalFormattingPlugin,
      ],
    });
    this.univer = univer;
    this.univerAPI = univerAPI;
  }

  initialize(snapshot: WorkbookSnapshot, ranges: WorkbookRange[], objects: SheetObjects[]): void {
    let objectsApplied = false;
    this.subscriptions.push(this.univerAPI.addEvent(
      this.univerAPI.Event.LifeCycleChanged,
      ({ stage }) => {
        if (objectsApplied || stage !== this.univerAPI.Enum.LifecycleStages.Rendered) {
          return;
        }
        objectsApplied = true;
        void this.applySheetObjects(objects);
      },
    ));
    this.univerAPI.createWorkbook(toUniverWorkbook(snapshot, ranges));
    for (const range of ranges) {
      this.applyWorkbookFeatures(range);
    }
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
      this.univerAPI.Event.BeforeUndo,
      (event) => {
        if (this.applyingRemote) return;
        event.cancel = true;
        this.historyListener?.('undo');
      },
    ));
    this.subscriptions.push(this.univerAPI.addEvent(
      this.univerAPI.Event.BeforeRedo,
      (event) => {
        if (this.applyingRemote) return;
        event.cancel = true;
        this.historyListener?.('redo');
      },
    ));
    this.subscriptions.push(this.univerAPI.addEvent(
      this.univerAPI.Event.CommandExecuted,
      ({ id, params }) => {
        if (this.applyingRemote || !this.operationListener) {
          return;
        }
        const worksheet = this.univerAPI.getActiveWorkbook()?.getActiveSheet();
        if (!worksheet) {
          return;
        }
        if (this.handleSheetObjectCommand(id, params, worksheet)) {
          return;
        }
        const range = worksheet?.getActiveRange();
        if (!range) {
          return;
        }
        const commandRange = commandParameterRange(params) ?? {
          startRow: range.getRow(),
          endRow: range.getRow() + range.getHeight() - 1,
          startColumn: range.getColumn(),
          endColumn: range.getColumn() + range.getWidth() - 1,
        };
        if (id === 'sheet.command.insert-row') {
          this.operationListener({ type: 'insert_rows', sheet: worksheet.getSheetName(), index: commandRange.startRow + 1, count: commandRange.endRow - commandRange.startRow + 1 });
          return;
        }
        if (id === 'sheet.command.remove-row') {
          this.operationListener({ type: 'delete_rows', sheet: worksheet.getSheetName(), index: commandRange.startRow + 1, count: commandRange.endRow - commandRange.startRow + 1 });
          return;
        }
        if (id === 'sheet.command.insert-col') {
          this.operationListener({ type: 'insert_columns', sheet: worksheet.getSheetName(), index: commandRange.startColumn + 1, count: commandRange.endColumn - commandRange.startColumn + 1 });
          return;
        }
        if (id === 'sheet.command.remove-col') {
          this.operationListener({ type: 'delete_columns', sheet: worksheet.getSheetName(), index: commandRange.startColumn + 1, count: commandRange.endColumn - commandRange.startColumn + 1 });
          return;
        }
        if (id === 'sheet.command.add-worksheet-merge') {
          if (range.getWidth() === 1 && range.getHeight() === 1) {
            return;
          }
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
      this.univerAPI.Event.Scroll,
      ({ worksheet }) => {
        if (!this.viewportListener) {
          return;
        }
        const state = worksheet.getScrollState();
        this.viewportListener({
          sheet: worksheet.getSheetName(),
          row: state.sheetViewStartRow,
          column: state.sheetViewStartColumn,
        });
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

  onViewportChange(listener: (viewport: ViewportChange) => void): void {
    this.viewportListener = listener;
  }

  onHistoryAction(listener: (direction: HistoryDirection) => void): void {
    this.historyListener = listener;
  }

  applyRange(source: WorkbookRange): void {
    const worksheet = this.univerAPI.getActiveWorkbook()?.getSheetByName(source.sheet);
    if (!worksheet) {
      return;
    }
    const [start = 'A1'] = source.ref.split(':');
    const offset = parseCellAddress(start);
    this.applyingRemote = true;
    try {
      source.rows.forEach((row, rowOffset) => {
        row.forEach((cell, columnOffset) => {
          const target = worksheet.getRange(offset.row + rowOffset, offset.column + columnOffset);
          if (cell.formula) {
            target.setFormula(normalizeFormula(cell.formula));
          } else {
            target.setValueForCell(toUniverCellData(cell.value));
          }
          if (cell.style?.bold !== undefined) target.setFontWeight(cell.style.bold ? 'bold' : 'normal');
          if (cell.style?.italic !== undefined) target.setFontStyle(cell.style.italic ? 'italic' : 'normal');
          if (cell.style?.fontFamily) target.setFontFamily(cell.style.fontFamily);
          if (cell.style?.fontSize) target.setFontSize(cell.style.fontSize);
          if (cell.style?.fontColor) target.setFontColor(cell.style.fontColor);
          if (cell.style?.fillColor) target.setBackgroundColor(cell.style.fillColor);
          if (cell.style?.numberFormat) target.setNumberFormat(cell.style.numberFormat);
        });
      });
      for (const merge of source.merges ?? []) {
        worksheet.getRange(merge).merge({ defaultMerge: true, isForceMerge: true });
      }
      this.applyWorkbookFeatures(source);
    } finally {
      this.applyingRemote = false;
    }
  }

  private applyWorkbookFeatures(source: WorkbookRange): void {
    const worksheet = this.univerAPI.getActiveWorkbook()?.getSheetByName(source.sheet);
    if (!worksheet) return;
    for (const validation of source.validations ?? []) {
      const key = validationKey(source.sheet, validation);
      if (this.appliedValidations.has(key)) continue;
      const target = worksheet.getRange(validation.range);
      const options = validationOptions(validation);
      const builder = this.univerAPI.newDataValidation();
      let configured = false;

      if (validation.type === 'list') {
        const rangeSource = parseValidationRange(validation.formula1 ?? '', source.sheet);
        if (rangeSource) {
          const sourceWorksheet = this.univerAPI.getActiveWorkbook()?.getSheetByName(rangeSource.sheet);
          if (sourceWorksheet) {
            builder.requireValueInRange(
              sourceWorksheet.getRange(rangeSource.range),
              false,
              !validation.showDropDown,
            );
            configured = true;
          }
        } else {
          const values = parseValidationList(validation.formula1 ?? '');
          if (values.length > 0) {
            builder.requireValueInList(values, false, !validation.showDropDown);
            configured = true;
          }
        }
      } else if (validation.type === 'whole' || validation.type === 'decimal') {
        configured = configureNumberValidation(builder, validation, validation.type === 'whole');
      } else if (validation.type === 'date') {
        configured = configureDateValidation(builder, validation);
      } else if (validation.type === 'custom' && validation.formula1) {
        builder.requireFormulaSatisfied(normalizeFormula(validation.formula1));
        configured = true;
      }

      if (!configured) continue;
      target.setDataValidation(builder.setOptions(options).build());
      this.appliedValidations.add(key);
    }
    for (const format of source.conditionalFormatting ?? []) {
      const key = conditionalFormatKey(source.sheet, format);
      if (this.appliedConditionalFormats.has(key)) continue;
      const base = worksheet.newConditionalFormattingRule();
      const targetRange = worksheet.getRange(format.range).getRange();

      if (format.type === '2_color_scale' || format.type === '3_color_scale') {
        const config = colorScaleConfig(format);
        if (!config) continue;
        const rule = base.setColorScale(config).setRanges([targetRange]).build();
        worksheet.addConditionalFormattingRule(rule);
        this.appliedConditionalFormats.add(key);
        continue;
      }
      if (format.type === 'data_bar') {
        const min = conditionalValue(format.minType, format.minValue);
        const max = conditionalValue(format.maxType, format.maxValue);
        if (!min || !max || !format.barColor) continue;
        const rule = base.setDataBar({
          min,
          max,
          isGradient: !format.barSolid,
          positiveColor: format.barColor,
          nativeColor: '#dc2626',
          isShowValue: !format.barOnly,
        }).setRanges([targetRange]).build();
        worksheet.addConditionalFormattingRule(rule);
        this.appliedConditionalFormats.add(key);
        continue;
      }

      const builder = highlightRuleBuilder(base, format);
      if (!builder) continue;
      applyConditionalStyle(builder, format);
      builder.setRanges([targetRange]);
      worksheet.addConditionalFormattingRule(builder.build());
      this.appliedConditionalFormats.add(key);
    }
  }

  private async applySheetObjects(groups: SheetObjects[]): Promise<void> {
    this.applyingRemote = true;
    try {
      const workbook = this.univerAPI.getActiveWorkbook();
      if (!workbook) return;
      for (const group of groups) {
        const worksheet = workbook.getSheetByName(group.sheet);
        if (!worksheet) continue;
        for (const source of group.images ?? []) {
          const image = await worksheet.newOverGridImage()
            .setSource(`data:${source.mimeType};base64,${source.data}`, this.univerAPI.Enum.ImageSourceType.BASE64)
            .setColumn(source.column)
            .setRow(source.row)
            .setColumnOffset(source.offsetX ?? 0)
            .setRowOffset(source.offsetY ?? 0)
            .setWidth(source.width)
            .setHeight(source.height)
            .buildAsync();
          image.drawingId = sheetObjectDrawingID(group.sheet, 'image', source.id);
          this.objectBindings.set(image.drawingId, { kind: 'image', source });
          worksheet.insertImages([image]);
        }
        for (const chart of group.charts ?? []) {
          const image = await worksheet.newOverGridImage()
            .setSource(chartDataURL(chart), this.univerAPI.Enum.ImageSourceType.BASE64)
            .setColumn(chart.column)
            .setRow(chart.row)
            .setColumnOffset(chart.offsetX ?? 0)
            .setRowOffset(chart.offsetY ?? 0)
            .setWidth(chart.width)
            .setHeight(chart.height)
            .buildAsync();
          image.drawingId = sheetObjectDrawingID(group.sheet, 'chart', chart.id);
          this.objectBindings.set(image.drawingId, { kind: 'chart', source: chart });
          worksheet.insertImages([image]);
        }
      }
    } finally {
      this.applyingRemote = false;
    }
  }

  private handleSheetObjectCommand(
    commandID: string,
    params: unknown,
    worksheet: FWorksheet,
  ): boolean {
    if (commandID !== 'sheet.command.set-sheet-image' && commandID !== 'sheet.command.set-drawing-placement' && commandID !== 'sheet.command.remove-sheet-image') {
      return false;
    }
    const drawingIDs = commandDrawingIDs(params);
    if (drawingIDs.length === 0) return false;
    for (const drawingID of drawingIDs) {
      const binding = this.objectBindings.get(drawingID);
      if (!binding) continue;
      const cell = toA1Range(binding.source.row, binding.source.column, binding.source.row, binding.source.column);
      if (commandID === 'sheet.command.remove-sheet-image') {
        this.operationListener?.({
          type: binding.kind === 'image' ? 'delete_image' : 'delete_chart',
          sheet: worksheet.getSheetName(),
          objectId: binding.source.id,
          cell,
        });
        this.objectBindings.delete(drawingID);
        continue;
      }
      const layout = worksheet.getDrawingLayout().drawings.find((drawing) => drawing.drawingId === drawingID);
      if (!layout) continue;
      const placement = worksheet.resolveDrawingPlacement({
        kind: this.univerAPI.Enum.SheetDrawingAnchorType.Position,
        bounds: layout.bounds,
      });
      if (placement.kind !== this.univerAPI.Enum.SheetDrawingAnchorType.Position) continue;
      const nextSource = {
        ...binding.source,
        row: placement.from.row,
        column: placement.from.column,
        offsetX: Math.max(0, Math.round(placement.from.columnOffset)),
        offsetY: Math.max(0, Math.round(placement.from.rowOffset)),
        width: Math.max(1, Math.round(layout.bounds.width)),
        height: Math.max(1, Math.round(layout.bounds.height)),
      };
      this.operationListener?.({
        type: binding.kind === 'image' ? 'set_image' : 'set_chart',
        sheet: worksheet.getSheetName(),
        objectId: binding.source.id,
        cell,
        targetCell: toA1Range(nextSource.row, nextSource.column, nextSource.row, nextSource.column),
        offsetX: nextSource.offsetX,
        offsetY: nextSource.offsetY,
        width: nextSource.width,
        height: nextSource.height,
      });
      if (binding.kind === 'image') {
        this.objectBindings.set(drawingID, { kind: 'image', source: nextSource as SheetImage });
      } else {
        this.objectBindings.set(drawingID, { kind: 'chart', source: nextSource as SheetChart });
      }
    }
    return true;
  }

  applyRemoteEvent(event: WorkbookEvent): void {
    if (!('sheet' in event) || !event.sheet) {
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
    if (
      event.type === 'sheet.insert_rows' ||
      event.type === 'sheet.delete_rows' ||
      event.type === 'sheet.insert_columns' ||
      event.type === 'sheet.delete_columns'
    ) {
      this.applyingRemote = true;
      try {
        const index = event.index - 1;
        if (event.type === 'sheet.insert_rows') worksheet.insertRows(index, event.count);
        if (event.type === 'sheet.delete_rows') worksheet.deleteRows(index, event.count);
        if (event.type === 'sheet.insert_columns') worksheet.insertColumns(index, event.count);
        if (event.type === 'sheet.delete_columns') worksheet.deleteColumns(index, event.count);
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

type SheetObjectBinding =
  | { kind: 'image'; source: SheetImage }
  | { kind: 'chart'; source: SheetChart };

function sheetObjectDrawingID(sheet: string, kind: SheetObjectBinding['kind'], objectID: string): string {
  return `xlsx-viewer:${encodeURIComponent(sheet)}:${kind}:${objectID}`;
}

function commandDrawingIDs(params: unknown): string[] {
  if (!params || typeof params !== 'object' || !('drawings' in params) || !Array.isArray(params.drawings)) {
    return [];
  }
  return params.drawings.flatMap((drawing) => {
    if (!drawing || typeof drawing !== 'object' || !('drawingId' in drawing) || typeof drawing.drawingId !== 'string') {
      return [];
    }
    return [drawing.drawingId];
  });
}

function commandParameterRange(params: unknown): { startRow: number; endRow: number; startColumn: number; endColumn: number } | undefined {
  if (!params || typeof params !== 'object' || !('range' in params)) {
    return undefined;
  }
  const range = params.range;
  if (!range || typeof range !== 'object') {
    return undefined;
  }
  const candidate = range as Record<string, unknown>;
  if (
    typeof candidate.startRow !== 'number' ||
    typeof candidate.endRow !== 'number' ||
    typeof candidate.startColumn !== 'number' ||
    typeof candidate.endColumn !== 'number'
  ) {
    return undefined;
  }
  return candidate as { startRow: number; endRow: number; startColumn: number; endColumn: number };
}

function parseValidationList(formula: string): string[] {
  const normalized = formula.startsWith('"') && formula.endsWith('"')
    ? formula.slice(1, -1).replaceAll('""', '"')
    : formula;
  return normalized.split(',').map((value) => value.trim()).filter(Boolean);
}

function validationKey(sheet: string, validation: DataValidation): string {
  return [sheet, validation.range, validation.type, validation.operator ?? '', validation.formula1 ?? '', validation.formula2 ?? ''].join('!');
}

function validationOptions(validation: DataValidation) {
  return {
    allowBlank: validation.allowBlank ?? false,
    showErrorMessage: validation.showErrorMessage ?? false,
    error: validation.error,
    errorTitle: validation.errorTitle,
    showInputMessage: validation.showInputMessage ?? false,
    prompt: validation.prompt,
    promptTitle: validation.promptTitle,
  };
}

function parseValidationRange(formula: string, currentSheet: string): { sheet: string; range: string } | undefined {
  const normalized = formula.trim().replace(/^=/, '').replace(/\$/g, '');
  if (!normalized || normalized.startsWith('"') || normalized.includes('[')) return undefined;
  const separator = normalized.lastIndexOf('!');
  const rawSheet = separator >= 0 ? normalized.slice(0, separator) : currentSheet;
  const range = separator >= 0 ? normalized.slice(separator + 1) : normalized;
  if (!/^[A-Z]{1,3}[1-9]\d*(?::[A-Z]{1,3}[1-9]\d*)?$/i.test(range)) return undefined;
  const sheet = rawSheet.startsWith("'") && rawSheet.endsWith("'")
    ? rawSheet.slice(1, -1).replace(/''/g, "'")
    : rawSheet;
  return { sheet, range };
}

function configureNumberValidation(builder: FDataValidationBuilder, validation: DataValidation, integer: boolean): boolean {
  const first = finiteNumber(validation.formula1);
  const second = finiteNumber(validation.formula2);
  switch (validation.operator) {
    case 'between':
      if (first === undefined || second === undefined) return false;
      builder.requireNumberBetween(first, second, integer);
      return true;
    case 'notBetween':
      if (first === undefined || second === undefined) return false;
      builder.requireNumberNotBetween(first, second, integer);
      return true;
    case 'equal':
      if (first === undefined) return false;
      builder.requireNumberEqualTo(first, integer);
      return true;
    case 'notEqual':
      if (first === undefined) return false;
      builder.requireNumberNotEqualTo(first, integer);
      return true;
    case 'greaterThan':
      if (first === undefined) return false;
      builder.requireNumberGreaterThan(first, integer);
      return true;
    case 'greaterThanOrEqual':
      if (first === undefined) return false;
      builder.requireNumberGreaterThanOrEqualTo(first, integer);
      return true;
    case 'lessThan':
      if (first === undefined) return false;
      builder.requireNumberLessThan(first, integer);
      return true;
    case 'lessThanOrEqual':
      if (first === undefined) return false;
      builder.requireNumberLessThanOrEqualTo(first, integer);
      return true;
    default:
      return false;
  }
}

function configureDateValidation(builder: FDataValidationBuilder, validation: DataValidation): boolean {
  const first = excelSerialDate(validation.formula1, validation.date1904 ?? false);
  const second = excelSerialDate(validation.formula2, validation.date1904 ?? false);
  switch (validation.operator) {
    case 'between':
      if (!first || !second) return false;
      builder.requireDateBetween(first, second);
      return true;
    case 'notBetween':
      if (!first || !second) return false;
      builder.requireDateNotBetween(first, second);
      return true;
    case 'equal':
      if (!first) return false;
      builder.requireDateEqualTo(first);
      return true;
    case 'greaterThan':
      if (!first) return false;
      builder.requireDateAfter(first);
      return true;
    case 'greaterThanOrEqual':
      if (!first) return false;
      builder.requireDateOnOrAfter(first);
      return true;
    case 'lessThan':
      if (!first) return false;
      builder.requireDateBefore(first);
      return true;
    case 'lessThanOrEqual':
      if (!first) return false;
      builder.requireDateOnOrBefore(first);
      return true;
    default:
      return false;
  }
}

function excelSerialDate(value: string | undefined, date1904: boolean): Date | undefined {
  const serial = finiteNumber(value);
  if (serial === undefined) return undefined;
  const epoch = date1904 ? Date.UTC(1904, 0, 1) : Date.UTC(1899, 11, 30);
  const date = new Date(epoch + serial * 86_400_000);
  return Number.isNaN(date.getTime()) ? undefined : date;
}

function finiteNumber(value: string | undefined): number | undefined {
  if (value === undefined || value.trim() === '') return undefined;
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : undefined;
}

interface ConditionalHighlightBuilder {
  setBackground(color?: string): ConditionalHighlightBuilder;
  setBold(value: boolean): ConditionalHighlightBuilder;
  setFontColor(color?: string): ConditionalHighlightBuilder;
  setItalic(value: boolean): ConditionalHighlightBuilder;
  setRanges(ranges: IRange[]): ConditionalHighlightBuilder;
  build(): IConditionFormattingRule;
}

function conditionalFormatKey(sheet: string, format: ConditionalFormat): string {
  return [sheet, format.range, format.type, format.criteria ?? '', format.value ?? '', format.minValue ?? '', format.midValue ?? '', format.maxValue ?? ''].join('!');
}

function highlightRuleBuilder(base: FConditionalFormattingBuilder, format: ConditionalFormat): ConditionalHighlightBuilder | undefined {
  const value = finiteNumber(format.value);
  const min = finiteNumber(format.minValue);
  const max = finiteNumber(format.maxValue);
  if (format.type === 'cell') {
    if (format.criteria === 'between' && min !== undefined && max !== undefined) return base.whenNumberBetween(min, max);
    if (format.criteria === 'not between' && min !== undefined && max !== undefined) return base.whenNumberNotBetween(min, max);
    if (value === undefined) return undefined;
    if (format.criteria === '>' || format.criteria === 'greater than') return base.whenNumberGreaterThan(value);
    if (format.criteria === '>=' || format.criteria === 'greater than or equal to') return base.whenNumberGreaterThanOrEqualTo(value);
    if (format.criteria === '<' || format.criteria === 'less than') return base.whenNumberLessThan(value);
    if (format.criteria === '<=' || format.criteria === 'less than or equal to') return base.whenNumberLessThanOrEqualTo(value);
    if (format.criteria === '=' || format.criteria === '==' || format.criteria === 'equal to') return base.whenNumberEqualTo(value);
    if (format.criteria === '!=' || format.criteria === 'not equal to') return base.whenNumberNotEqualTo(value);
    return undefined;
  }
  if (format.type === 'text' && format.value !== undefined) {
    if (format.criteria === 'containing') return base.whenTextContains(format.value);
    if (format.criteria === 'not containing') return base.whenTextDoesNotContain(format.value);
    if (format.criteria === 'begins with') return base.whenTextStartsWith(format.value);
    if (format.criteria === 'ends with') return base.whenTextEndsWith(format.value);
    if (format.criteria === 'equal to') return base.whenTextEqualTo(format.value);
    return undefined;
  }
  if (format.type === 'blanks') return base.whenCellEmpty();
  if (format.type === 'no_blanks') return base.whenCellNotEmpty();
  if (format.type === 'unique') return base.setUniqueValues();
  if (format.type === 'duplicate') return base.setDuplicateValues();
  if ((format.type === 'top' || format.type === 'bottom') && value !== undefined && value > 0) {
    return base.setRank({ isBottom: format.type === 'bottom', isPercent: format.percent ?? false, value });
  }
  if (format.type === 'average') {
    return base.setAverage(format.aboveAverage ? CFNumberOperator.greaterThan : CFNumberOperator.lessThan);
  }
  if (format.type === 'formula' && format.criteria) return base.whenFormulaSatisfied(normalizeFormula(format.criteria));
  if (format.type === 'time_period') {
    const period = timePeriodOperator(format.criteria);
    return period ? base.whenDate(period) : undefined;
  }
  return undefined;
}

function applyConditionalStyle(builder: ConditionalHighlightBuilder, format: ConditionalFormat): void {
  if (format.style?.fillColor) builder.setBackground(format.style.fillColor);
  if (format.style?.fontColor) builder.setFontColor(format.style.fontColor);
  if (format.style?.bold) builder.setBold(true);
  if (format.style?.italic) builder.setItalic(true);
}

function timePeriodOperator(criteria: string | undefined): CFTimePeriodOperator | undefined {
  switch (criteria) {
    case 'today': return CFTimePeriodOperator.today;
    case 'yesterday': return CFTimePeriodOperator.yesterday;
    case 'tomorrow': return CFTimePeriodOperator.tomorrow;
    case 'last 7 days': return CFTimePeriodOperator.last7Days;
    case 'this month': return CFTimePeriodOperator.thisMonth;
    case 'last month': return CFTimePeriodOperator.lastMonth;
    case 'continue month': return CFTimePeriodOperator.nextMonth;
    case 'this week': return CFTimePeriodOperator.thisWeek;
    case 'last week': return CFTimePeriodOperator.lastWeek;
    case 'continue week': return CFTimePeriodOperator.nextWeek;
    default: return undefined;
  }
}

function conditionalValue(type: string | undefined, value: string | undefined): IValueConfig | undefined {
  switch (type) {
    case 'min': return { type: CFValueType.min };
    case 'max': return { type: CFValueType.max };
    case 'num': {
      const parsed = finiteNumber(value);
      return parsed === undefined ? undefined : { type: CFValueType.num, value: parsed };
    }
    case 'percent': {
      const parsed = finiteNumber(value);
      return parsed === undefined ? undefined : { type: CFValueType.percent, value: parsed };
    }
    case 'percentile': {
      const parsed = finiteNumber(value);
      return parsed === undefined ? undefined : { type: CFValueType.percentile, value: parsed };
    }
    case 'formula': return value ? { type: CFValueType.formula, value: normalizeFormula(value) } : undefined;
    default: return undefined;
  }
}

function colorScaleConfig(format: ConditionalFormat): Array<{ index: number; color: string; value: IValueConfig }> | undefined {
  const min = conditionalValue(format.minType, format.minValue);
  const max = conditionalValue(format.maxType, format.maxValue);
  if (!min || !max || !format.minColor || !format.maxColor) return undefined;
  if (format.type === '2_color_scale') {
    return [{ index: 0, color: format.minColor, value: min }, { index: 1, color: format.maxColor, value: max }];
  }
  const mid = conditionalValue(format.midType, format.midValue);
  if (!mid || !format.midColor) return undefined;
  return [
    { index: 0, color: format.minColor, value: min },
    { index: 1, color: format.midColor, value: mid },
    { index: 2, color: format.maxColor, value: max },
  ];
}

const chartColors = ['#2563eb', '#059669', '#dc2626', '#7c3aed', '#d97706', '#0891b2'];

function chartDataURL(chart: SheetChart): string {
  const svg = renderChartSVG(chart);
  const bytes = new TextEncoder().encode(svg);
  let binary = '';
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return `data:image/svg+xml;base64,${window.btoa(binary)}`;
}

function renderChartSVG(chart: SheetChart): string {
  const width = Math.max(240, chart.width);
  const height = Math.max(160, chart.height);
  const title = escapeXML(chart.title || 'Chart preview');
  const frame = `<rect x="0.5" y="0.5" width="${width - 1}" height="${height - 1}" rx="4" fill="#ffffff" stroke="#cbd5e1"/>`;
  const heading = `<text x="${width / 2}" y="24" text-anchor="middle" font-family="Arial, sans-serif" font-size="14" font-weight="600" fill="#0f172a">${title}</text>`;
  const body = chart.type === 'pie' || chart.type === 'doughnut'
    ? renderPieChart(chart, width, height)
    : renderCartesianChart(chart, width, height);
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}">${frame}${heading}${body}</svg>`;
}

function renderCartesianChart(chart: SheetChart, width: number, height: number): string {
  const left = 42;
  const top = 40;
  const right = 18;
  const bottom = 42;
  const plotWidth = width - left - right;
  const plotHeight = height - top - bottom;
  const categories = chart.series[0]?.categories ?? [];
  const values = chart.series.flatMap((series) => series.values).filter(Number.isFinite);
  const maximum = Math.max(1, ...values);
  const axes = `<path d="M${left} ${top}V${top + plotHeight}H${left + plotWidth}" fill="none" stroke="#94a3b8"/>`;
  const labels = categories.map((category, index) => {
    const x = left + ((index + 0.5) * plotWidth) / Math.max(1, categories.length);
    return `<text x="${x}" y="${top + plotHeight + 18}" text-anchor="middle" font-family="Arial, sans-serif" font-size="10" fill="#475569">${escapeXML(shortLabel(category))}</text>`;
  }).join('');
  if (chart.type === 'bar') {
    const groupWidth = plotWidth / Math.max(1, categories.length);
    const barWidth = Math.max(3, (groupWidth * 0.72) / Math.max(1, chart.series.length));
    const bars = chart.series.flatMap((series, seriesIndex) => series.values.map((value, index) => {
      const barHeight = Math.max(0, (value / maximum) * plotHeight);
      const x = left + index * groupWidth + groupWidth * 0.14 + seriesIndex * barWidth;
      const y = top + plotHeight - barHeight;
      return `<rect x="${x}" y="${y}" width="${barWidth - 1}" height="${barHeight}" fill="${chartColors[seriesIndex % chartColors.length]}"/>`;
    })).join('');
    return axes + bars + labels + renderLegend(chart, width, height);
  }
  const paths = chart.series.map((series, seriesIndex) => {
    const points = series.values.map((value, index) => {
      const x = left + ((index + 0.5) * plotWidth) / Math.max(1, series.values.length);
      const y = top + plotHeight - (value / maximum) * plotHeight;
      return `${x},${y}`;
    });
    const color = chartColors[seriesIndex % chartColors.length];
    const area = chart.type === 'area' && points.length > 0
      ? `<polygon points="${left + plotWidth / Math.max(2, points.length * 2)},${top + plotHeight} ${points.join(' ')} ${left + plotWidth - plotWidth / Math.max(2, points.length * 2)},${top + plotHeight}" fill="${color}" fill-opacity="0.18"/>`
      : '';
    return `${area}<polyline points="${points.join(' ')}" fill="none" stroke="${color}" stroke-width="2"/>`;
  }).join('');
  return axes + paths + labels + renderLegend(chart, width, height);
}

function renderPieChart(chart: SheetChart, width: number, height: number): string {
  const series = chart.series[0];
  if (!series) return '';
  const total = series.values.reduce((sum, value) => sum + Math.max(0, value), 0);
  if (total <= 0) return '';
  const radius = Math.max(30, Math.min(width * 0.24, (height - 58) * 0.45));
  const centerX = width * 0.38;
  const centerY = 40 + (height - 58) / 2;
  let angle = -Math.PI / 2;
  const slices = series.values.map((value, index) => {
    const portion = Math.max(0, value) / total;
    const next = angle + portion * Math.PI * 2;
    const largeArc = next - angle > Math.PI ? 1 : 0;
    const x1 = centerX + radius * Math.cos(angle);
    const y1 = centerY + radius * Math.sin(angle);
    const x2 = centerX + radius * Math.cos(next);
    const y2 = centerY + radius * Math.sin(next);
    const path = `<path d="M${centerX} ${centerY}L${x1} ${y1}A${radius} ${radius} 0 ${largeArc} 1 ${x2} ${y2}Z" fill="${chartColors[index % chartColors.length]}"/>`;
    angle = next;
    return path;
  }).join('');
  const hole = chart.type === 'doughnut'
    ? `<circle cx="${centerX}" cy="${centerY}" r="${radius * 0.52}" fill="#ffffff"/>`
    : '';
  const legend = series.categories.map((category, index) => {
    const y = 52 + index * 18;
    return `<rect x="${width * 0.68}" y="${y - 9}" width="10" height="10" fill="${chartColors[index % chartColors.length]}"/><text x="${width * 0.68 + 15}" y="${y}" font-family="Arial, sans-serif" font-size="10" fill="#334155">${escapeXML(shortLabel(category))}</text>`;
  }).join('');
  return slices + hole + legend;
}

function renderLegend(chart: SheetChart, width: number, height: number): string {
  if (chart.series.length < 2) return '';
  return chart.series.map((series, index) => {
    const x = 46 + index * Math.max(72, (width - 60) / chart.series.length);
    return `<rect x="${x}" y="${height - 14}" width="9" height="9" fill="${chartColors[index % chartColors.length]}"/><text x="${x + 13}" y="${height - 6}" font-family="Arial, sans-serif" font-size="9" fill="#475569">${escapeXML(shortLabel(series.name || `Series ${index + 1}`))}</text>`;
  }).join('');
}

function shortLabel(value: string): string {
  return value.length > 14 ? `${value.slice(0, 13)}…` : value;
}

function escapeXML(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&apos;');
}
