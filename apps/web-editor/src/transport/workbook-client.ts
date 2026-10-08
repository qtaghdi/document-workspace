import type {
  SelectionChange,
  WorkbookEvent,
  WorkbookOperation,
  WorkbookRange,
  WorkbookSnapshot,
  HistoryDirection,
  SheetObjects,
} from '../contracts';

export interface ApplyResponse {
  workbook: WorkbookSnapshot;
  applied: number;
}

export interface EventSubscription {
  onerror: ((error: unknown) => void) | null;
  close(): void;
}

export interface WorkbookClient {
  connect(): Promise<void>;
  getWorkbook(): Promise<WorkbookSnapshot>;
  readRange(sheet: string, range: string): Promise<WorkbookRange>;
  readSheetObjects(sheet: string): Promise<SheetObjects>;
  applyOperations(baseRevision: number, operations: WorkbookOperation[]): Promise<ApplyResponse>;
  restoreHistory(baseRevision: number, direction: HistoryDirection): Promise<ApplyResponse>;
  updatePresence(selection: SelectionChange): Promise<void>;
  subscribe(onEvent: (event: WorkbookEvent) => void, checkpoint: WorkbookSnapshot): EventSubscription;
}
