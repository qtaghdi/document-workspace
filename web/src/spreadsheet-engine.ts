import type {
  CellEdit,
  WorkbookEvent,
  WorkbookRange,
  WorkbookSnapshot,
} from './contracts';

export interface SpreadsheetEngine {
  initialize(snapshot: WorkbookSnapshot, ranges: WorkbookRange[]): void;
  onCellEdit(listener: (edit: CellEdit) => void): void;
  applyRemoteEvent(event: WorkbookEvent): void;
  dispose(): void;
}
