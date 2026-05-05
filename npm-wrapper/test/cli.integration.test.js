/**
 * Integration tests for CLI entry point
 * Tests end-to-end flow with mocked dependencies
 */

const fs = require('fs');
const path = require('path');
const os = require('os');
const { spawn } = require('child_process');

describe('CLI Integration Tests', () => {
  let tempDir;
  let originalPlatform;
  let originalArch;
  let cliPath;

  beforeEach(() => {
    // Create temporary directory for tests
    tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'mnemos-cli-test-'));
    
    // Save original platform values
    originalPlatform = process.platform;
    originalArch = process.arch;
    
    // CLI path
    cliPath = path.join(__dirname, '..', 'cli.js');
  });

  afterEach(() => {
    // Clean up temporary directory
    if (fs.existsSync(tempDir)) {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
    
    // Restore original platform values
    Object.defineProperty(process, 'platform', {
      value: originalPlatform,
      writable: true,
      configurable: true,
    });
    Object.defineProperty(process, 'arch', {
      value: originalArch,
      writable: true,
      configurable: true,
    });
  });

  describe('End-to-End Flow with Mocked Binary', () => {
    test('executes successfully when binary is cached', (done) => {
      // Create a mock cached binary
      const version = require('../package.json').version;
      const versionTag = version.startsWith('v') ? version : `v${version}`;
      const cacheDir = path.join(os.homedir(), '.mnemos', 'bin', versionTag);
      
      // Create cache directory
      fs.mkdirSync(cacheDir, { recursive: true });
      
      // Create mock binary (simple script that exits with code 0)
      const mockBinaryPath = path.join(cacheDir, 'mnemos');
      const mockBinaryContent = process.platform === 'win32'
        ? '@echo off\nexit 0'
        : '#!/bin/sh\nexit 0';
      
      fs.writeFileSync(mockBinaryPath, mockBinaryContent);
      if (process.platform !== 'win32') {
        fs.chmodSync(mockBinaryPath, 0o755);
      }
      
      // Execute CLI
      const child = spawn('node', [cliPath, '--version'], {
        stdio: ['ignore', 'pipe', 'pipe'],
      });
      
      let stderr = '';
      child.stderr.on('data', (data) => {
        stderr += data.toString();
      });
      
      child.on('exit', (code) => {
        // Clean up mock binary
        fs.rmSync(cacheDir, { recursive: true, force: true });
        
        // Should exit successfully
        expect(code).toBe(0);
        
        // Should not show download messages (binary was cached)
        expect(stderr).not.toContain('Downloading');
        
        done();
      });
    }, 10000);

    test('forwards command-line arguments to binary', (done) => {
      // Create a mock cached binary that verifies arguments were passed
      const version = require('../package.json').version;
      const versionTag = version.startsWith('v') ? version : `v${version}`;
      const cacheDir = path.join(os.homedir(), '.mnemos', 'bin', versionTag);
      
      fs.mkdirSync(cacheDir, { recursive: true });
      
      const mockBinaryPath = path.join(cacheDir, 'mnemos');
      // Binary that exits with success if it receives arguments
      const mockBinaryContent = process.platform === 'win32'
        ? '@echo off\nif "%1"=="" (exit /b 1) else (echo Arguments received & exit /b 0)'
        : '#!/bin/sh\nif [ $# -eq 0 ]; then exit 1; fi\necho "Arguments received"\nexit 0';
      
      fs.writeFileSync(mockBinaryPath, mockBinaryContent);
      if (process.platform !== 'win32') {
        fs.chmodSync(mockBinaryPath, 0o755);
      }
      
      // Execute CLI with arguments
      const child = spawn('node', [cliPath, 'serve', '--port', '8080'], {
        stdio: ['ignore', 'pipe', 'pipe'],
      });
      
      let stdout = '';
      child.stdout.on('data', (data) => {
        stdout += data.toString();
      });
      
      child.on('exit', (code) => {
        // Clean up
        try { fs.rmSync(cacheDir, { recursive: true, force: true }); } catch (_) {}
        
        // Should exit with 0 (arguments were passed)
        expect(code).toBe(0);
        
        // Should have received arguments
        expect(stdout).toContain('Arguments received');
        
        done();
      });
    }, 20000);

    test('forwards exit code from binary', (done) => {
      // Create a mock binary that exits with code 42
      const version = require('../package.json').version;
      const versionTag = version.startsWith('v') ? version : `v${version}`;
      const cacheDir = path.join(os.homedir(), '.mnemos', 'bin', versionTag);
      
      fs.mkdirSync(cacheDir, { recursive: true });
      
      const mockBinaryPath = path.join(cacheDir, 'mnemos');
      const mockBinaryContent = process.platform === 'win32'
        ? '@echo off\nexit 42'
        : '#!/bin/sh\nexit 42';
      
      fs.writeFileSync(mockBinaryPath, mockBinaryContent);
      if (process.platform !== 'win32') {
        fs.chmodSync(mockBinaryPath, 0o755);
      }
      
      // Execute CLI
      const child = spawn('node', [cliPath], {
        stdio: ['ignore', 'pipe', 'pipe'],
      });
      
      child.on('exit', (code) => {
        // Clean up
        fs.rmSync(cacheDir, { recursive: true, force: true });
        
        // Should forward the exit code
        expect(code).toBe(42);
        
        done();
      });
    }, 10000);
  });

  describe('Error Handling - Unsupported Platforms', () => {
    test('exits with error for unsupported platform', (done) => {
      // Execute CLI with NODE_PLATFORM and NODE_ARCH environment variables
      // to simulate unsupported platform
      const child = spawn('node', [
        '-e',
        `
        Object.defineProperty(process, 'platform', { value: 'freebsd', configurable: true });
        Object.defineProperty(process, 'arch', { value: 'x64', configurable: true });
        require('${cliPath.replace(/\\/g, '\\\\')}');
        `
      ], {
        stdio: ['ignore', 'pipe', 'pipe'],
      });
      
      let stderr = '';
      child.stderr.on('data', (data) => {
        stderr += data.toString();
      });
      
      child.on('exit', (code) => {
        // Should exit with error code
        expect(code).toBe(1);
        
        // Should show error message
        expect(stderr).toContain('[mnemos-cli] Error:');
        expect(stderr).toContain('Unsupported platform');
        expect(stderr).toContain('freebsd');
        
        done();
      });
    }, 10000);

    test('lists supported platforms in error message', (done) => {
      // Execute CLI with mocked unsupported architecture
      const child = spawn('node', [
        '-e',
        `
        Object.defineProperty(process, 'platform', { value: 'linux', configurable: true });
        Object.defineProperty(process, 'arch', { value: 'arm', configurable: true });
        require('${cliPath.replace(/\\/g, '\\\\')}');
        `
      ], {
        stdio: ['ignore', 'pipe', 'pipe'],
      });
      
      let stderr = '';
      child.stderr.on('data', (data) => {
        stderr += data.toString();
      });
      
      child.on('exit', (code) => {
        expect(code).toBe(1);
        
        // Should list all supported platforms
        expect(stderr).toContain('darwin-x64');
        expect(stderr).toContain('darwin-arm64');
        expect(stderr).toContain('linux-x64');
        expect(stderr).toContain('linux-arm64');
        expect(stderr).toContain('win32-x64');
        
        done();
      });
    }, 10000);

    test('includes GitHub link in error message', (done) => {
      // Execute CLI with mocked unsupported platform
      const child = spawn('node', [
        '-e',
        `
        Object.defineProperty(process, 'platform', { value: 'aix', configurable: true });
        Object.defineProperty(process, 'arch', { value: 'ppc64', configurable: true });
        require('${cliPath.replace(/\\/g, '\\\\')}');
        `
      ], {
        stdio: ['ignore', 'pipe', 'pipe'],
      });
      
      let stderr = '';
      child.stderr.on('data', (data) => {
        stderr += data.toString();
      });
      
      child.on('exit', (code) => {
        expect(code).toBe(1);
        expect(stderr).toContain('https://github.com/s60yucca/mnemos#installation');
        
        done();
      });
    }, 10000);
  });

  describe('Error Handling - Download Failures', () => {
    test('exits with error when download fails with network error', (done) => {
      // Create a test script that mocks download failure
      const testScript = `
        const path = require('path');
        const Module = require('module');
        const originalRequire = Module.prototype.require;
        
        // Mock the download module to simulate network failure
        Module.prototype.require = function(id) {
          if (id === './lib/download' || id.endsWith('/lib/download.js')) {
            return {
              downloadBinary: async () => {
                throw new Error('Network error: ECONNREFUSED');
              }
            };
          }
          return originalRequire.apply(this, arguments);
        };
        
        require('${cliPath.replace(/\\/g, '\\\\')}');
      `;
      
      const child = spawn('node', ['-e', testScript], {
        stdio: ['ignore', 'pipe', 'pipe'],
      });
      
      let stderr = '';
      child.stderr.on('data', (data) => {
        stderr += data.toString();
      });
      
      child.on('exit', (code) => {
        // Should exit with error code
        expect(code).toBe(1);
        
        // Should show error message
        expect(stderr).toContain('[mnemos-cli] Error:');
        
        done();
      });
    }, 10000);

    test('exits with error when binary not in cache and version not found', (done) => {
      // Create a test script that mocks 404 error
      const testScript = `
        const path = require('path');
        const Module = require('module');
        const originalRequire = Module.prototype.require;
        
        // Mock the download module to simulate 404
        Module.prototype.require = function(id) {
          if (id === './lib/download' || id.endsWith('/lib/download.js')) {
            return {
              downloadBinary: async () => {
                const err = new Error('Failed to download mnemos binary\\n\\nDetails: HTTP 404\\nURL: https://github.com/s60yucca/mnemos/releases/download/v999.999.999/mnemos_darwin_arm64.tar.gz');
                throw err;
              }
            };
          }
          return originalRequire.apply(this, arguments);
        };
        
        require('${cliPath.replace(/\\/g, '\\\\')}');
      `;
      
      const child = spawn('node', ['-e', testScript], {
        stdio: ['ignore', 'pipe', 'pipe'],
      });
      
      let stderr = '';
      child.stderr.on('data', (data) => {
        stderr += data.toString();
      });
      
      child.on('exit', (code) => {
        // Should exit with error code
        expect(code).toBe(1);
        
        // Should show error message
        expect(stderr).toContain('[mnemos-cli] Error:');
        
        done();
      });
    }, 10000);
  });

  describe('Stdio Forwarding', () => {
    test('forwards stdout from binary', (done) => {
      // Create a mock binary that writes to stdout
      const version = require('../package.json').version;
      const versionTag = version.startsWith('v') ? version : `v${version}`;
      const cacheDir = path.join(os.homedir(), '.mnemos', 'bin', versionTag);
      
      fs.mkdirSync(cacheDir, { recursive: true });
      
      const mockBinaryPath = path.join(cacheDir, 'mnemos');
      const mockBinaryContent = process.platform === 'win32'
        ? '@echo off\necho Hello from mnemos\nexit 0'
        : '#!/bin/sh\necho "Hello from mnemos"\nexit 0';
      
      fs.writeFileSync(mockBinaryPath, mockBinaryContent);
      if (process.platform !== 'win32') {
        fs.chmodSync(mockBinaryPath, 0o755);
      }
      
      // Execute CLI
      const child = spawn('node', [cliPath], {
        stdio: ['ignore', 'pipe', 'pipe'],
      });
      
      let stdout = '';
      child.stdout.on('data', (data) => {
        stdout += data.toString();
      });
      
      child.on('exit', (code) => {
        // Clean up
        fs.rmSync(cacheDir, { recursive: true, force: true });
        
        expect(code).toBe(0);
        expect(stdout).toContain('Hello from mnemos');
        
        done();
      });
    }, 10000);

    test('forwards stderr from binary', (done) => {
      // Create a mock binary that writes to stderr
      const version = require('../package.json').version;
      const versionTag = version.startsWith('v') ? version : `v${version}`;
      const cacheDir = path.join(os.homedir(), '.mnemos', 'bin', versionTag);
      
      fs.mkdirSync(cacheDir, { recursive: true });
      
      const mockBinaryPath = path.join(cacheDir, 'mnemos');
      const mockBinaryContent = process.platform === 'win32'
        ? '@echo off\necho Error message 1>&2\nexit 0'
        : '#!/bin/sh\necho "Error message" >&2\nexit 0';
      
      fs.writeFileSync(mockBinaryPath, mockBinaryContent);
      if (process.platform !== 'win32') {
        fs.chmodSync(mockBinaryPath, 0o755);
      }
      
      // Execute CLI
      const child = spawn('node', [cliPath], {
        stdio: ['ignore', 'pipe', 'pipe'],
      });
      
      let stderr = '';
      child.stderr.on('data', (data) => {
        stderr += data.toString();
      });
      
      child.on('exit', (code) => {
        // Clean up
        fs.rmSync(cacheDir, { recursive: true, force: true });
        
        expect(code).toBe(0);
        expect(stderr).toContain('Error message');
        
        done();
      });
    }, 10000);
  });

  describe('MCP Stdio Compatibility', () => {
    test('does not pollute stdout with wrapper messages when binary is cached', (done) => {
      // Create a mock binary that outputs JSON-RPC
      const version = require('../package.json').version;
      const versionTag = version.startsWith('v') ? version : `v${version}`;
      const cacheDir = path.join(os.homedir(), '.mnemos', 'bin', versionTag);
      
      fs.mkdirSync(cacheDir, { recursive: true });
      
      const mockBinaryPath = path.join(cacheDir, 'mnemos');
      const jsonRpcResponse = '{"jsonrpc":"2.0","result":{"version":"1.0.0"},"id":1}';
      const mockBinaryContent = process.platform === 'win32'
        ? `@echo off\necho ${jsonRpcResponse}\nexit 0`
        : `#!/bin/sh\necho '${jsonRpcResponse}'\nexit 0`;
      
      fs.writeFileSync(mockBinaryPath, mockBinaryContent);
      if (process.platform !== 'win32') {
        fs.chmodSync(mockBinaryPath, 0o755);
      }
      
      // Execute CLI
      const child = spawn('node', [cliPath], {
        stdio: ['ignore', 'pipe', 'pipe'],
      });
      
      let stdout = '';
      let stderr = '';
      
      child.stdout.on('data', (data) => {
        stdout += data.toString();
      });
      
      child.stderr.on('data', (data) => {
        stderr += data.toString();
      });
      
      child.on('exit', (code) => {
        // Clean up
        fs.rmSync(cacheDir, { recursive: true, force: true });
        
        expect(code).toBe(0);
        
        // Stdout should only contain the JSON-RPC response
        expect(stdout.trim()).toBe(jsonRpcResponse);
        
        // Wrapper messages should only be on stderr (if any)
        expect(stdout).not.toContain('[mnemos-cli]');
        
        done();
      });
    }, 10000);
  });
});
