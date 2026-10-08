import type { CellFormat } from '../../contracts';

interface FormattableRange {
  getCellStyle(): {
    bold: boolean;
    italic: boolean;
    fontFamily?: string | null | void;
    fontSize?: number;
    color?: { rgb?: string | null | void } | null | void;
    background?: { rgb?: string | null | void } | null | void;
    numberFormat?: { pattern: string } | null | void;
  } | null;
}

export function formatAfterCommand(
  commandID: string,
  range: FormattableRange,
): CellFormat | undefined {
  const style = range.getCellStyle();
  if (!style) return undefined;
  switch (commandID) {
    case 'sheet.command.set-bold': return { bold: style.bold };
    case 'sheet.command.set-italic': return { italic: style.italic };
    case 'sheet.command.set-font-family': return { fontFamily: style.fontFamily ?? '' };
    case 'sheet.command.set-font-size': return { fontSize: style.fontSize ?? 11 };
    case 'sheet.command.set-text-color':
    case 'sheet.command.reset-text-color': return { fontColor: style.color?.rgb ?? '' };
    case 'sheet.command.set-background-color':
    case 'sheet.command.reset-background-color': return { fillColor: style.background?.rgb ?? '' };
    default:
      if (commandID.startsWith('sheet.command.numfmt.')) {
        return { numberFormat: style.numberFormat?.pattern ?? '' };
      }
      return undefined;
  }
}
