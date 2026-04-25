/**
 * Binary execution module
 * Spawns mnemos binary as child process with transparent stdio forwarding
 */

const { spawn } = require('child_process');

/**
 * Execute the mnemos binary with forwarded stdio and signals
 * @param {string} binaryPath - Path to the mnemos binary
 * @param {string[]} args - Command-line arguments to forward
 * @returns {Promise<number>} Exit code from the binary
 */
function executeBinary(binaryPath, args = []) {
  return new Promise((resolve, reject) => {
    // Spawn child process with inherited stdio streams
    const child = spawn(binaryPath, args, {
      stdio: 'inherit', // Forward stdin, stdout, stderr
      cwd: process.cwd(),
    });

    // Forward SIGINT (Ctrl+C) to child process
    const sigintHandler = () => {
      child.kill('SIGINT');
    };
    process.on('SIGINT', sigintHandler);

    // Forward SIGTERM to child process
    const sigtermHandler = () => {
      child.kill('SIGTERM');
    };
    process.on('SIGTERM', sigtermHandler);

    // Handle child process exit
    child.on('exit', (code, signal) => {
      // Clean up signal handlers
      process.removeListener('SIGINT', sigintHandler);
      process.removeListener('SIGTERM', sigtermHandler);

      // If child was killed by signal, exit with appropriate code
      if (signal) {
        // Exit with 128 + signal number (standard Unix convention)
        const signalExitCode = signal === 'SIGINT' ? 130 : 143;
        resolve(signalExitCode);
      } else {
        // Return child's exit code
        resolve(code || 0);
      }
    });

    // Handle spawn errors (e.g., binary not found, not executable)
    child.on('error', (err) => {
      // Clean up signal handlers
      process.removeListener('SIGINT', sigintHandler);
      process.removeListener('SIGTERM', sigtermHandler);

      reject(
        new Error(
          `Failed to execute binary: ${binaryPath}\n` +
            `Error: ${err.message}\n` +
            `Action: Check that the binary exists and has executable permissions`
        )
      );
    });
  });
}

module.exports = { executeBinary };
