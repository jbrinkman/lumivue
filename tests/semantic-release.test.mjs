import assert from 'node:assert/strict';
import { execFile } from 'node:child_process';
import { mkdtemp } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import { promisify } from 'node:util';
import semanticRelease from 'semantic-release';
import test from 'node:test';

const exec = promisify(execFile);

async function git(cwd, ...args) {
  await exec('git', args, { cwd });
}

async function repository() {
  const directory = await mkdtemp(path.join(os.tmpdir(), 'lumivue-release-'));
  const remote = path.join(directory, 'remote.git');
  const worktree = path.join(directory, 'worktree');
  await git(directory, 'init', '--bare', remote);
  await git(directory, 'init', '--initial-branch=main', worktree);
  await git(worktree, 'config', 'user.name', 'Release Test');
  await git(worktree, 'config', 'user.email', 'release@example.com');
  await git(worktree, 'commit', '--allow-empty', '-m', 'feat: initial release');
  await git(worktree, 'remote', 'add', 'origin', remote);
  await git(worktree, 'push', '-u', 'origin', 'main');
  await git(directory, `--git-dir=${remote}`, 'symbolic-ref', 'HEAD', 'refs/heads/main');
  return { remote, worktree };
}

const options = (repositoryPath) => ({
  branches: ['main'],
  repositoryUrl: pathToFileURL(repositoryPath).href,
  tagFormat: 'v${version}',
  plugins: [
    '@semantic-release/commit-analyzer',
    '@semantic-release/release-notes-generator',
  ],
  dryRun: true,
  ci: false,
});

test('the first semantic release is 1.0.0', async () => {
  const { remote, worktree } = await repository();
  const result = await semanticRelease(options(remote), { cwd: worktree });
  assert.equal(result.nextRelease.version, '1.0.0');
});

test('subsequent conventional commits determine the next release', async () => {
  const { remote, worktree } = await repository();
  await git(worktree, 'tag', 'v1.0.0');
  await git(worktree, 'push', 'origin', 'v1.0.0');
  await git(worktree, 'commit', '--allow-empty', '-m', 'feat: add release delivery');
  await git(worktree, 'push');

  const result = await semanticRelease(options(remote), { cwd: worktree });
  assert.equal(result.nextRelease.version, '1.1.0');
});

test('non-releasable commits after 1.0.0 create no release', async () => {
  const { remote, worktree } = await repository();
  await git(worktree, 'tag', 'v1.0.0');
  await git(worktree, 'push', 'origin', 'v1.0.0');
  await git(worktree, 'commit', '--allow-empty', '-m', 'chore: update maintenance metadata');
  await git(worktree, 'push');

  const result = await semanticRelease(options(remote), { cwd: worktree });
  assert.equal(result, false);
});
