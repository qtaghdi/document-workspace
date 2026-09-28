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
}

export interface WorkbookRange {
  sheet: string;
  ref: string;
  rows: WorkbookCell[][];
}

export interface WorkbookOperation {
  type: 'set_cell' | 'set_formula';
  sheet: string;
  cell: string;
  value?: unknown;
  formula?: string;
}

export interface WorkbookEvent {
  sequence: number;
  revision: number;
  actor: 'ai' | 'human' | string;
  type: 'cursor.move' | 'cell.typing' | 'cell.commit' | string;
  sheet?: string;
  cell?: string;
  text?: string;
}

export interface CellEdit {
  sheet: string;
  cell: string;
  value: unknown;
  formula?: string;
}
