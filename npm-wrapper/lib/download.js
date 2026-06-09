/**
 * Binary downloader module with checksum verification and archive extraction
 * Downloads mnemos binaries from GitHub releases, verifies checksums, and extracts archives
 */

const https = require('https');
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');
const zlib = require('zlib');
const { pipeline } = require('stream/promises');

/**
 * Download a file from a URL to a destination path
 * @param {string} url - URL to download from
 * @param {string} dest - Destination file path
 * @returns {Promise<void>}
 */
function downloadFile(url, dest) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(dest);
    const removePartial = () => fs.rmSync(dest, { force: true });
    
    https.get(url, (response) => {
      if (response.statusCode === 302 || response.statusCode === 301) {
        // Follow redirect
        file.close();
        removePartial();
        return downloadFile(response.headers.location, dest)
          .then(resolve)
          .catch(reject);
      }
      
      if (response.statusCode !== 200) {
        file.close();
        removePartial();
        return reject(new Error(`HTTP ${response.statusCode}: ${url}`));
      }
      
      response.pipe(file);
      
      file.on('finish', () => {
        file.close();
        resolve();
      });
    }).on('error', (err) => {
      file.close();
      removePartial();
      reject(err);
    });
    
    file.on('error', (err) => {
      file.close();
      removePartial();
      reject(err);
    });
  });
}

/**
 * Compute SHA256 hash of a file
 * @param {string} filePath - Path to file
 * @returns {Promise<string>} Hex-encoded SHA256 hash
 */
async function computeSHA256(filePath) {
  const hash = crypto.createHash('sha256');
  const stream = fs.createReadStream(filePath);
  
  for await (const chunk of stream) {
    hash.update(chunk);
  }
  
  return hash.digest('hex');
}

/**
 * Parse checksums.txt file
 * @param {string} content - Content of checksums.txt
 * @returns {Map<string, string>} Map of filename to hash
 */
function parseChecksums(content) {
  const checksums = new Map();
  const lines = content.split('\n');
  
  for (const line of lines) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    
    // Format: "hash  filename" (two spaces)
    const parts = trimmed.split(/\s+/);
    if (parts.length >= 2) {
      const hash = parts[0];
      const filename = parts.slice(1).join(' ');
      checksums.set(filename, hash);
    }
  }
  
  return checksums;
}

/**
 * Extract .tar.gz archive
 * @param {string} archivePath - Path to .tar.gz file
 * @param {string} destDir - Destination directory
 * @returns {Promise<void>}
 */
async function extractTarGz(archivePath, destDir) {
  const { Transform } = require('stream');
  
  return new Promise((resolve, reject) => {
    const gunzip = zlib.createGunzip();
    const input = fs.createReadStream(archivePath);
    
    let buffer = Buffer.alloc(0);
    let currentFile = null;
    let fileStream = null;
    let bytesWritten = 0;
    
    const parser = new Transform({
      transform(chunk, encoding, callback) {
        buffer = Buffer.concat([buffer, chunk]);
        
        while (buffer.length >= 512) {
          // If we're currently writing a file
          if (currentFile) {
            const remaining = currentFile.size - bytesWritten;
            const toWrite = Math.min(remaining, buffer.length);
            
            fileStream.write(buffer.slice(0, toWrite));
            bytesWritten += toWrite;
            buffer = buffer.slice(toWrite);
            
            if (bytesWritten >= currentFile.size) {
              // File complete, pad to 512-byte boundary
              const padding = (512 - (currentFile.size % 512)) % 512;
              buffer = buffer.slice(padding);
              fileStream.end();
              currentFile = null;
              fileStream = null;
              bytesWritten = 0;
            }
            
            if (buffer.length < 512) break;
          }
          
          // Parse tar header
          const header = buffer.slice(0, 512);
          
          // Check if this is a zero block (end of archive)
          if (header.every(b => b === 0)) {
            buffer = buffer.slice(512);
            continue;
          }
          
          // Parse file name (offset 0, 100 bytes)
          const fileName = header.toString('utf8', 0, 100).replace(/\0.*$/, '');
          
          // Parse file size (offset 124, 12 bytes, octal)
          const sizeStr = header.toString('utf8', 124, 136).replace(/\0.*$/, '').trim();
          const fileSize = parseInt(sizeStr, 8) || 0;
          
          // Parse file type (offset 156, 1 byte)
          const fileType = header.toString('utf8', 156, 157);
          
          buffer = buffer.slice(512);
          
          // Only extract regular files containing 'mnemos'
          if ((fileType === '0' || fileType === '\0') && fileName.includes('mnemos')) {
            const outputPath = path.join(destDir, path.basename(fileName));
            fileStream = fs.createWriteStream(outputPath);
            currentFile = { name: fileName, size: fileSize, path: outputPath };
            bytesWritten = 0;
            
            fileStream.on('finish', () => {
              // Set executable permissions on Unix
              if (process.platform !== 'win32') {
                fs.chmodSync(outputPath, 0o755);
              }
            });
          } else {
            // Skip this file
            const padding = (512 - (fileSize % 512)) % 512;
            const totalSkip = fileSize + padding;
            buffer = buffer.slice(totalSkip);
          }
        }
        
        callback();
      }
    });
    
    parser.on('finish', () => {
      setTimeout(() => resolve(), 100); // Give file streams time to close
    });
    
    input.pipe(gunzip).pipe(parser).on('error', reject);
  });
}

