import type {
  SelectionChange,
  WorkbookEvent,
  WorkbookOperation,
  WorkbookRange,
  WorkbookSnapshot,
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
  applyOperations(baseRevision: number, operations: WorkbookOperation[]): Promise<ApplyResponse>;
  updatePresence(selection: SelectionChange): Promise<void>;
  subscribe(onEvent: (event: WorkbookEvent) => void): EventSubscription;
}
