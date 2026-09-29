import type { WorkbookEvent } from '../contracts';

export class AppView {
  readonly spreadsheet: HTMLElement;
  private readonly workbookName: HTMLElement;
  private readonly revisionStatus: HTMLElement;
  private readonly presence: HTMLElement;
  private readonly presenceText: HTMLElement;
  private readonly errorPanel: HTMLElement;
  private presenceTimer: number | undefined;

  constructor(document: Document) {
    this.workbookName = requiredElement(document, 'workbook-name');
    this.revisionStatus = requiredElement(document, 'revision-status');
    this.presence = requiredElement(document, 'ai-presence');
    this.presenceText = requiredElement(document, 'ai-presence-text');
    this.errorPanel = requiredElement(document, 'fatal-error');
    this.spreadsheet = requiredElement(document, 'spreadsheet');
  }

  setWorkbookName(name: string): void {
    this.workbookName.textContent = name;
  }

  showRevision(message: string, revision: number): void {
    this.revisionStatus.textContent = revision > 0 ? `${message} · revision ${revision}` : message;
  }

  showAIPresence(event: WorkbookEvent, revision: number): void {
    window.clearTimeout(this.presenceTimer);
    this.presence.classList.remove('hidden');
    this.presence.classList.add('flex');
    const selection = event.type === 'cell.typing' || event.type === 'cell.commit'
      ? event.cell
      : event.range;
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

  dispose(): void {
    window.clearTimeout(this.presenceTimer);
  }
}

function requiredElement(document: Document, id: string): HTMLElement {
  const element = document.getElementById(id);
  if (!element) {
    throw new Error(`Missing required element #${id}`);
  }
  return element;
}
