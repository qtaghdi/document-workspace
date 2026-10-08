export interface WorkbookSnapshot {
  sequence: number;
  id: string;
  name: string;
  sheets: string[];
  sheetDimensions: SheetDimensions[];
  warnings?: FeatureWarning[];
  canUndo: boolean;
  canRedo: boolean;
  revision: number;
  formulaPolicy: FormulaPolicy;
}

export interface FormulaPolicy {
  storage: 'preserved';
  serverCalculation: 'none';
  browserCalculation: 'preview';
  nativeRecalculation: 'requested_after_formula_affecting_edits';
}

export interface FeatureWarning {
  feature: string;
  message: string;
}

export interface SheetDimensions {
  name: string;
  rows: number;
  columns: number;
}

export interface WorkbookCell {
  address: string;
  value: string;
  formula?: string;
  formulaValueStatus?: 'cached' | 'unavailable';
  style?: CellStyle;
}

export interface CellStyle {
  bold?: boolean;
  italic?: boolean;
  fontFamily?: string;
  fontSize?: number;
  fontColor?: string;
  fillColor?: string;
  numberFormat?: string;
}

export type CellFormat = CellStyle;

export interface WorkbookRange {
  sheet: string;
  ref: string;
  rows: WorkbookCell[][];
  merges?: string[];
  validations?: DataValidation[];
  conditionalFormatting?: ConditionalFormat[];
}

export interface DataValidation {
  range: string;
  type: string;
  operator?: string;
  formula1?: string;
  formula2?: string;
  allowBlank?: boolean;
  showDropDown?: boolean;
  showErrorMessage?: boolean;
  error?: string;
  errorTitle?: string;
  showInputMessage?: boolean;
  prompt?: string;
  promptTitle?: string;
  date1904?: boolean;
}

export interface ConditionalFormat {
  range: string;
  type: string;
  criteria?: string;
  value?: string;
  minType?: string;
  midType?: string;
  maxType?: string;
  minValue?: string;
  midValue?: string;
  maxValue?: string;
  minColor?: string;
  midColor?: string;
  maxColor?: string;
  barColor?: string;
  barOnly?: boolean;
  barSolid?: boolean;
  aboveAverage?: boolean;
  percent?: boolean;
  style?: CellStyle;
}

export interface SheetObjects {
  sheet: string;
  images?: SheetImage[];
  charts?: SheetChart[];
  truncated?: boolean;
}

export interface SheetImage {
  id: string;
  name?: string;
  altText?: string;
  mimeType: string;
  data: string;
  row: number;
  column: number;
  offsetX?: number;
  offsetY?: number;
  width: number;
  height: number;
}

export interface SheetChart {
  id: string;
  title?: string;
  type: 'bar' | 'line' | 'pie' | 'doughnut' | 'area';
  row: number;
  column: number;
  offsetX?: number;
  offsetY?: number;
  width: number;
  height: number;
  series: ChartSeries[];
}

export interface ChartSeries {
  name?: string;
  categories: string[];
  values: number[];
}

interface CellOperation {
  sheet: string;
  cell: string;
}

export interface CellInput {
  value?: unknown;
  formula?: string;
}

export interface CellChange extends CellInput {
  cell: string;
}

interface RangeOperation {
  sheet: string;
  range: string;
}

export type WorkbookOperation =
  | (CellOperation & { type: 'set_cell'; value: unknown })
  | (CellOperation & { type: 'set_formula'; formula: string })
  | (RangeOperation & { type: 'paste_range'; cells: CellInput[][] })
  | (RangeOperation & { type: 'set_format'; format: CellFormat })
  | (RangeOperation & { type: 'merge_cells' | 'unmerge_cells' })
  | { type: 'insert_rows' | 'delete_rows' | 'insert_columns' | 'delete_columns'; sheet: string; index: number; count: number }
  | {
      type: 'set_image' | 'set_chart';
      sheet: string;
      objectId: string;
      cell: string;
      targetCell: string;
      offsetX: number;
      offsetY: number;
      width: number;
      height: number;
      title?: string;
    }
  | { type: 'delete_image' | 'delete_chart'; sheet: string; objectId: string; cell: string };

interface EventBase {
  sequence: number;
  revision: number;
  actor: string;
}

export type WorkbookEvent =
  | (EventBase & {
      type: 'workbook.reload';
      state: 'undo' | 'redo' | 'objects' | 'resync';
    })
  | (EventBase & {
      type: 'presence.update';
      sheet: string;
      range: string;
      state?: 'selecting' | 'editing' | 'idle';
    })
  | (EventBase & {
      type: 'cell.typing';
      sheet: string;
      cell: string;
      text?: string;
    })
  | (EventBase & {
      type: 'cell.commit';
      sheet: string;
      cell: string;
      text?: string;
      cells?: CellChange[];
    })
  | (EventBase & {
      type: 'range.commit';
      sheet: string;
      range: string;
      cells: CellChange[];
    })
  | (EventBase & {
      type: 'range.format';
      sheet: string;
      range: string;
      format: CellFormat;
    })
  | (EventBase & {
      type: 'range.merge_cells' | 'range.unmerge_cells';
      sheet: string;
      range: string;
    })
  | (EventBase & {
      type: 'sheet.insert_rows' | 'sheet.delete_rows' | 'sheet.insert_columns' | 'sheet.delete_columns';
      sheet: string;
      index: number;
      count: number;
    });

export interface CellEdit {
  sheet: string;
  cell: string;
  value: unknown;
  formula?: string;
}

export interface RangeEdit {
  sheet: string;
  range: string;
  cells: CellInput[][];
}

export interface SelectionChange {
  sheet: string;
  range: string;
  state: 'selecting' | 'editing' | 'idle';
}

export interface ViewportChange {
  sheet: string;
  row: number;
  column: number;
}

export type HistoryDirection = 'undo' | 'redo';