/**
 * Extract .zip archive
 * @param {string} archivePath - Path to .zip file
 * @param {string} destDir - Destination directory
 * @returns {Promise<void>}
 */
async function extractZip(archivePath, destDir) {
  // Simple zip extraction using zlib
  // For Windows .zip files, we need to parse the zip format
  return new Promise((resolve, reject) => {
    const data = fs.readFileSync(archivePath);
    
    // Find central directory end record (last 22+ bytes)
    let cdEnd = -1;
    for (let i = data.length - 22; i >= 0; i--) {
      if (data.readUInt32LE(i) === 0x06054b50) {
        cdEnd = i;
        break;
      }
    }
    
    if (cdEnd === -1) {
      return reject(new Error('Invalid ZIP file: central directory not found'));
    }
    
    // Read central directory offset
    const cdOffset = data.readUInt32LE(cdEnd + 16);
    
    // Parse central directory entries
    let offset = cdOffset;
    while (offset < cdEnd) {
      const signature = data.readUInt32LE(offset);
      if (signature !== 0x02014b50) break; // Central directory file header signature
      
      const fileNameLength = data.readUInt16LE(offset + 28);
      const extraFieldLength = data.readUInt16LE(offset + 30);
      const fileCommentLength = data.readUInt16LE(offset + 32);
      const localHeaderOffset = data.readUInt32LE(offset + 42);
      
      const fileName = data.toString('utf8', offset + 46, offset + 46 + fileNameLength);
      
      // Only extract the mnemos binary
      if (fileName.includes('mnemos')) {
        // Read local file header
        const localSig = data.readUInt32LE(localHeaderOffset);
        if (localSig !== 0x04034b50) {
          return reject(new Error('Invalid ZIP file: local header signature mismatch'));
        }
        
        const compressionMethod = data.readUInt16LE(localHeaderOffset + 8);
        const compressedSize = data.readUInt32LE(localHeaderOffset + 18);
        const localFileNameLength = data.readUInt16LE(localHeaderOffset + 26);
        const localExtraFieldLength = data.readUInt16LE(localHeaderOffset + 28);
        
        const dataOffset = localHeaderOffset + 30 + localFileNameLength + localExtraFieldLength;
        const compressedData = data.slice(dataOffset, dataOffset + compressedSize);
        
        let fileData;
        if (compressionMethod === 0) {
          // No compression
          fileData = compressedData;
        } else if (compressionMethod === 8) {
          // Deflate compression
          fileData = zlib.inflateRawSync(compressedData);
        } else {
          return reject(new Error(`Unsupported compression method: ${compressionMethod}`));
        }
        
        const outputPath = path.join(destDir, path.basename(fileName));
        fs.writeFileSync(outputPath, fileData);
        
        return resolve();
      }
      
      offset += 46 + fileNameLength + extraFieldLength + fileCommentLength;
    }
    
    reject(new Error('mnemos binary not found in ZIP archive'));
  });
}

