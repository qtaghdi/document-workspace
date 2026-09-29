export interface WorkbookSnapshot {
  id: string;
  name: string;
  sheets: string[];
  revision: number;
}

export interface WorkbookCell {
  address: string;
  value: string;
  formula?: string;
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
  | (RangeOperation & { type: 'merge_cells' | 'unmerge_cells' });

interface EventBase {
  sequence: number;
  revision: number;
  actor: string;
}

export type WorkbookEvent =
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
