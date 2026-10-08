import { readFile } from 'node:fs/promises';

const lockURL = new URL('../../../pnpm-lock.yaml', import.meta.url);
const lock = await readFile(lockURL, 'utf8');
const prohibited = lock.match(/@univerjs-pro\/[a-z0-9_-]+/gi) ?? [];

if (prohibited.length > 0) {
  console.error('Univer Pro packages are not allowed in this repository:');
  for (const path of prohibited) {
    console.error(`- ${path}`);
  }
  process.exit(1);
}

console.log('OSS dependency check passed');
