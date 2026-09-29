import type { z } from 'zod';

import type { SelectionChange, WorkbookEvent, WorkbookOperation } from '../contracts';
import {
  applyResponseSchema,
  errorResponseSchema,
  workbookEventSchema,
  workbookRangeSchema,
  workbookSnapshotSchema,
} from './schemas';
import type { EventSubscription, WorkbookClient } from './workbook-client';

async function requestJSON<T>(url: string, schema: z.ZodType<T>, init?: RequestInit): Promise<T> {
  const response = await fetch(url, init);
  const body: unknown = await response.json();
  if (!response.ok) {
    const parsedError = errorResponseSchema.safeParse(body);
    throw new Error(parsedError.success ? parsedError.data.error : response.statusText);
  }
  return schema.parse(body);
}

export class HTTPWorkbookClient implements WorkbookClient {
  async connect(): Promise<void> {}

  async getWorkbook() {
    return requestJSON('/api/workbook', workbookSnapshotSchema);
  }

  async readRange(sheet: string, range: string) {
    const query = new URLSearchParams({ sheet, range });
    return requestJSON(`/api/range?${query.toString()}`, workbookRangeSchema);
  }

  async applyOperations(baseRevision: number, operations: WorkbookOperation[]) {
    return requestJSON('/api/operations', applyResponseSchema, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ baseRevision, operations }),
    });
  }

  async updatePresence(selection: SelectionChange): Promise<void> {
    const response = await fetch('/api/presence', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(selection),
    });
    if (!response.ok) {
      const body: unknown = await response.json();
      const parsedError = errorResponseSchema.safeParse(body);
      throw new Error(parsedError.success ? parsedError.data.error : response.statusText);
    }
  }

  subscribe(onEvent: (event: WorkbookEvent) => void): EventSubscription {
    const source = new EventSource('/api/events');
    let onerror: ((error: unknown) => void) | null = null;
    source.addEventListener('workbook', (message) => {
      try {
        onEvent(workbookEventSchema.parse(JSON.parse(message.data) as unknown));
      } catch (error) {
        onerror?.(error);
      }
    });
    source.onerror = (error) => onerror?.(error);
    return {
      get onerror() { return onerror; },
      set onerror(handler) { onerror = handler; },
      close: () => source.close(),
    };
  }
}
