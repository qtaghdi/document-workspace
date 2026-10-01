import type {
  CellEdit,
  RangeEdit,
  SelectionChange,
  WorkbookEvent,
  WorkbookOperation,
  WorkbookSnapshot,
} from '../contracts';
import type { SpreadsheetEngine } from '../spreadsheet-engine';
import { toA1Range } from '../spreadsheet/a1';
import type { EventSubscription, WorkbookClient } from '../transport/workbook-client';
import type { AppView } from '../ui/app-view';

export type SpreadsheetEngineFactory = (container: HTMLElement) => SpreadsheetEngine;

const maxRangeCells = 10_000;
const maxInitialCellsPerSheet = 100_000;
const maxInitialCellsPerWorkbook = 500_000;

export class WorkbookController {
  private revision = 0;
  private saveQueue = Promise.resolve();
  private presenceTimer: number | undefined;
  private pendingSelection: SelectionChange | undefined;
  private eventSubscription: EventSubscription | undefined;
  private engine: SpreadsheetEngine | undefined;

  constructor(
    private readonly client: WorkbookClient,
    private readonly view: AppView,
    private readonly createEngine: SpreadsheetEngineFactory,
  ) {}

  async start(): Promise<void> {
    try {
      await this.client.connect();
      const snapshot = await this.client.getWorkbook();
      const initialRanges = planInitialRanges(snapshot);
      const ranges = [];
      for (const ref of initialRanges.refs) {
        ranges.push(await this.client.readRange(ref.sheet, ref.range));
      }
      this.revision = snapshot.revision;
      this.view.setWorkbookName(snapshot.name);
      this.view.showRevision(
        initialRanges.truncated ? 'Ready with a bounded initial data window' : 'Ready',
        this.revision,
      );

      this.engine = this.createEngine(this.view.spreadsheet);
      this.engine.initialize(snapshot, ranges);
      this.engine.onCellEdit((edit) => this.queueCellEdit(edit));
      this.engine.onRangeEdit((edit) => this.queueRangeEdit(edit));
      this.engine.onSelectionChange((selection) => this.queuePresence(selection));
      this.engine.onOperation((operation) => this.queueOperation(operation));
      this.eventSubscription = this.client.subscribe((event) => this.handleWorkbookEvent(event));
      this.eventSubscription.onerror = () => {
        this.view.showRevision('Reconnecting to live events', this.revision);
      };
    } catch (error) {
      this.view.showError(error, this.revision);
    }
  }

  dispose(): void {
    window.clearTimeout(this.presenceTimer);
    this.eventSubscription?.close();
    this.engine?.dispose();
    this.view.dispose();
  }

  private queueCellEdit(edit: CellEdit): void {
    const operation: WorkbookOperation = edit.formula
      ? { type: 'set_formula', sheet: edit.sheet, cell: edit.cell, formula: edit.formula }
      : { type: 'set_cell', sheet: edit.sheet, cell: edit.cell, value: edit.value };
    this.queueOperation(operation);
  }

  private queueRangeEdit(edit: RangeEdit): void {
    this.queueOperation({
      type: 'paste_range',
      sheet: edit.sheet,
      range: edit.range,
      cells: edit.cells,
    });
  }

  private queueOperation(operation: WorkbookOperation): void {
    const target = 'range' in operation ? operation.range : operation.cell;
    this.saveQueue = this.saveQueue.then(async () => {
      this.view.showRevision(`Saving ${operation.sheet}!${target}`, this.revision);
      try {
        const response = await this.client.applyOperations(this.revision, [operation]);
        this.revision = response.workbook.revision;
        this.view.showRevision('Saved', this.revision);
      } catch (error) {
        this.view.showError(error, this.revision);
        throw error;
      }
    });
    this.saveQueue = this.saveQueue.catch(() => undefined);
  }

  private queuePresence(selection: SelectionChange): void {
    this.pendingSelection = selection;
    window.clearTimeout(this.presenceTimer);
    this.presenceTimer = window.setTimeout(() => {
      const next = this.pendingSelection;
      this.pendingSelection = undefined;
      if (next) {
        void this.client.updatePresence(next).catch((error) => {
          this.view.showError(error, this.revision);
        });
      }
    }, 80);
  }

  private handleWorkbookEvent(event: WorkbookEvent): void {
    if (event.actor !== 'ai') {
      return;
    }
    const commitsWorkbook = event.type !== 'presence.update' && event.type !== 'cell.typing';
    if (event.revision > this.revision && commitsWorkbook) {
      this.revision = event.revision;
    }
    this.engine?.applyRemoteEvent(event);
    this.view.showAIPresence(event, this.revision);
  }
}

export function planInitialRanges(snapshot: WorkbookSnapshot): {
  refs: Array<{ sheet: string; range: string }>;
  truncated: boolean;
} {
  const refs: Array<{ sheet: string; range: string }> = [];
  let workbookRemaining = maxInitialCellsPerWorkbook;
  let truncated = false;

  for (const sheet of snapshot.sheets) {
    const dimensions = snapshot.sheetDimensions.find((candidate) => candidate.name === sheet);
    const rows = Math.max(1, dimensions?.rows ?? 200);
    const columns = Math.max(1, dimensions?.columns ?? 50);
    let sheetRemaining = Math.min(maxInitialCellsPerSheet, workbookRemaining);
    let loadedCells = 0;
    let loadedForSheet = false;

    for (let startColumn = 0; startColumn < columns && sheetRemaining > 0; startColumn += maxRangeCells) {
      const width = Math.min(columns - startColumn, maxRangeCells);
      const rowsPerRequest = Math.max(1, Math.floor(maxRangeCells / width));
      for (let startRow = 0; startRow < rows && sheetRemaining > 0; startRow += rowsPerRequest) {
        const height = Math.min(rows - startRow, rowsPerRequest, Math.floor(sheetRemaining / width));
        if (height < 1) {
          break;
        }
        refs.push({
          sheet,
          range: toA1Range(
            startRow,
            startColumn,
            startRow + height - 1,
            startColumn + width - 1,
          ),
        });
        const loaded = height * width;
        loadedCells += loaded;
        sheetRemaining -= loaded;
        workbookRemaining -= loaded;
        loadedForSheet = true;
      }
    }
    if (!loadedForSheet) {
      refs.push({ sheet, range: 'A1' });
    }
    if (loadedCells < rows * columns) {
      truncated = true;
    }
  }

  return { refs, truncated };
}
