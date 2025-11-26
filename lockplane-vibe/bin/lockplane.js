#!/usr/bin/env node

const { spawn } = require('child_process');
const path = require('path');

const binaryName = process.platform === 'win32' ? 'lockplane.exe' : 'lockplane';
const binaryPath = path.join(__dirname, binaryName);

const child = spawn(binaryPath, process.argv.slice(2), {
  stdio: 'inherit',
  windowsHide: false
});

child.on('exit', (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
  } else {
    process.exit(code);
  }
});

child.on('error', (err) => {
  console.error('Error executing lockplane:', err.message);
  console.error('\nIf the binary is missing, try reinstalling:');
  console.error('  npm install lockplane');
  process.exit(1);
});
