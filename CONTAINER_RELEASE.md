# Container Release Workflow

This repository includes an automated GitHub Actions workflow that builds and publishes Docker containers to GitHub Container Registry (GHCR).

## Features

- **Automated Nightly Builds**: Runs every Sunday on midnight UTC
- **Manual Trigger**: Can be triggered manually from GitHub Actions tab
- **Auto-incrementing Versions**: Each build increments the version number (1, 2, 3, etc.)
- **Multiple Tags**: Each build creates both a versioned tag and updates the `latest` tag
- **Multi-platform**: Builds for both AMD64 and ARM64 architectures

## Container Location

The containers are published to:
```
ghcr.io/neptunehub/audiomuse-ai-musicserver:latest
ghcr.io/neptunehub/audiomuse-ai-musicserver:1
ghcr.io/neptunehub/audiomuse-ai-musicserver:2
# ... and so on
```

## Usage

### Pull the Latest Version
```bash
docker pull ghcr.io/neptunehub/audiomuse-ai-musicserver:latest
```

### Pull a Specific Version
```bash
docker pull ghcr.io/neptunehub/audiomuse-ai-musicserver:1
```

### Run the Container
```bash
docker run -d \
  --name audiomuse \
  -p 3000:3000 \
  -p 8080:8080 \
  -v /path/to/music:/music \
  -v /path/to/config:/config \
  ghcr.io/neptunehub/audiomuse-ai-musicserver:latest
```

### Kubernetes Deployment
For Kubernetes deployments, see the included `deployment.yaml` file which provides a complete manifest with persistent volumes and services.

## Manual Trigger

To manually trigger a new build:

1. Go to the GitHub repository
2. Click on "Actions" tab
3. Select "Build and Publish Container" workflow
4. Click "Run workflow" button
5. Select the `main` branch and click "Run workflow"

## Version Tracking

The current version is tracked in the `VERSION` file in the repository root. Each successful build:
1. Reads the current version
2. Increments it by 1
3. Commits the new version back to the repository
4. Tags the container with both the new version number and `latest`

## Workflow Configuration

The workflow is defined in `.github/workflows/build-and-publish.yml` and includes:

- **Scheduled Trigger**: `0 0 * * *` (midnight UTC daily)
- **Manual Trigger**: `workflow_dispatch` event
- **Push Trigger**: Runs on pushes to `main` branch
- **Multi-platform Build**: Linux AMD64 and ARM64
- **Caching**: Docker layer caching for faster builds
- **Automatic Versioning**: Incremental version numbers
- **GHCR Publishing**: Publishes to GitHub Container Registry

## Permissions

The workflow requires:
- `contents: write` - To update the VERSION file
- `packages: write` - To publish to GitHub Container Registry

These permissions are automatically available to GitHub Actions in this repository.