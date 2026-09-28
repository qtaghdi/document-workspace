import './styles.css';

import { WorkbookAPI } from './api';
import type { CellEdit, WorkbookEvent } from './contracts';
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
let eventSource: EventSource | undefined;
let engine: SpreadsheetEngine | undefined;

void boot();

async function boot(): Promise<void> {
  try {
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
    eventSource = api.subscribe(handleWorkbookEvent);
    eventSource.onerror = () => showRevision('Reconnecting to live events');
  } catch (error) {
    showError(error);
  }
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
  if (event.revision > revision && event.type === 'cell.commit') {
    revision = event.revision;
  }
  engine?.applyRemoteEvent(event);
  showAIPresence(event);
}

function showAIPresence(event: WorkbookEvent): void {
  window.clearTimeout(presenceTimer);
  presence.classList.remove('hidden');
  presence.classList.add('flex');
  const location = event.sheet && event.cell ? `${event.sheet}!${event.cell}` : 'workbook';
  if (event.type === 'cursor.move') {
    presenceText.textContent = `AI selected ${location}`;
  } else if (event.type === 'cell.typing') {
    presenceText.textContent = `AI is editing ${location}`;
  } else if (event.type === 'cell.commit') {
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
