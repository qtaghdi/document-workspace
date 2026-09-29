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

export interface CellFormat {
  bold?: boolean;
  italic?: boolean;
  fontFamily?: string;
  fontSize?: number;
  fontColor?: string;
  fillColor?: string;
  numberFormat?: string;
}

export interface WorkbookRange {
  sheet: string;
  ref: string;
  rows: WorkbookCell[][];
  merges?: string[];
}

export interface WorkbookOperation {
  type: 'set_cell' | 'set_formula' | 'paste_range' | 'set_format' | 'merge_cells' | 'unmerge_cells';
  sheet: string;
  cell?: string;
  range?: string;
  value?: unknown;
  formula?: string;
  cells?: CellInput[][];
  format?: CellFormat;
}

export interface CellInput {
  value?: unknown;
  formula?: string;
}

export interface CellChange extends CellInput {
  cell: string;
}

export interface WorkbookEvent {
  sequence: number;
  revision: number;
  actor: 'ai' | 'human' | string;
  type: 'presence.update' | 'cell.typing' | 'cell.commit' | 'range.commit' | 'range.format' | 'range.merge_cells' | 'range.unmerge_cells' | string;
  sheet?: string;
  cell?: string;
  range?: string;
  text?: string;
  state?: 'selecting' | 'editing' | 'idle' | string;
  cells?: CellChange[];
  format?: CellFormat;
}

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
