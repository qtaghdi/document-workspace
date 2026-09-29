import type {
  CellEdit,
  RangeEdit,
  SelectionChange,
  WorkbookEvent,
  WorkbookOperation,
  WorkbookRange,
  WorkbookSnapshot,
} from './contracts';

export interface SpreadsheetEngine {
  initialize(snapshot: WorkbookSnapshot, ranges: WorkbookRange[]): void;
  onCellEdit(listener: (edit: CellEdit) => void): void;
  onRangeEdit(listener: (edit: RangeEdit) => void): void;
  onSelectionChange(listener: (selection: SelectionChange) => void): void;
  onOperation(listener: (operation: WorkbookOperation) => void): void;
  applyRemoteEvent(event: WorkbookEvent): void;
  dispose(): void;
}
