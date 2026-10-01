import { LocaleType, mergeLocales } from '@univerjs/core';
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
import { UniverSheetsConditionalFormattingPlugin } from '@univerjs/sheets-conditional-formatting';
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
        const range = worksheet?.getActiveRange();
        if (!worksheet || !range) {
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
      const key = `${source.sheet}!${validation.range}!${validation.type}!${validation.formula1 ?? ''}`;
      if (this.appliedValidations.has(key) || validation.type !== 'list') continue;
      const values = parseValidationList(validation.formula1 ?? '');
      if (values.length === 0) continue;
      const rule = this.univerAPI.newDataValidation()
        .requireValueInList(values, false, validation.showDropDown !== false)
        .setAllowBlank(validation.allowBlank ?? false)
        .build();
      worksheet.getRange(validation.range).setDataValidation(rule);
      this.appliedValidations.add(key);
    }
    for (const format of source.conditionalFormatting ?? []) {
      const key = `${source.sheet}!${format.range}!${format.type}!${format.criteria ?? ''}!${format.value ?? ''}`;
      if (this.appliedConditionalFormats.has(key) || format.type !== 'cell') continue;
      const value = Number(format.value);
      if (!Number.isFinite(value)) continue;
      const base = worksheet.newConditionalFormattingRule();
      let builder;
      if (format.criteria === '>') builder = base.whenNumberGreaterThan(value);
      else if (format.criteria === '>=') builder = base.whenNumberGreaterThanOrEqualTo(value);
      else if (format.criteria === '<') builder = base.whenNumberLessThan(value);
      else if (format.criteria === '<=') builder = base.whenNumberLessThanOrEqualTo(value);
      else if (format.criteria === '=') builder = base.whenNumberEqualTo(value);
      else if (format.criteria === '!=') builder = base.whenNumberNotEqualTo(value);
      else continue;
      if (format.style?.fillColor) builder.setBackground(format.style.fillColor);
      builder.setRanges([worksheet.getRange(format.range).getRange()]);
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
          worksheet.insertImages([image]);
        }
      }
    } finally {
      this.applyingRemote = false;
    }
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