/**
 * Download and verify mnemos binary with retry logic
 * @param {string} version - Version to download (e.g., 'v1.1.10')
 * @param {Object} platformInfo - Platform information from detectPlatform()
 * @param {string} cacheDir - Cache directory path
 * @returns {Promise<void>}
 */
async function downloadBinary(version, platformInfo, cacheDir) {
  const owner = 's60yucca';
  const repo = 'mnemos';
  const versionTag = version.startsWith('v') ? version : `v${version}`;
  
  const archiveUrl = `https://github.com/${owner}/${repo}/releases/download/${versionTag}/${platformInfo.archiveName}`;
  const checksumsUrl = `https://github.com/${owner}/${repo}/releases/download/${versionTag}/checksums.txt`;
  
  const archivePath = path.join(cacheDir, platformInfo.archiveName);
  const checksumsPath = path.join(cacheDir, 'checksums.txt');
  
  // Retry logic: 2 retries with 2-second delay
  const maxAttempts = 3;
  const retryDelay = 2000;
  
  for (let attempt = 0; attempt < maxAttempts; attempt++) {
    try {
      // Download checksums.txt
      process.stderr.write(`[mnemos-cli] Downloading checksums...\n`);
      await downloadFile(checksumsUrl, checksumsPath);
      
      // Parse checksums
      const checksumsContent = fs.readFileSync(checksumsPath, 'utf8');
      const checksums = parseChecksums(checksumsContent);
      const expectedHash = checksums.get(platformInfo.archiveName);
      
      if (!expectedHash) {
        throw new Error(
          `Checksum not found for ${platformInfo.archiveName}\n` +
          `Available checksums: ${Array.from(checksums.keys()).join(', ')}`
        );
      }
      
      // Download archive
      process.stderr.write(`[mnemos-cli] Downloading ${platformInfo.archiveName}...\n`);
      await downloadFile(archiveUrl, archivePath);
      
      // Verify checksum
      process.stderr.write(`[mnemos-cli] Verifying checksum...\n`);
      const actualHash = await computeSHA256(archivePath);
      
      if (actualHash !== expectedHash) {
        fs.unlinkSync(archivePath);
        throw new Error(
          `Checksum mismatch for ${platformInfo.archiveName}\n` +
          `Expected: ${expectedHash}\n` +
          `Actual: ${actualHash}\n` +
          `Action: The download may be corrupted. Please try again.`
        );
      }
      
      // Extract archive
      process.stderr.write(`[mnemos-cli] Extracting binary...\n`);
      if (platformInfo.archiveName.endsWith('.tar.gz')) {
        await extractTarGz(archivePath, cacheDir);
      } else if (platformInfo.archiveName.endsWith('.zip')) {
        await extractZip(archivePath, cacheDir);
      } else {
        throw new Error(`Unsupported archive format: ${platformInfo.archiveName}`);
      }
      
      // Clean up archive and checksums
      fs.unlinkSync(archivePath);
      fs.unlinkSync(checksumsPath);
      
      process.stderr.write(`[mnemos-cli] Download complete\n`);
      return;
      
    } catch (err) {
      // Check if we should retry
      const isNetworkError = err.code === 'ECONNRESET' || 
                            err.code === 'ETIMEDOUT' || 
                            err.code === 'ENOTFOUND' ||
                            err.code === 'ECONNREFUSED';
      
      const isLastAttempt = attempt === maxAttempts - 1;
      
      if (!isNetworkError || isLastAttempt) {
        // Don't retry for non-network errors or if this was the last attempt
        throw new Error(
          `Failed to download mnemos binary\n\n` +
          `Details: ${err.message}\n` +
          `URL: ${archiveUrl}\n\n` +
          `Action: Check your internet connection and verify the release exists\n` +
          `For more information: https://github.com/${owner}/${repo}/releases/tag/${versionTag}`
        );
      }
      
      // Retry after delay
      process.stderr.write(`[mnemos-cli] Download failed, retrying in ${retryDelay/1000}s... (attempt ${attempt + 1}/${maxAttempts})\n`);
      await new Promise(resolve => setTimeout(resolve, retryDelay));
    }
  }
}

module.exports = {
  downloadBinary,
  downloadFile,
  computeSHA256,
  parseChecksums,
  extractTarGz,
  extractZip,
};
