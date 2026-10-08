const title = process.env.PR_TITLE ?? '';
const body = process.env.PR_BODY ?? '';
const errors = [];

const titlePattern = /^(feat|fix|docs|refactor|test|build|ci|chore|perf|revert|security)(\([a-z0-9][a-z0-9-]*\))?!?: [a-z0-9].+$/;
if (!titlePattern.test(title)) {
  errors.push('PR title must follow Conventional Commits with an allowed type.');
}
if (title.length > 72) {
  errors.push('PR title must be 72 characters or fewer.');
}
if (title.endsWith('.')) {
  errors.push('PR title must not end with a period.');
}
if (title.includes('\u2014') || body.includes('\u2014')) {
  errors.push('PR title and body must not contain em dash characters.');
}

const requiredHeadings = [
  '## Linked issue',
  '## Problem',
  '## Scope',
  '## Out of scope',
  '## Changes',
  '## Verification',
  '## Impact',
  '## Follow-up work',
];
for (const heading of requiredHeadings) {
  if (!body.includes(heading)) {
    errors.push(`PR body is missing required heading: ${heading}`);
  }
}

if (!/\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)\s+#\d+\b/i.test(body)) {
  errors.push('PR body must include a closing issue reference such as Closes #123.');
}

if (errors.length > 0) {
  for (const error of errors) {
    console.error(`::error::${error}`);
  }
  process.exit(1);
}

console.log('Pull request metadata policy passed.');
