import type {
  WorkbookEvent,
  WorkbookOperation,
  WorkbookRange,
  WorkbookSnapshot,
} from './contracts';

interface ApplyResponse {
  workbook: WorkbookSnapshot;
  applied: number;
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
  async getWorkbook(): Promise<WorkbookSnapshot> {
    return requestJSON<WorkbookSnapshot>('/api/workbook');
  }

  async readRange(sheet: string, range: string): Promise<WorkbookRange> {
    const query = new URLSearchParams({ sheet, range });
    return requestJSON<WorkbookRange>(`/api/range?${query.toString()}`);
  }

  async applyOperations(
    baseRevision: number,
    operations: WorkbookOperation[],
  ): Promise<ApplyResponse> {
    return requestJSON<ApplyResponse>('/api/operations', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ baseRevision, operations }),
    });
  }

  subscribe(onEvent: (event: WorkbookEvent) => void): EventSource {
    const source = new EventSource('/api/events');
    source.addEventListener('workbook', (message) => {
      onEvent(JSON.parse(message.data) as WorkbookEvent);
    });
    return source;
  }
}
