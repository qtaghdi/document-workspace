import { App, PostMessageTransport } from '@modelcontextprotocol/ext-apps';
import type { z } from 'zod';

import type { HistoryDirection, SelectionChange, WorkbookEvent, WorkbookOperation } from '../contracts';
import {
  applyResponseSchema,
  eventPollResponseSchema,
  presenceResponseSchema,
  workbookRangeSchema,
  workbookSnapshotSchema,
  sheetObjectsSchema,
} from './schemas';
import type { EventSubscription, WorkbookClient } from './workbook-client';

export class MCPWorkbookClient implements WorkbookClient {
  private app: App | undefined;

  async connect(): Promise<void> {
    this.app = new App(
      { name: 'document-workspace', version: '0.1.0' },
      { availableDisplayModes: ['inline', 'fullscreen'] },
      { autoResize: true },
    );
    await this.app.connect(new PostMessageTransport(window.parent, window.parent));
  }

  async getWorkbook() {
    return this.callTool('get_workbook', {}, workbookSnapshotSchema);
  }

  async readRange(sheet: string, range: string) {
    return this.callTool('read_range', { sheet, range }, workbookRangeSchema);
  }

  async readSheetObjects(sheet: string) {
    return this.callTool('get_sheet_objects', { sheet }, sheetObjectsSchema);
  }

  async applyOperations(baseRevision: number, operations: WorkbookOperation[]) {
    return this.callTool('apply_user_operations', { baseRevision, operations }, applyResponseSchema);
  }

  async restoreHistory(baseRevision: number, direction: HistoryDirection) {
    return this.callTool('restore_user_history', { baseRevision, direction }, applyResponseSchema);
  }

  async updatePresence(selection: SelectionChange): Promise<void> {
    await this.callTool('update_user_presence', { ...selection }, presenceResponseSchema);
  }

  subscribe(onEvent: (event: WorkbookEvent) => void): EventSubscription {
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
        const response = await this.callTool('get_events', { afterSequence }, eventPollResponseSchema);
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

  private async callTool<T>(name: string, args: Record<string, unknown>, schema: z.ZodType<T>): Promise<T> {
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
    return schema.parse(result.structuredContent);
  }
}
