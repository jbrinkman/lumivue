export default {
  branches: ['main'],
  tagFormat: 'v${version}',
  plugins: [
    '@semantic-release/commit-analyzer',
    '@semantic-release/release-notes-generator',
    [
      '@semantic-release/exec',
      {
        prepareCmd: 'task release:package VERSION=${nextRelease.version}',
      },
    ],
    [
      '@semantic-release/github',
      {
        assets: [
          {
            path: 'build/release/lumivue-${nextRelease.version}-darwin-arm64.dmg',
            label: 'Lumivue ${nextRelease.version} for Apple Silicon macOS (unsigned)',
          },
        ],
      },
    ],
  ],
};
