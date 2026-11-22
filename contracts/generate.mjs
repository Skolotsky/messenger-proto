// Writes generated/types.ts from schema/contexia.json. With --check, fails if the file is out of date.
import { readFileSync, writeFileSync } from 'node:fs';
import { compile } from 'json-schema-to-typescript';

const schema = JSON.parse(readFileSync(new URL('schema/contexia.json', import.meta.url)));
const names = Object.keys(schema.$defs);
const root = { ...schema, title: 'ContractTypes', type: 'object', additionalProperties: false,
  properties: Object.fromEntries(names.map((n) => [n, { $ref: `#/$defs/${n}` }])) };
// allOf holds conditions a TypeScript type cannot express; left in, it erases the fields.
for (const [n, d] of Object.entries(root.$defs)) { d.title = n; delete d.allOf; }
const out = await compile(root, 'ContractTypes', {
  bannerComment: '// Generated from contracts/schema/contexia.json. Do not edit: run ./scripts/contracts',
  additionalProperties: false, unreachableDefinitions: true, format: true,
});
const target = new URL('generated/types.ts', import.meta.url);
if (process.argv.includes('--check')) {
  if (readFileSync(target, 'utf8') !== out) { console.error('out of date'); process.exit(1); }
} else writeFileSync(target, out);
