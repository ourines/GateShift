# Release Process

This project uses GitHub Actions to automatically build and publish releases.

## Creating a New Release

1. Make sure all your changes are committed and pushed to the main branch.

2. Create and push a new tag with a semantic version (e.g., v1.0.0):
   ```
   git tag v1.0.0
   git push origin v1.0.0
   ```

3. The GitHub Actions workflow will automatically:
   - Build binaries for multiple platforms (Linux, macOS, Windows)
   - Generate SHA256 checksums
   - Create a changelog from commit messages
   - Create a GitHub Release with all artifacts

4. Check the Actions tab in GitHub to monitor the build and release process.

5. Once complete, the new release will be available at:
   https://github.com/[username]/GateShift/releases

## Alternative Build Methods

### Manual Workflow Trigger (Without Tags)

For testing or for changes that don't warrant a new version tag, you can manually trigger the build workflow:

1. Go to the Actions tab in your GitHub repository
2. Select the "Build and Release" workflow
3. Click "Run workflow" dropdown button
4. Enter a version identifier (or leave as "dev") 
5. Click "Run workflow"

This will build all platform binaries and make them available as downloadable artifacts, but won't create an official GitHub Release.

### Automatic Build Testing

A separate workflow automatically runs whenever changes are pushed to the main branch that affect code, Makefile, or Go dependencies. This workflow ensures that the build process is working correctly without creating a release.

To check build tests:
1. Go to the Actions tab
2. Click on the "Test Build" workflow
3. View the latest workflow runs

## Supported Platforms

The automated build process creates binaries for:
- Linux (amd64, arm64): `gateshift-linux-amd64`, `gateshift-linux-arm64`
- macOS (amd64, arm64): `gateshift-darwin-amd64`, `gateshift-darwin-arm64`
- Windows (amd64): `gateshift-windows-amd64.exe`

## Manually Testing Before Release

Before creating a release tag, you can test the build process locally:

```
make clean
make build-all
```

This will build all platform binaries in the `bin/` directory. 