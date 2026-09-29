import { App, PostMessageTransport } from '@modelcontextprotocol/ext-apps';

import type {
  WorkbookEvent,
  WorkbookOperation,
  WorkbookRange,
  WorkbookSnapshot,
  SelectionChange,
} from './contracts';

interface ApplyResponse {
  workbook: WorkbookSnapshot;
  applied: number;
}

interface EventPollResponse {
  events: WorkbookEvent[];
  workbook: WorkbookSnapshot;
}

export interface EventSubscription {
  onerror: ((error: any) => void) | null;
  close(): void;
}

async function requestJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, init);
  const body = (await response.json()) as T | { error: string };
  if (!response.ok) {
    const message = 'error' in (body as { error?: string })
      ? (body as { error: string }).error
      : response.statusText;
    throw new Error(message);
  }
  return body as T;
}

export class WorkbookAPI {
  private app: App | undefined;
  private readonly appMode = window.parent !== window;

  async connect(): Promise<void> {
    if (!this.appMode) {
      return;
    }
    this.app = new App(
      { name: 'xlsx-viewer', version: '0.1.0' },
      { availableDisplayModes: ['inline', 'fullscreen'] },
      { autoResize: true },
    );
    await this.app.connect(new PostMessageTransport(window.parent, window.parent));
  }

  async getWorkbook(): Promise<WorkbookSnapshot> {
    if (this.app) {
      return this.callTool<WorkbookSnapshot>('get_workbook', {});
    }
    return requestJSON<WorkbookSnapshot>('/api/workbook');
  }

  async readRange(sheet: string, range: string): Promise<WorkbookRange> {
    if (this.app) {
      return this.callTool<WorkbookRange>('read_range', { sheet, range });
    }
    const query = new URLSearchParams({ sheet, range });
    return requestJSON<WorkbookRange>(`/api/range?${query.toString()}`);
  }

  async applyOperations(
    baseRevision: number,
    operations: WorkbookOperation[],
  ): Promise<ApplyResponse> {
    if (this.app) {
      return this.callTool<ApplyResponse>('apply_user_operations', { baseRevision, operations });
    }
    return requestJSON<ApplyResponse>('/api/operations', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ baseRevision, operations }),
    });
  }

  async updatePresence(selection: SelectionChange): Promise<void> {
    if (this.app) {
      await this.callTool<{ updated: boolean }>('update_user_presence', { ...selection });
      return;
    }
    const response = await fetch('/api/presence', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(selection),
    });
    if (!response.ok) {
      const body = (await response.json()) as { error?: string };
      throw new Error(body.error ?? response.statusText);
    }
  }

  subscribe(onEvent: (event: WorkbookEvent) => void): EventSubscription {
    if (this.app) {
      return this.pollEvents(onEvent);
    }
    const source = new EventSource('/api/events');
    source.addEventListener('workbook', (message) => {
      onEvent(JSON.parse(message.data) as WorkbookEvent);
    });
    return source;
  }

  private async callTool<T>(name: string, args: Record<string, unknown>): Promise<T> {
    if (!this.app) {
      throw new Error('MCP App bridge is not connected');
    }
    const result = await this.app.callServerTool({ name, arguments: args });
    if (result.isError) {
      const message = result.content
        .filter((item): item is { type: 'text'; text: string } => item.type === 'text')
        .map((item) => item.text)
        .join('\n');
      throw new Error(message || `Tool ${name} failed`);
    }
    if (!result.structuredContent) {
      throw new Error(`Tool ${name} returned no structured content`);
    }
    return result.structuredContent as T;
  }

  private pollEvents(onEvent: (event: WorkbookEvent) => void): EventSubscription {
    let closed = false;
    let timer: number | undefined;
    let afterSequence = 0;
    const subscription: EventSubscription = {
      onerror: null,
      close: () => {
        closed = true;
        window.clearTimeout(timer);
      },
    };
    const poll = async (): Promise<void> => {
      try {
        const response = await this.callTool<EventPollResponse>('get_events', { afterSequence });
        for (const event of response.events) {
          afterSequence = Math.max(afterSequence, event.sequence);
          onEvent(event);
        }
      } catch (error) {
        subscription.onerror?.(error);
      } finally {
        if (!closed) {
          timer = window.setTimeout(() => void poll(), 500);
        }
      }
    };
    void poll();
    return subscription;
  }
}
