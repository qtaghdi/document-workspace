import type { FeatureWarning, HistoryDirection, WorkbookEvent } from '../contracts';

export class AppView {
  readonly spreadsheet: HTMLElement;
  private readonly workbookName: HTMLElement;
  private readonly revisionStatus: HTMLElement;
  private readonly presence: HTMLElement;
  private readonly presenceText: HTMLElement;
  private readonly errorPanel: HTMLElement;
  private readonly warningPanel: HTMLElement;
  private readonly undoButton: HTMLButtonElement;
  private readonly redoButton: HTMLButtonElement;
  private presenceTimer: number | undefined;
  private historyListener: ((direction: HistoryDirection) => void) | undefined;

  constructor(document: Document) {
    this.workbookName = requiredElement(document, 'workbook-name');
    this.revisionStatus = requiredElement(document, 'revision-status');
    this.presence = requiredElement(document, 'ai-presence');
    this.presenceText = requiredElement(document, 'ai-presence-text');
    this.errorPanel = requiredElement(document, 'fatal-error');
    this.warningPanel = requiredElement(document, 'feature-warnings');
    this.undoButton = requiredButton(document, 'history-undo');
    this.redoButton = requiredButton(document, 'history-redo');
    this.undoButton.addEventListener('click', this.handleUndo);
    this.redoButton.addEventListener('click', this.handleRedo);
    this.spreadsheet = requiredElement(document, 'spreadsheet');
  }

  onHistoryAction(listener: (direction: HistoryDirection) => void): void {
    this.historyListener = listener;
  }

  setHistoryState(canUndo: boolean, canRedo: boolean): void {
    this.undoButton.disabled = !canUndo;
    this.redoButton.disabled = !canRedo;
  }

  showWarnings(warnings: FeatureWarning[]): void {
    if (warnings.length === 0) {
      this.warningPanel.classList.add('hidden');
      return;
    }
    this.warningPanel.textContent = `Compatibility notice: ${warnings.map((warning) => warning.message).join('; ')}`;
    this.warningPanel.classList.remove('hidden');
  }

  setWorkbookName(name: string): void {
    this.workbookName.textContent = name;
  }

  showRevision(message: string, revision: number): void {
    this.revisionStatus.textContent = revision > 0 ? `${message} · revision ${revision}` : message;
  }

  showAIPresence(event: WorkbookEvent, revision: number): void {
    if (event.type === 'workbook.reload') {
      this.showRevision(`AI ${event.state} applied`, revision);
      return;
    }
    window.clearTimeout(this.presenceTimer);
    this.presence.classList.remove('hidden');
    this.presence.classList.add('flex');
    const selection = event.type === 'cell.typing' || event.type === 'cell.commit'
      ? event.cell
      : 'range' in event
        ? event.range
        : `${event.index}:${event.index + event.count - 1}`;
    const location = `${event.sheet}!${selection}`;
    if (event.type === 'presence.update') {
      this.presenceText.textContent = `AI selected ${location}`;
    } else if (event.type === 'cell.typing') {
      this.presenceText.textContent = `AI is editing ${location}`;
    } else {
      const action = event.type === 'cell.commit' || event.type === 'range.commit'
        ? 'committed'
        : 'updated';
      this.presenceText.textContent = `AI ${action} ${location}`;
      this.showRevision('AI change saved', revision);
      this.presenceTimer = window.setTimeout(() => {
        this.presence.classList.add('hidden');
        this.presence.classList.remove('flex');
      }, 4000);
    }
  }

  showError(error: unknown, revision: number): void {
    const message = error instanceof Error ? error.message : String(error);
    this.errorPanel.textContent = message;
    this.errorPanel.classList.remove('hidden');
    this.showRevision('Save failed', revision);
  }

  showRecovery(message: string, revision: number): void {
    this.errorPanel.textContent = `${message} Editing is paused. Copy any unsaved drafts before reloading. Do not repeat an uncertain write until you have checked the saved workbook. `;
    const reload = this.errorPanel.ownerDocument.createElement('button');
    reload.type = 'button';
    reload.className = 'rounded border px-3 py-1 font-semibold underline';
    reload.textContent = 'Reload saved workbook';
    reload.onclick = () => window.location.reload();
    this.errorPanel.append(reload);
    this.errorPanel.classList.remove('hidden');
    this.setHistoryState(false, false);
    this.showRevision('Recovery required', revision);
  }

  dispose(): void {
    window.clearTimeout(this.presenceTimer);
    this.undoButton.removeEventListener('click', this.handleUndo);
    this.redoButton.removeEventListener('click', this.handleRedo);
  }

  private readonly handleUndo = (): void => this.historyListener?.('undo');
  private readonly handleRedo = (): void => this.historyListener?.('redo');
}

function requiredElement(document: Document, id: string): HTMLElement {
  const element = document.getElementById(id);
  if (!element) {
    throw new Error(`Missing required element #${id}`);
  }
  return element;
}

function requiredButton(document: Document, id: string): HTMLButtonElement {
  const element = document.getElementById(id);
  if (!(element instanceof HTMLButtonElement)) {
    throw new Error(`Missing button #${id}`);
  }
  return element;
}
