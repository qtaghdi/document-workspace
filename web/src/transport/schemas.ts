import { z } from 'zod';

const cellStyleSchema = z.object({
  bold: z.boolean().optional(),
  italic: z.boolean().optional(),
  fontFamily: z.string().optional(),
  fontSize: z.number().optional(),
  fontColor: z.string().optional(),
  fillColor: z.string().optional(),
  numberFormat: z.string().optional(),
});

const cellInputSchema = z.object({
  value: z.unknown().optional(),
  formula: z.string().optional(),
});

const cellChangeSchema = cellInputSchema.extend({ cell: z.string() });

export const workbookSnapshotSchema = z.object({
  id: z.string(),
  name: z.string(),
  sheets: z.array(z.string()),
  sheetDimensions: z.array(z.object({
    name: z.string(),
    rows: z.number().int().nonnegative(),
    columns: z.number().int().nonnegative(),
  })),
  warnings: z.array(z.object({
    feature: z.string(),
    message: z.string(),
  })).optional(),
  canUndo: z.boolean(),
  canRedo: z.boolean(),
  revision: z.number().int().nonnegative(),
});

export const workbookRangeSchema = z.object({
  sheet: z.string(),
  ref: z.string(),
  rows: z.array(z.array(z.object({
    address: z.string(),
    value: z.string(),
    formula: z.string().optional(),
    style: cellStyleSchema.optional(),
  }))),
  merges: z.array(z.string()).optional(),
  validations: z.array(z.object({
    range: z.string(), type: z.string(), operator: z.string().optional(),
    formula1: z.string().optional(), formula2: z.string().optional(),
    allowBlank: z.boolean().optional(), showDropDown: z.boolean().optional(),
  })).optional(),
  conditionalFormatting: z.array(z.object({
    range: z.string(), type: z.string(), criteria: z.string().optional(),
    value: z.string().optional(), style: cellStyleSchema.optional(),
  })).optional(),
});

const eventBase = {
  sequence: z.number().int().nonnegative(),
  revision: z.number().int().nonnegative(),
  actor: z.string(),
};

export const workbookEventSchema = z.discriminatedUnion('type', [
  z.object({
    ...eventBase,
    type: z.literal('workbook.reload'),
    state: z.enum(['undo', 'redo']),
  }),
  z.object({
    ...eventBase,
    type: z.literal('presence.update'),
    sheet: z.string(),
    range: z.string(),
    state: z.enum(['selecting', 'editing', 'idle']).optional(),
  }),
  z.object({
    ...eventBase,
    type: z.literal('cell.typing'),
    sheet: z.string(),
    cell: z.string(),
    text: z.string().optional(),
  }),
  z.object({
    ...eventBase,
    type: z.literal('cell.commit'),
    sheet: z.string(),
    cell: z.string(),
    text: z.string().optional(),
    cells: z.array(cellChangeSchema).optional(),
  }),
  z.object({
    ...eventBase,
    type: z.literal('range.commit'),
    sheet: z.string(),
    range: z.string(),
    cells: z.array(cellChangeSchema),
  }),
  z.object({
    ...eventBase,
    type: z.literal('range.format'),
    sheet: z.string(),
    range: z.string(),
    format: cellStyleSchema,
  }),
  z.object({
    ...eventBase,
    type: z.union([z.literal('range.merge_cells'), z.literal('range.unmerge_cells')]),
    sheet: z.string(),
    range: z.string(),
  }),
  z.object({
    ...eventBase,
    type: z.union([
      z.literal('sheet.insert_rows'),
      z.literal('sheet.delete_rows'),
      z.literal('sheet.insert_columns'),
      z.literal('sheet.delete_columns'),
    ]),
    sheet: z.string(),
    index: z.number().int().positive(),
    count: z.number().int().positive(),
  }),
]);

export const applyResponseSchema = z.object({
  workbook: workbookSnapshotSchema,
  applied: z.number().int().nonnegative(),
});

export const eventPollResponseSchema = z.object({
  events: z.array(workbookEventSchema),
  workbook: workbookSnapshotSchema,
});

export const presenceResponseSchema = z.object({ updated: z.boolean() });
export const errorResponseSchema = z.object({ error: z.string() });
