import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const root = new URL('../', import.meta.url);
const read = (path) => readFile(new URL(path, root), 'utf8');

test('release packaging requires a version and builds a versioned Wails v3 macOS arm64 DMG', async () => {
  const taskfile = await read('Taskfile.yaml');
  const script = await read('build/release-macos.sh');
  const config = await read('build/config.yml');

  assert.match(taskfile, /release:package:/);
  assert.match(taskfile, /test -n "\{\{\.VERSION\}\}"/);
  assert.match(taskfile, /build\/release-macos\.sh "\{\{\.VERSION\}\}"/);
  assert.match(config, /version: "0\.0\.0"/);
  assert.match(script, /const path = 'build\/config\.yml'/);
  assert.match(script, /\$1 "\$\{version\}"/);
  assert.match(script, /wails3 task darwin:package:dmg/);
  assert.match(script, /lumivue-\$\{version\}-darwin-arm64\.dmg/);
});

test('Semantic Release creates 1.0.0 first and prepares subsequent conventional releases', async () => {
  const packageJson = JSON.parse(await read('package.json'));
  const config = (await import(new URL('../release.config.mjs', import.meta.url))).default;

  assert.equal(packageJson.private, true);
  assert.equal(packageJson.version, '0.0.0');
  assert.deepEqual(config.branches, ['main']);
  assert.equal(config.tagFormat, 'v${version}');
  assert.deepEqual(config.plugins.slice(0, 2), [
    '@semantic-release/commit-analyzer',
    '@semantic-release/release-notes-generator',
  ]);

  const execIndex = config.plugins.findIndex(([name]) => name === '@semantic-release/exec');
  const githubIndex = config.plugins.findIndex(([name]) => name === '@semantic-release/github');
  const exec = config.plugins[execIndex];
  assert.equal(exec[1].prepareCmd, 'task release:package VERSION=${nextRelease.version}');
  assert.ok(execIndex < githubIndex);

  const github = config.plugins[githubIndex];
  assert.equal(github[1].assets[0].path, 'build/release/lumivue-${nextRelease.version}-darwin-arm64.dmg');
});

test('CD is manual, gated, serialized, and limited to release permissions', async () => {
  const workflow = await read('.github/workflows/release.yaml');
  const ci = await read('.github/workflows/ci.yaml');

  assert.match(workflow, /on:\n  workflow_dispatch:/);
  assert.doesNotMatch(workflow, /\n  (push|schedule|workflow_run):/);
  assert.match(workflow, /permissions:\n  contents: write/);
  assert.match(workflow, /concurrency:\n  group: release/);
  assert.match(workflow, /runs-on: macos-15/);
  assert.match(workflow, /ref: main/);
  assert.match(workflow, /fetch-depth: 0/);
  assert.match(workflow, /go install github\.com\/wailsapp\/wails\/v3\/cmd\/wails3@v3\.0\.0-beta\.16/);
  assert.match(workflow, /go install github\.com\/go-task\/task\/v3\/cmd\/task@v3\.53\.1/);
  assert.ok(workflow.indexOf('go-task/task') < workflow.indexOf('task check'));
  assert.match(workflow, /brew install ffmpeg@8/);
  assert.match(workflow, /PKG_CONFIG_PATH/);
  assert.ok(workflow.indexOf('brew install ffmpeg@8') < workflow.indexOf('task check'));
  assert.match(workflow, /npm ci/);
  assert.doesNotMatch(workflow, /continue-on-error:/);
  assert.ok(workflow.indexOf('task check') < workflow.indexOf('npm run test:release'));
  assert.ok(workflow.indexOf('npm run test:release') < workflow.indexOf('npm run release'));
  assert.match(workflow, /GITHUB_TOKEN: \$\{\{ secrets\.GITHUB_TOKEN \}\}/);
  assert.match(ci, /wails\/v3\/cmd\/wails3@v3\.0\.0-beta\.16/);
  assert.match(ci, /wails3 build/);
});

test('workflow actions use maintained Node 24 action majors', async () => {
  const workflows = `${await read('.github/workflows/ci.yaml')}\n${await read('.github/workflows/release.yaml')}`;
  const references = [...workflows.matchAll(/uses: (actions\/(?:checkout|setup-go|setup-node))@(v\d+)/g)];

  assert.equal(references.length, 9);
  assert.equal([...workflows.matchAll(/node-version: '24'/g)].length, 3);
  assert.doesNotMatch(workflows, /node-version: '20'/);
  const ci = await read('.github/workflows/ci.yaml');
  const testJob = ci.slice(ci.indexOf('  test:'), ci.indexOf('  build:'));
  assert.match(testJob, /runs-on: macos-15/);
  assert.equal([...ci.matchAll(/runs-on: macos-15/g)].length, 2);
  assert.equal([...ci.matchAll(/brew update/g)].length, 2);
  assert.equal([...ci.matchAll(/brew install ffmpeg@8/g)].length, 2);
  assert.equal([...ci.matchAll(/PKG_CONFIG_PATH/g)].length, 2);
  assert.ok(testJob.indexOf('brew install ffmpeg@8') < testJob.indexOf('go vet'));
  assert.ok(testJob.indexOf('npm run build') < testJob.indexOf('go vet'));
  for (const [, action, version] of references) {
    assert.equal(version, 'v7', `${action} must use its Node 24 v7 release`);
  }
});
