# Installation Guide

## Download Pre-built Binary (Recommended)

1. Download the latest release for your platform from [GitHub Releases](https://github.com/zakandrewking/lockplane/releases/latest)
2. Extract the archive: `tar -xzf lockplane_*.tar.gz`
3. Move to your PATH: `sudo mv lockplane /usr/local/bin/`
4. Verify: `lockplane version`

## Build from Source

```bash
git clone https://github.com/zakandrewking/lockplane.git
cd lockplane
go install .
```

## Verify Installation

```bash
lockplane
lockplane version
lockplane help
```
