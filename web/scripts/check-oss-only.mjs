import { readFile } from 'node:fs/promises';

const lockURL = new URL('../package-lock.json', import.meta.url);
const lock = JSON.parse(await readFile(lockURL, 'utf8'));
const packagePaths = Object.keys(lock.packages ?? {});
const prohibited = packagePaths.filter((path) =>
  path.includes('node_modules/@univerjs-pro/'),
);

if (prohibited.length > 0) {
  console.error('Univer Pro packages are not allowed in this repository:');
  for (const path of prohibited) {
    console.error(`- ${path}`);
  }
  process.exit(1);
}

console.log('OSS dependency check passed');
