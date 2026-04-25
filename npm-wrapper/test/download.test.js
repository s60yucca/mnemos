/**
 * Unit tests for binary download module
 */

const fs = require('fs');
const path = require('path');
const os = require('os');
const nock = require('nock');
const {
  downloadBinary,
  downloadFile,
  computeSHA256,
  parseChecksums,
} = require('../lib/download');

describe('Download Module', () => {
  let tempDir;

  beforeEach(() => {
    // Create temporary directory for tests
    tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'mnemos-test-'));
    
    // Clean up any pending nock interceptors
    nock.cleanAll();
  });

  afterEach(() => {
    // Clean up temporary directory
    if (fs.existsSync(tempDir)) {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
    
    // Verify all nock interceptors were used
    nock.cleanAll();
  });

  describe('URL Construction', () => {
    test('constructs correct URL for macOS Intel', async () => {
      const platformInfo = {
        os: 'darwin',
        arch: 'amd64',
        archiveName: 'mnemos_darwin_amd64.tar.gz',
        binaryName: 'mnemos',
      };

      // Mock the HTTP requests
      nock('https://github.com')
        .get('/s60yucca/mnemos/releases/download/v1.1.10/checksums.txt')
        .reply(200, 'abc123  mnemos_darwin_amd64.tar.gz\n');

      nock('https://github.com')
        .get('/s60yucca/mnemos/releases/download/v1.1.10/mnemos_darwin_amd64.tar.gz')
        .reply(404);

      await expect(
        downloadBinary('v1.1.10', platformInfo, tempDir)
      ).rejects.toThrow();

      // Verify the correct URLs were called
      expect(nock.isDone()).toBe(true);
    });

    test('constructs correct URL for Linux ARM64', async () => {
      const platformInfo = {
        os: 'linux',
        arch: 'arm64',
        archiveName: 'mnemos_linux_arm64.tar.gz',
        binaryName: 'mnemos',
      };

      nock('https://github.com')
        .get('/s60yucca/mnemos/releases/download/v1.1.10/checksums.txt')
        .reply(200, 'def456  mnemos_linux_arm64.tar.gz\n');

      nock('https://github.com')
        .get('/s60yucca/mnemos/releases/download/v1.1.10/mnemos_linux_arm64.tar.gz')
        .reply(404);

      await expect(
        downloadBinary('v1.1.10', platformInfo, tempDir)
      ).rejects.toThrow();

      expect(nock.isDone()).toBe(true);
    });

    test('constructs correct URL for Windows', async () => {
      const platformInfo = {
        os: 'windows',
        arch: 'amd64',
        archiveName: 'mnemos_windows_amd64.zip',
        binaryName: 'mnemos.exe',
      };

      nock('https://github.com')
        .get('/s60yucca/mnemos/releases/download/v1.1.10/checksums.txt')
        .reply(200, 'ghi789  mnemos_windows_amd64.zip\n');

      nock('https://github.com')
        .get('/s60yucca/mnemos/releases/download/v1.1.10/mnemos_windows_amd64.zip')
        .reply(404);

      await expect(
        downloadBinary('v1.1.10', platformInfo, tempDir)
      ).rejects.toThrow();

      expect(nock.isDone()).toBe(true);
    });

    test('handles version without v prefix', async () => {
      const platformInfo = {
        os: 'darwin',
        arch: 'amd64',
        archiveName: 'mnemos_darwin_amd64.tar.gz',
        binaryName: 'mnemos',
      };

      nock('https://github.com')
        .get('/s60yucca/mnemos/releases/download/v1.1.10/checksums.txt')
        .reply(200, 'abc123  mnemos_darwin_amd64.tar.gz\n');

      nock('https://github.com')
        .get('/s60yucca/mnemos/releases/download/v1.1.10/mnemos_darwin_amd64.tar.gz')
        .reply(404);

      await expect(
        downloadBinary('1.1.10', platformInfo, tempDir)
      ).rejects.toThrow();

      expect(nock.isDone()).toBe(true);
    });
  });

  describe('Checksum Parsing', () => {
    test('parses checksums.txt with standard format', () => {
      const content = `abc123  mnemos_darwin_amd64.tar.gz
def456  mnemos_linux_amd64.tar.gz
ghi789  mnemos_windows_amd64.zip`;

      const checksums = parseChecksums(content);

      expect(checksums.size).toBe(3);
      expect(checksums.get('mnemos_darwin_amd64.tar.gz')).toBe('abc123');
      expect(checksums.get('mnemos_linux_amd64.tar.gz')).toBe('def456');
      expect(checksums.get('mnemos_windows_amd64.zip')).toBe('ghi789');
    });

    test('handles empty lines in checksums.txt', () => {
      const content = `abc123  mnemos_darwin_amd64.tar.gz

def456  mnemos_linux_amd64.tar.gz

`;

      const checksums = parseChecksums(content);

      expect(checksums.size).toBe(2);
      expect(checksums.get('mnemos_darwin_amd64.tar.gz')).toBe('abc123');
      expect(checksums.get('mnemos_linux_amd64.tar.gz')).toBe('def456');
    });

    test('handles filenames with spaces', () => {
      const content = `abc123  some file.tar.gz
def456  another file.zip`;

      const checksums = parseChecksums(content);

      expect(checksums.size).toBe(2);
      expect(checksums.get('some file.tar.gz')).toBe('abc123');
      expect(checksums.get('another file.zip')).toBe('def456');
    });

    test('handles multiple spaces between hash and filename', () => {
      const content = `abc123    mnemos_darwin_amd64.tar.gz
def456     mnemos_linux_amd64.tar.gz`;

      const checksums = parseChecksums(content);

      expect(checksums.size).toBe(2);
      expect(checksums.get('mnemos_darwin_amd64.tar.gz')).toBe('abc123');
      expect(checksums.get('mnemos_linux_amd64.tar.gz')).toBe('def456');
    });
  });

  describe('SHA256 Computation', () => {
    test('computes correct SHA256 hash', async () => {
      const testFile = path.join(tempDir, 'test.txt');
      fs.writeFileSync(testFile, 'hello world');

      const hash = await computeSHA256(testFile);

      // SHA256 of "hello world"
      expect(hash).toBe('b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9');
    });

    test('computes different hashes for different content', async () => {
      const file1 = path.join(tempDir, 'file1.txt');
      const file2 = path.join(tempDir, 'file2.txt');
      
      fs.writeFileSync(file1, 'content1');
      fs.writeFileSync(file2, 'content2');

      const hash1 = await computeSHA256(file1);
      const hash2 = await computeSHA256(file2);

      expect(hash1).not.toBe(hash2);
    });

    test('computes same hash for same content', async () => {
      const file1 = path.join(tempDir, 'file1.txt');
      const file2 = path.join(tempDir, 'file2.txt');
      
      fs.writeFileSync(file1, 'same content');
      fs.writeFileSync(file2, 'same content');

      const hash1 = await computeSHA256(file1);
      const hash2 = await computeSHA256(file2);

      expect(hash1).toBe(hash2);
    });
  });

  describe('Download File', () => {
    test('downloads file successfully', async () => {
      const destPath = path.join(tempDir, 'downloaded.txt');
      
      nock('https://example.com')
        .get('/file.txt')
        .reply(200, 'file content');

      await downloadFile('https://example.com/file.txt', destPath);

      expect(fs.existsSync(destPath)).toBe(true);
      expect(fs.readFileSync(destPath, 'utf8')).toBe('file content');
    });

    test('follows redirects', async () => {
      const destPath = path.join(tempDir, 'downloaded.txt');
      
      nock('https://example.com')
        .get('/redirect')
        .reply(302, '', { location: 'https://example.com/final' });
      
      nock('https://example.com')
        .get('/final')
        .reply(200, 'final content');

      await downloadFile('https://example.com/redirect', destPath);

      expect(fs.existsSync(destPath)).toBe(true);
      expect(fs.readFileSync(destPath, 'utf8')).toBe('final content');
    });

    test('throws error on 404', async () => {
      const destPath = path.join(tempDir, 'downloaded.txt');
      
      nock('https://example.com')
        .get('/notfound')
        .reply(404);

      await expect(
        downloadFile('https://example.com/notfound', destPath)
      ).rejects.toThrow('HTTP 404');
    });

    test('cleans up partial download on error', async () => {
      const destPath = path.join(tempDir, 'downloaded.txt');
      
      nock('https://example.com')
        .get('/error')
        .replyWithError('Network error');

      await expect(
        downloadFile('https://example.com/error', destPath)
      ).rejects.toThrow();

      expect(fs.existsSync(destPath)).toBe(false);
    });
  });

  describe('Error Handling', () => {
    test('throws error when checksums.txt not found', async () => {
      const platformInfo = {
        os: 'darwin',
        arch: 'amd64',
        archiveName: 'mnemos_darwin_amd64.tar.gz',
        binaryName: 'mnemos',
      };

      nock('https://github.com')
        .get('/s60yucca/mnemos/releases/download/v1.1.10/checksums.txt')
        .reply(404);

      await expect(
        downloadBinary('v1.1.10', platformInfo, tempDir)
      ).rejects.toThrow();
    });

    test('throws error when archive not found', async () => {
      const platformInfo = {
        os: 'darwin',
        arch: 'amd64',
        archiveName: 'mnemos_darwin_amd64.tar.gz',
        binaryName: 'mnemos',
      };

      nock('https://github.com')
        .get('/s60yucca/mnemos/releases/download/v1.1.10/checksums.txt')
        .reply(200, 'abc123  mnemos_darwin_amd64.tar.gz\n');

      nock('https://github.com')
        .get('/s60yucca/mnemos/releases/download/v1.1.10/mnemos_darwin_amd64.tar.gz')
        .reply(404);

      await expect(
        downloadBinary('v1.1.10', platformInfo, tempDir)
      ).rejects.toThrow('Failed to download mnemos binary');
    });

    test('throws error when checksum not found for platform', async () => {
      const platformInfo = {
        os: 'darwin',
        arch: 'amd64',
        archiveName: 'mnemos_darwin_amd64.tar.gz',
        binaryName: 'mnemos',
      };

      nock('https://github.com')
        .get('/s60yucca/mnemos/releases/download/v1.1.10/checksums.txt')
        .reply(200, 'abc123  mnemos_linux_amd64.tar.gz\n');

      await expect(
        downloadBinary('v1.1.10', platformInfo, tempDir)
      ).rejects.toThrow('Checksum not found');
    });

    test('error message includes GitHub release URL', async () => {
      const platformInfo = {
        os: 'darwin',
        arch: 'amd64',
        archiveName: 'mnemos_darwin_amd64.tar.gz',
        binaryName: 'mnemos',
      };

      nock('https://github.com')
        .get('/s60yucca/mnemos/releases/download/v1.1.10/checksums.txt')
        .reply(404);

      await expect(
        downloadBinary('v1.1.10', platformInfo, tempDir)
      ).rejects.toThrow('https://github.com/s60yucca/mnemos/releases/tag/v1.1.10');
    });
  });

  describe('Retry Logic', () => {
    test('retries on network error', async () => {
      const platformInfo = {
        os: 'darwin',
        arch: 'amd64',
        archiveName: 'mnemos_darwin_amd64.tar.gz',
        binaryName: 'mnemos',
      };

      // First attempt fails with network error
      nock('https://github.com')
        .get('/s60yucca/mnemos/releases/download/v1.1.10/checksums.txt')
        .replyWithError({ code: 'ECONNRESET' });

      // Second attempt succeeds
      nock('https://github.com')
        .get('/s60yucca/mnemos/releases/download/v1.1.10/checksums.txt')
        .reply(200, 'abc123  mnemos_darwin_amd64.tar.gz\n');

      nock('https://github.com')
        .get('/s60yucca/mnemos/releases/download/v1.1.10/mnemos_darwin_amd64.tar.gz')
        .reply(404);

      await expect(
        downloadBinary('v1.1.10', platformInfo, tempDir)
      ).rejects.toThrow();

      // Verify retry happened
      expect(nock.isDone()).toBe(true);
    }, 10000);

    test('does not retry on 404 error', async () => {
      const platformInfo = {
        os: 'darwin',
        arch: 'amd64',
        archiveName: 'mnemos_darwin_amd64.tar.gz',
        binaryName: 'mnemos',
      };

      nock('https://github.com')
        .get('/s60yucca/mnemos/releases/download/v1.1.10/checksums.txt')
        .reply(404);

      // Should not make a second request
      await expect(
        downloadBinary('v1.1.10', platformInfo, tempDir)
      ).rejects.toThrow();

      expect(nock.isDone()).toBe(true);
    });

    test('gives up after max retries', async () => {
      const platformInfo = {
        os: 'darwin',
        arch: 'amd64',
        archiveName: 'mnemos_darwin_amd64.tar.gz',
        binaryName: 'mnemos',
      };

      // All attempts fail with network error
      nock('https://github.com')
        .get('/s60yucca/mnemos/releases/download/v1.1.10/checksums.txt')
        .times(3)
        .replyWithError({ code: 'ETIMEDOUT' });

      await expect(
        downloadBinary('v1.1.10', platformInfo, tempDir)
      ).rejects.toThrow('Failed to download mnemos binary');

      expect(nock.isDone()).toBe(true);
    }, 15000);
  });

  describe('Progress Messages', () => {
    let stderrWrite;
    let stderrOutput;

    beforeEach(() => {
      stderrOutput = [];
      stderrWrite = process.stderr.write;
      process.stderr.write = (msg) => {
        stderrOutput.push(msg);
        return true;
      };
    });

    afterEach(() => {
      process.stderr.write = stderrWrite;
    });

    test('writes progress messages to stderr only', async () => {
      const platformInfo = {
        os: 'darwin',
        arch: 'amd64',
        archiveName: 'mnemos_darwin_amd64.tar.gz',
        binaryName: 'mnemos',
      };

      nock('https://github.com')
        .get('/s60yucca/mnemos/releases/download/v1.1.10/checksums.txt')
        .reply(404);

      await expect(
        downloadBinary('v1.1.10', platformInfo, tempDir)
      ).rejects.toThrow();

      // Verify messages were written to stderr
      const allOutput = stderrOutput.join('');
      expect(allOutput).toContain('[mnemos-cli]');
      expect(allOutput).toContain('Downloading checksums');
    });
  });
});
