import type {
  CellEdit,
  RangeEdit,
  SelectionChange,
  ViewportChange,
  HistoryDirection,
  WorkbookEvent,
  WorkbookOperation,
  WorkbookSnapshot,
} from '../contracts';
import type { SpreadsheetEngine } from '../spreadsheet-engine';
import { toA1Range } from '../spreadsheet/a1';
import { parseCellAddress } from '../spreadsheet/a1';
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
  private snapshot: WorkbookSnapshot | undefined;
  private readonly loadedRanges = new Map<string, LoadedRectangle[]>();
  private readonly pendingRangeLoads = new Set<string>();
  private viewportTimer: number | undefined;
  private pendingViewport: ViewportChange | undefined;

  constructor(
    private readonly client: WorkbookClient,
    private readonly view: AppView,
    private readonly createEngine: SpreadsheetEngineFactory,
  ) {}

  async start(): Promise<void> {
    try {
      await this.client.connect();
      const snapshot = await this.client.getWorkbook();
      this.snapshot = snapshot;
      const initialRanges = planInitialRanges(snapshot);
      const ranges = [];
      for (const ref of initialRanges.refs) {
        ranges.push(await this.client.readRange(ref.sheet, ref.range));
      }
      const objects = [];
      for (const sheet of snapshot.sheets) {
        objects.push(await this.client.readSheetObjects(sheet));
      }
      this.revision = snapshot.revision;
      this.view.setWorkbookName(snapshot.name);
      const warnings = [...(snapshot.warnings ?? [])];
      if (objects.some((sheetObjects) => sheetObjects.truncated)) {
        warnings.push({
          feature: 'sheet-objects',
          message: 'Some images or charts were omitted because preview safety limits were reached.',
        });
      }
      this.view.showWarnings(warnings);
      this.view.setHistoryState(snapshot.canUndo, snapshot.canRedo);
      this.view.onHistoryAction((direction) => this.queueHistoryAction(direction));
      this.view.showRevision(
        initialRanges.truncated ? 'Ready with a bounded initial data window' : 'Ready',
        this.revision,
      );

      this.engine = this.createEngine(this.view.spreadsheet);
      this.engine.initialize(snapshot, ranges, objects);
      this.engine.onCellEdit((edit) => this.queueCellEdit(edit));
      this.engine.onRangeEdit((edit) => this.queueRangeEdit(edit));
      this.engine.onSelectionChange((selection) => this.queuePresence(selection));
      this.engine.onOperation((operation) => this.queueOperation(operation));
      this.engine.onViewportChange((viewport) => this.queueViewportLoad(viewport));
      this.engine.onHistoryAction((direction) => this.queueHistoryAction(direction));
      for (const range of ranges) {
        this.recordLoadedRange(range.sheet, range.ref);
      }
      this.eventSubscription = this.client.subscribe((event) => this.handleWorkbookEvent(event));
      this.eventSubscription.onerror = () => {
        this.view.showRevision('Reconnecting to live events', this.revision);
      };
    } catch (error) {
      this.view.showError(error, this.revision);
    }
  }

  private queueHistoryAction(direction: HistoryDirection): void {
    const available = direction === 'undo' ? this.snapshot?.canUndo : this.snapshot?.canRedo;
    if (!available) {
      this.view.showRevision(direction === 'undo' ? 'Nothing to undo' : 'Nothing to redo', this.revision);
      return;
    }
    this.saveQueue = this.saveQueue.then(async () => {
      this.view.showRevision(direction === 'undo' ? 'Undoing change' : 'Redoing change', this.revision);
      try {
        const response = await this.client.restoreHistory(this.revision, direction);
        this.revision = response.workbook.revision;
        this.snapshot = response.workbook;
        this.view.setHistoryState(response.workbook.canUndo, response.workbook.canRedo);
        window.location.reload();
      } catch (error) {
        this.view.showError(error, this.revision);
      }
    });
  }

  dispose(): void {
    window.clearTimeout(this.presenceTimer);
    window.clearTimeout(this.viewportTimer);
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
    const target = operationTarget(operation);
    this.saveQueue = this.saveQueue.then(async () => {
      this.view.showRevision(`Saving ${operation.sheet}!${target}`, this.revision);
      try {
        const response = await this.client.applyOperations(this.revision, [operation]);
        this.revision = response.workbook.revision;
        this.snapshot = response.workbook;
        this.view.setHistoryState(response.workbook.canUndo, response.workbook.canRedo);
        if (operation.type.startsWith('insert_') || operation.type.startsWith('delete_')) {
          this.loadedRanges.delete(operation.sheet);
        }
        this.view.showRevision('Saved', this.revision);
      } catch (error) {
        this.view.showError(error, this.revision);
        throw error;
      }
    });
    this.saveQueue = this.saveQueue.catch(() => undefined);
  }

  private queueViewportLoad(viewport: ViewportChange): void {
    this.pendingViewport = viewport;
    window.clearTimeout(this.viewportTimer);
    this.viewportTimer = window.setTimeout(() => {
      const next = this.pendingViewport;
      this.pendingViewport = undefined;
      if (next) {
        void this.loadViewport(next);
      }
    }, 80);
  }

  private async loadViewport(viewport: ViewportChange): Promise<void> {
    const snapshot = this.snapshot;
    const dimensions = snapshot?.sheetDimensions.find((candidate) => candidate.name === viewport.sheet);
    if (!snapshot || !dimensions) {
      return;
    }
    const rectangle = planViewportRectangle(viewport, dimensions.rows, dimensions.columns);
    if (!rectangle || this.isRangeLoaded(viewport.sheet, rectangle)) {
      return;
    }
    const ref = toA1Range(rectangle.startRow, rectangle.startColumn, rectangle.endRow, rectangle.endColumn);
    const key = `${viewport.sheet}!${ref}`;
    if (this.pendingRangeLoads.has(key)) {
      return;
    }
    this.pendingRangeLoads.add(key);
    try {
      const range = await this.client.readRange(viewport.sheet, ref);
      this.engine?.applyRange(range);
      this.recordLoadedRange(viewport.sheet, range.ref);
      this.view.showRevision(`Loaded ${viewport.sheet}!${ref}`, this.revision);
    } catch (error) {
      this.view.showError(error, this.revision);
    } finally {
      this.pendingRangeLoads.delete(key);
    }
  }

  private recordLoadedRange(sheet: string, ref: string): void {
    const rectangle = rectangleFromA1(ref);
    const ranges = this.loadedRanges.get(sheet) ?? [];
    ranges.push(rectangle);
    this.loadedRanges.set(sheet, ranges);
  }

  private isRangeLoaded(sheet: string, target: LoadedRectangle): boolean {
    return (this.loadedRanges.get(sheet) ?? []).some((loaded) =>
      loaded.startRow <= target.startRow && loaded.endRow >= target.endRow &&
      loaded.startColumn <= target.startColumn && loaded.endColumn >= target.endColumn,
    );
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
    if (event.type === 'workbook.reload') {
      window.location.reload();
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

interface LoadedRectangle {
  startRow: number;
  startColumn: number;
  endRow: number;
  endColumn: number;
}

function operationTarget(operation: WorkbookOperation): string {
  if ('range' in operation) {
    return operation.range;
  }
  if ('cell' in operation) {
    return operation.cell;
  }
  return `${operation.index}:${operation.index + operation.count - 1}`;
}

function rectangleFromA1(ref: string): LoadedRectangle {
  const [start = 'A1', end = start] = ref.split(':');
  const first = parseCellAddress(start);
  const last = parseCellAddress(end);
  return { startRow: first.row, startColumn: first.column, endRow: last.row, endColumn: last.column };
}

function planViewportRectangle(viewport: ViewportChange, rows: number, columns: number): LoadedRectangle | undefined {
  if (rows < 1 || columns < 1) {
    return undefined;
  }
  const tileRows = 200;
  const tileColumns = 50;
  const startRow = Math.floor(Math.max(0, viewport.row) / tileRows) * tileRows;
  const startColumn = Math.floor(Math.max(0, viewport.column) / tileColumns) * tileColumns;
  return {
    startRow,
    startColumn,
    endRow: Math.min(rows - 1, startRow + tileRows - 1),
    endColumn: Math.min(columns - 1, startColumn + tileColumns - 1),
  };
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
