#!/usr/bin/env node

const https = require('https');
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const packageJson = require('../package.json');

const VERSION = packageJson.version;
const GITHUB_REPO = 'lockplane/lockplane';

function getPlatformInfo() {
  const platform = process.platform;
  const arch = process.arch;

  let os, archName, ext;

  // Determine OS
  if (platform === 'darwin') {
    os = 'Darwin';
    ext = 'tar.gz';
  } else if (platform === 'linux') {
    os = 'Linux';
    ext = 'tar.gz';
  } else if (platform === 'win32') {
    os = 'Windows';
    ext = 'zip';
  } else {
    throw new Error(`Unsupported platform: ${platform}`);
  }

  // Determine architecture
  // GoReleaser uses 'amd64' and 'arm64' in asset names
  if (arch === 'x64') {
    archName = 'amd64';
  } else if (arch === 'arm64') {
    archName = 'arm64';
  } else {
    throw new Error(`Unsupported architecture: ${arch}`);
  }

  return { os, archName, ext };
}

function getDownloadUrl() {
  const { os, archName, ext } = getPlatformInfo();
  // GoReleaser creates assets with this naming:
  // - Linux/macOS: lockplane-{os}-{arch}.tar.gz
  // - Windows: lockplane-windows-{arch}.exe.zip
  const osName = os.toLowerCase();
  let filename;
  if (os === 'Windows') {
    filename = `lockplane-${osName}-${archName}.exe.${ext}`;
  } else {
    filename = `lockplane-${osName}-${archName}.${ext}`;
  }
  return `https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}/${filename}`;
}

function download(url, dest) {
  return new Promise((resolve, reject) => {
    console.log(`Downloading lockplane v${VERSION}...`);
    console.log(`URL: ${url}`);

    const file = fs.createWriteStream(dest);
    https.get(url, (response) => {
      if (response.statusCode === 302 || response.statusCode === 301) {
        // Follow redirect
        return download(response.headers.location, dest).then(resolve).catch(reject);
      }

      if (response.statusCode !== 200) {
        reject(new Error(`Failed to download: ${response.statusCode} ${response.statusMessage}`));
        return;
      }

      response.pipe(file);
      file.on('finish', () => {
        file.close();
        resolve();
      });
    }).on('error', (err) => {
      fs.unlink(dest, () => {}); // Delete the file
      reject(err);
    });
  });
}

function extract(archivePath, destDir) {
  const { ext } = getPlatformInfo();

  console.log('Extracting binary...');

  if (ext === 'tar.gz') {
    execSync(`tar -xzf "${archivePath}" -C "${destDir}"`, { stdio: 'inherit' });
  } else if (ext === 'zip') {
    // Use unzip on Unix-like systems, or built-in on Windows
    if (process.platform === 'win32') {
      execSync(`powershell -command "Expand-Archive -Path '${archivePath}' -DestinationPath '${destDir}'"`, { stdio: 'inherit' });
    } else {
      execSync(`unzip -q "${archivePath}" -d "${destDir}"`, { stdio: 'inherit' });
    }
  }
}

async function install() {
  try {
    const binDir = path.join(__dirname, '..', 'bin');
    const tmpDir = path.join(__dirname, '..', 'tmp');

    // Create directories
    if (!fs.existsSync(binDir)) {
      fs.mkdirSync(binDir, { recursive: true });
    }
    if (!fs.existsSync(tmpDir)) {
      fs.mkdirSync(tmpDir, { recursive: true });
    }

    const { ext } = getPlatformInfo();
    const archivePath = path.join(tmpDir, `lockplane.${ext}`);
    const url = getDownloadUrl();

    // Download
    await download(url, archivePath);

    // Extract
    extract(archivePath, binDir);

    // Rename the binary from lockplane-{os}-{arch} to lockplane
    const { os, archName } = getPlatformInfo();
    const osName = os.toLowerCase();
    const extractedName = process.platform === 'win32'
      ? `lockplane-${osName}-${archName}.exe`
      : `lockplane-${osName}-${archName}`;
    const finalName = process.platform === 'win32' ? 'lockplane.exe' : 'lockplane';

    const extractedPath = path.join(binDir, extractedName);
    const finalPath = path.join(binDir, finalName);

    if (fs.existsSync(extractedPath)) {
      fs.renameSync(extractedPath, finalPath);
    }

    // Make binary executable (Unix-like systems)
    if (process.platform !== 'win32') {
      const binaryPath = path.join(binDir, 'lockplane');
      if (fs.existsSync(binaryPath)) {
        fs.chmodSync(binaryPath, '755');
      }
    }

    // Clean up
    fs.rmSync(tmpDir, { recursive: true, force: true });

    console.log('✓ lockplane installed successfully!');
    process.exit(0);
  } catch (error) {
    console.error('Error installing lockplane:', error.message);
    console.error('\nPlease install manually from:');
    console.error(`https://github.com/${GITHUB_REPO}/releases/tag/v${VERSION}`);
    process.exit(1);
  }
}

install();
