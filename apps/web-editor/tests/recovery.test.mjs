import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import vm from 'node:vm';
import test from 'node:test';
import ts from 'typescript';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const nativeRequire = createRequire(import.meta.url);
// Compile the real controller in an isolated browser environment without adding
// a browser framework or changing production module resolution for tests.
const loadModule = (path, globals) => {
  const module = { exports: {} };
  const code = ts.transpileModule(readFileSync(path, 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  vm.runInNewContext(code, {
    ...globals, module, exports: module.exports,
    require: (name) => name.startsWith('.')
      ? loadModule(resolve(dirname(path), `${name}.ts`), globals)
      : nativeRequire(name),
  }, { filename: path });
  return module.exports;
};

const fixture = async (overrides = {}) => {
  const state = { writes: 0, reloads: 0, recoveries: [], statuses: [], callbacks: {}, subscriptionClosed: false };
  const snapshot = { id: 'session', sequence: 5, name: 'book.xlsx', sheets: ['Sheet1'],
    sheetDimensions: [{ name: 'Sheet1', rows: 1, columns: 1 }], canUndo: true, canRedo: false, revision: 2 };
  const window = { clearTimeout() {}, setTimeout: () => 1, location: { reload: () => state.reloads++ } };
  const { WorkbookController } = loadModule(resolve(root, 'src/app/workbook-controller.ts'), { window });
  const client = {
    connect: async () => {}, getWorkbook: async () => snapshot,
    readRange: async () => ({ sheet: 'Sheet1', ref: 'A1', rows: [] }),
    readSheetObjects: async () => ({ sheet: 'Sheet1' }),
    applyOperations: async () => { state.writes++; return { workbook: { ...snapshot, revision: 3 }, applied: 1 }; },
    restoreHistory: async () => { state.writes++; return { workbook: { ...snapshot, revision: 3 }, applied: 1 }; },
    subscribe: (handler, checkpoint) => {
      state.onEvent = handler; state.checkpoint = checkpoint;
      state.subscription = { onerror: null, close: () => { state.subscriptionClosed = true; } };
      return state.subscription;
    },
    ...overrides,
  };
  const view = { spreadsheet: {}, setWorkbookName() {}, showWarnings() {}, setHistoryState() {},
    onHistoryAction: (handler) => { state.history = handler; },
    showRevision: (message) => state.statuses.push(message),
    showRecovery: (message) => state.recoveries.push(message), showError: (error) => { throw error; },
    showAIPresence() {}, dispose() {},
  };
  const engine = { initialize() {}, dispose() {}, applyRemoteEvent() {} };
  for (const method of ['onCellEdit','onRangeEdit','onSelectionChange','onOperation','onViewportChange','onHistoryAction']) {
    engine[method] = (callback) => { state.callbacks[method] = callback; };
  }
  const controller = new WorkbookController(client, view, () => engine);
  await controller.start();
  return { state, controller, client, snapshot };
};
const edit = { type: 'set_cell', sheet: 'Sheet1', cell: 'A1', value: 'draft' };
const flush = () => new Promise((resolve) => setImmediate(resolve));

test('lost response blocks queued edits and reconciles without retry or reload', async () => {
  const { state, client } = await fixture();
  client.applyOperations = async () => { state.writes++; throw new Error('response lost'); };
  state.callbacks.onOperation(edit);
  state.callbacks.onOperation(edit);
  await flush();
  assert.equal(state.writes, 1);
  assert.equal(state.reloads, 0);
  assert.ok(state.recoveries.length > 0);
  assert.equal(state.subscriptionClosed, true);
  state.callbacks.onOperation(edit);
  state.history('undo');
  await flush();
  assert.equal(state.writes, 1);
  assert.ok(!state.statuses.includes('Saved'));
});

test('event loss pauses editing even when the status read is unavailable', async () => {
  const { state, client } = await fixture();
  client.getWorkbook = async () => { throw new Error('offline'); };
  state.subscription.onerror(new Error('disconnected'));
  state.callbacks.onOperation(edit);
  await flush();
  assert.equal(state.writes, 0);
  assert.match(state.recoveries.at(-1), /could not be checked/);
  assert.equal(state.reloads, 0);
});

test('subscription starts at the snapshot checkpoint and resync is not ignored', async () => {
  const { state } = await fixture();
  assert.equal(state.checkpoint.sequence, 5);
  state.onEvent({ type: 'workbook.reload', state: 'resync', actor: 'system', sequence: 600, revision: 9 });
  state.callbacks.onOperation(edit);
  await flush();
  assert.equal(state.writes, 0);
  assert.match(state.recoveries[0], /history is incomplete/);
});

test('a successful queued write still commits normally', async () => {
  const { state } = await fixture();
  state.callbacks.onOperation(edit);
  await flush();
  assert.equal(state.writes, 1);
  assert.ok(state.statuses.includes('Saved'));
  assert.equal(state.recoveries.length, 0);
});

test('remote commit during an unresolved local save cannot rebase queued drafts', async () => {
  const { state, client, snapshot } = await fixture();
  let finish;
  client.applyOperations = () => { state.writes++; return new Promise((resolve) => { finish = resolve; }); };
  state.callbacks.onOperation(edit);
  state.callbacks.onOperation(edit);
  await flush();
  state.onEvent({ type: 'cell.commit', actor: 'ai', revision: 3, sequence: 6, sheet: 'Sheet1', cell: 'A1' });
  finish({ workbook: { ...snapshot, revision: 4 }, applied: 1 });
  await flush();
  assert.equal(state.writes, 1);
  assert.equal(state.reloads, 0);
  assert.ok(!state.statuses.includes('Saved'));
});

test('failed undo does not allow a queued edit to continue', async () => {
  const { state, client } = await fixture();
  client.restoreHistory = async () => { state.writes++; throw new Error('unknown undo outcome'); };
  state.history('undo');
  state.callbacks.onOperation(edit);
  await flush();
  assert.equal(state.writes, 1);
  assert.equal(state.reloads, 0);
});

test('successful undo cancels queued edits based on the previous grid', async () => {
  const { state } = await fixture();
  state.history('undo');
  state.callbacks.onOperation(edit);
  await flush();
  assert.equal(state.writes, 1);
  assert.equal(state.reloads, 1);
});
