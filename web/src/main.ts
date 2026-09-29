import './styles.css';

import { WorkbookAPI, type EventSubscription } from './api';
import type { CellEdit, RangeEdit, SelectionChange, WorkbookEvent } from './contracts';
import type { SpreadsheetEngine } from './spreadsheet-engine';
import { UniverSpreadsheetEngine } from './univer-engine';

const workbookName = requiredElement('workbook-name');
const revisionStatus = requiredElement('revision-status');
const presence = requiredElement('ai-presence');
const presenceText = requiredElement('ai-presence-text');
const errorPanel = requiredElement('fatal-error');
const container = requiredElement('spreadsheet');

const api = new WorkbookAPI();
let revision = 0;
let saveQueue = Promise.resolve();
let presenceTimer: number | undefined;
let eventSource: EventSubscription | undefined;
let engine: SpreadsheetEngine | undefined;
let presenceTimerRequest: number | undefined;
let pendingSelection: SelectionChange | undefined;

void boot();

async function boot(): Promise<void> {
  try {
    await api.connect();
    const snapshot = await api.getWorkbook();
    const ranges = await Promise.all(
      snapshot.sheets.map((sheet) => api.readRange(sheet, 'A1:AX200')),
    );
    revision = snapshot.revision;
    workbookName.textContent = snapshot.name;
    showRevision('Ready');

    engine = new UniverSpreadsheetEngine(container);
    engine.initialize(snapshot, ranges);
    engine.onCellEdit(queueHumanEdit);
    engine.onRangeEdit(queueHumanRangeEdit);
    engine.onSelectionChange(queueHumanPresence);
    engine.onOperation(queueHumanOperation);
    const subscription = api.subscribe(handleWorkbookEvent);
    subscription.onerror = () => showRevision('Reconnecting to live events');
    eventSource = subscription;
  } catch (error) {
    showError(error);
  }
}

function queueHumanOperation(operation: import('./contracts').WorkbookOperation): void {
  const target = operation.range ?? operation.cell ?? 'workbook';
  saveQueue = saveQueue.then(async () => {
    showRevision(`Saving ${operation.sheet}!${target}`);
    try {
      const response = await api.applyOperations(revision, [operation]);
      revision = response.workbook.revision;
      showRevision('Saved');
    } catch (error) {
      showError(error);
      throw error;
    }
  });
  saveQueue = saveQueue.catch(() => undefined);
}

function queueHumanRangeEdit(edit: RangeEdit): void {
  saveQueue = saveQueue.then(async () => {
    showRevision(`Saving ${edit.sheet}!${edit.range}`);
    try {
      const response = await api.applyOperations(revision, [{
        type: 'paste_range',
        sheet: edit.sheet,
        range: edit.range,
        cells: edit.cells,
      }]);
      revision = response.workbook.revision;
      showRevision('Saved');
    } catch (error) {
      showError(error);
      throw error;
    }
  });
  saveQueue = saveQueue.catch(() => undefined);
}

function queueHumanPresence(selection: SelectionChange): void {
  pendingSelection = selection;
  window.clearTimeout(presenceTimerRequest);
  presenceTimerRequest = window.setTimeout(() => {
    const next = pendingSelection;
    pendingSelection = undefined;
    if (next) {
      void api.updatePresence(next).catch(showError);
    }
  }, 80);
}

function queueHumanEdit(edit: CellEdit): void {
  saveQueue = saveQueue.then(async () => {
    showRevision(`Saving ${edit.sheet}!${edit.cell}`);
    const operation = edit.formula
      ? {
          type: 'set_formula' as const,
          sheet: edit.sheet,
          cell: edit.cell,
          formula: edit.formula,
        }
      : {
          type: 'set_cell' as const,
          sheet: edit.sheet,
          cell: edit.cell,
          value: edit.value,
        };
    try {
      const response = await api.applyOperations(revision, [operation]);
      revision = response.workbook.revision;
      showRevision('Saved');
    } catch (error) {
      showError(error);
      throw error;
    }
  });
  saveQueue = saveQueue.catch(() => undefined);
}

function handleWorkbookEvent(event: WorkbookEvent): void {
  if (event.actor !== 'ai') {
    return;
  }
  if (event.revision > revision && (event.type === 'cell.commit' || event.type === 'range.commit')) {
    revision = event.revision;
  }
  engine?.applyRemoteEvent(event);
  showAIPresence(event);
}

function showAIPresence(event: WorkbookEvent): void {
  window.clearTimeout(presenceTimer);
  presence.classList.remove('hidden');
  presence.classList.add('flex');
  const selection = event.cell ?? event.range;
  const location = event.sheet && selection ? `${event.sheet}!${selection}` : 'workbook';
  if (event.type === 'presence.update') {
    presenceText.textContent = `AI selected ${location}`;
  } else if (event.type === 'cell.typing') {
    presenceText.textContent = `AI is editing ${location}`;
  } else if (event.type === 'cell.commit' || event.type === 'range.commit') {
    presenceText.textContent = `AI committed ${location}`;
    showRevision('AI change saved');
    presenceTimer = window.setTimeout(() => {
      presence.classList.add('hidden');
      presence.classList.remove('flex');
    }, 4000);
  }
}

function showRevision(message: string): void {
  revisionStatus.textContent = revision > 0 ? `${message} · revision ${revision}` : message;
}

function showError(error: unknown): void {
  const message = error instanceof Error ? error.message : String(error);
  errorPanel.textContent = message;
  errorPanel.classList.remove('hidden');
  showRevision('Save failed');
}

function requiredElement(id: string): HTMLElement {
  const element = document.getElementById(id);
  if (!element) {
    throw new Error(`Missing required element #${id}`);
  }
  return element;
}

window.addEventListener('beforeunload', () => {
  eventSource?.close();
  engine?.dispose();
});
