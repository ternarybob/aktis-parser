#!/usr/bin/env node
/**
 * Index all Go functions in the codebase
 * Creates a registry to prevent duplicate implementations
 */

const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

function indexGoFunctions() {
  const index = {
    lastUpdated: new Date().toISOString(),
    functions: []
  };

  try {
    // Find all Go files first
    const goFiles = execSync(
      'find . -name "*.go" -type f 2>/dev/null || dir /s /b *.go 2>nul',
      { encoding: 'utf-8', shell: 'bash', maxBuffer: 10 * 1024 * 1024 }
    ).split('\n').filter(Boolean);

    let result = '';

    // Read each Go file and extract functions
    goFiles.forEach(file => {
      try {
        const content = fs.readFileSync(file, 'utf-8');
        const lines = content.split('\n');

        lines.forEach((line, idx) => {
          if (/^func\s+(\([^)]+\)\s+)?[A-Z]\w+\s*\(/.test(line)) {
            result += `${file}:${idx + 1}:${line}\n`;
          }
        });
      } catch (err) {
        // Skip files that can't be read
      }
    });

    const lines = result.split('\n').filter(Boolean);

    lines.forEach(line => {
      const match = line.match(/^([^:]+):(\d+):(.+)$/);
      if (match) {
        const [, filePath, lineNum, funcDef] = match;

        // Extract function name
        const funcMatch = funcDef.match(/func\s+(?:\([^)]+\)\s+)?([A-Z]\w+)\s*\(/);
        if (funcMatch) {
          const funcName = funcMatch[1];
          const isReceiver = /func\s+\([^)]+\)/.test(funcDef);

          index.functions.push({
            name: funcName,
            file: filePath.replace(/^\.\//, ''),
            line: parseInt(lineNum),
            isReceiver,
            signature: funcDef.trim()
          });
        }
      }
    });

    // Write index
    const indexPath = path.join(process.cwd(), '.claude', 'go-function-index.json');
    fs.writeFileSync(indexPath, JSON.stringify(index, null, 2));

    console.log(`✅ Indexed ${index.functions.length} Go functions`);
  } catch (error) {
    console.error('⚠️  Function indexing failed:', error.message);
  }
}

indexGoFunctions();
