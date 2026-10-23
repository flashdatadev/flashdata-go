// Refresh only response-format operations and their dependencies. Onboarding
// snapshots have their own contract and are deliberately preserved here.
'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const root = path.resolve(__dirname, '..');
const [openapiPath, formatterPath, flag] = process.argv.slice(2);
assert(openapiPath && formatterPath, 'Usage: node scripts/update_response_contract.cjs OPENAPI_JSON RESPONSE_FORMATTING_JS [--check]');
assert(!flag || flag === '--check');
const read = name => JSON.parse(fs.readFileSync(path.join(root, name), 'utf8'));
const snapshot = read('internal/contract/openapi.json');
const source = JSON.parse(fs.readFileSync(openapiPath, 'utf8'));
const operations = [
  ['/v1/queries/realtime', 'post'], ['/v1/queries', 'post'],
  ['/v1/queries/batch', 'post'], ['/jobs/{jobId}', 'get'],
  ['/jobs/{jobId}/results', 'get'],
];
const included = new Set();
function include(value) {
  if (Array.isArray(value)) return value.forEach(include);
  if (!value || typeof value !== 'object') return;
  if (value.$ref?.startsWith('#/components/')) {
    const [category, name] = value.$ref.slice('#/components/'.length).split('/');
    const key = `${category}/${name}`;
    if (!included.has(key)) {
      included.add(key);
      const item = structuredClone(source.components[category][name]);
      (snapshot.components[category] ??= {})[name] = item;
      include(item);
    }
  }
  Object.values(value).forEach(include);
}
for (const [route, method] of operations) {
  const operation = structuredClone(source.paths[route][method]);
  operation.security = operation.security.filter(item => Object.keys(item).length === 1 && item.dataApiKey);
  assert(operation.security.length);
  snapshot.paths[route][method] = operation;
  include(operation);
}
const { formatResponse } = require(path.resolve(formatterPath));
const examples = read('testdata/source-examples.json');
const realtime = {}, stored = {};
function cases(original) {
  const result = [];
  for (const accept of ['application/json', 'text/plain'])
    for (const respondWith of [null, 'markdown'])
      result.push({ accept, respondWith, response: formatResponse(original, { accept, representation: respondWith ?? 'original' }) });
  return result;
}
for (const [source, example] of Object.entries(examples)) {
  if (source !== 'youtube_download') realtime[source] = cases(example.response);
  stored[source] = cases({
    id: 'job_fixture0001', source, status: 'completed',
    results: source === 'youtube_download' ? [{
      file_url: 'https://customer.example/videos/example.mp4', downloaded_bytes: 1024,
      file_size: 1024, md5: '0123456789abcdef0123456789abcdef', videoId: 'dQw4w9WgXcQ',
    }] : example.response.results,
    ...(source === 'youtube_download' ? {} : { usage: example.response.usage }),
    completed_at: '2026-10-23T00:00:00Z', expires_at: '2026-10-24T00:00:00Z',
  });
}
const publicExamplesPath = path.join(path.dirname(path.resolve(openapiPath)), 'response-format-examples.generated.json');
const publicExamples = JSON.parse(fs.readFileSync(publicExamplesPath, 'utf8'));
for (const [file, value] of [
  ['internal/contract/openapi.json', snapshot],
  ['testdata/realtime-response-formats.json', realtime],
  ['testdata/stored-result-formats.json', stored],
  ['testdata/response-formats.json', publicExamples],
]) {
  if (flag) assert.deepEqual(read(file), value, `${file} is out of date`);
  else fs.writeFileSync(path.join(root, file), JSON.stringify(value, null, 2) + '\n');
}
console.log(flag ? 'Response contract and fixtures match the API.' : 'Updated response contract and fixtures.');
