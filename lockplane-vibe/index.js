const { execFileSync } = require('child_process');
const path = require('path');

const binaryName = process.platform === 'win32' ? 'lockplane.exe' : 'lockplane';
const binaryPath = path.join(__dirname, 'bin', binaryName);

/**
 * Execute lockplane CLI programmatically
 * @param {string[]} args - Command line arguments
 * @param {object} options - Execution options (stdio, cwd, etc.)
 * @returns {Buffer|string} - Command output
 */
function lockplane(args, options = {}) {
  return execFileSync(binaryPath, args, {
    encoding: 'utf8',
    ...options
  });
}

module.exports = lockplane;
module.exports.lockplane = lockplane;
module.exports.binaryPath = binaryPath;
