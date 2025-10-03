#!/usr/bin/env node
/**
 * Go Structure Enforcement Hook
 * Validates Go clean architecture patterns and prevents duplicate functions
 */

const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

// Read JSON input from stdin
let inputData = '';
process.stdin.on('data', chunk => inputData += chunk);
process.stdin.on('end', () => {
  try {
    const data = JSON.parse(inputData);
    const toolName = data.tool_name || '';
    const toolInput = data.tool_input || {};

    if (toolName === 'Write') {
      validateWrite(toolInput.file_path || '', toolInput.content || '');
    } else if (toolName === 'Edit') {
      validateEdit(toolInput.file_path || '', toolInput.new_string || '');
    } else {
      process.exit(0);
    }
  } catch (error) {
    console.error('❌ Hook error:', error.message);
    process.exit(1);
  }
});

function validateWrite(filePath, content) {
  if (!filePath.endsWith('.go')) {
    process.exit(0); // Not a Go file
  }

  const issues = [];

  // Check directory structure rules
  if (filePath.includes('internal/common/')) {
    checkCommonDirectory(content, issues);
  } else if (filePath.includes('internal/services/')) {
    checkServicesDirectory(content, issues);
  } else if (filePath.includes('internal/handlers/')) {
    checkHandlersDirectory(content, issues);
  }

  // Check for duplicate functions
  checkDuplicateFunctions(filePath, content, issues);

  // Check logging standards
  checkLoggingStandards(content, issues);

  // Check error handling
  checkErrorHandling(content, issues);

  // Report issues
  if (issues.length > 0) {
    console.error('');
    console.error('❌ Go Structure Validation FAILED:');
    console.error('');
    issues.forEach(issue => {
      console.error(`   ${issue}`);
    });
    console.error('');
    process.exit(1);
  }

  console.log('✅ Go structure validation passed');
  process.exit(0);
}

function validateEdit(filePath, newString) {
  if (!filePath.endsWith('.go')) {
    process.exit(0);
  }

  // For edits, check if new code introduces issues
  const issues = [];

  if (filePath.includes('internal/common/')) {
    checkCommonDirectory(newString, issues);
  } else if (filePath.includes('internal/services/')) {
    checkServicesDirectory(newString, issues);
  }

  checkDuplicateFunctions(filePath, newString, issues);
  checkLoggingStandards(newString, issues);
  checkErrorHandling(newString, issues);

  if (issues.length > 0) {
    console.error('');
    console.error('❌ Go Structure Validation FAILED:');
    console.error('');
    issues.forEach(issue => {
      console.error(`   ${issue}`);
    });
    console.error('');
    process.exit(1);
  }

  console.log('✅ Go structure validation passed');
  process.exit(0);
}

function checkCommonDirectory(content, issues) {
  // internal/common/ MUST NOT have receiver methods
  const receiverPattern = /func\s+\([a-z]+\s+\*?[A-Z]\w+\)\s+\w+\s*\(/g;
  if (receiverPattern.test(content)) {
    issues.push('❌ BLOCKED: Receiver methods NOT allowed in internal/common/');
    issues.push('   internal/common/ is for stateless utility functions only');
    issues.push('   Move receiver methods to internal/services/');
  }

  // internal/common/ should not have struct definitions with state
  const structPattern = /type\s+(\w+)\s+struct\s*{[^}]*\w+\s+\w+/;
  if (structPattern.test(content)) {
    issues.push('⚠️  WARNING: Stateful struct in internal/common/');
    issues.push('   Consider if this belongs in internal/services/ instead');
  }
}

function checkServicesDirectory(content, issues) {
  // internal/services/ SHOULD have receiver methods
  const hasReceiverMethod = /func\s+\([a-z]+\s+\*?[A-Z]\w+Service\)\s+\w+\s*\(/g.test(content);
  const hasStandaloneFunc = /^func\s+[A-Z]\w+\s*\(/m.test(content);

  if (!hasReceiverMethod && hasStandaloneFunc) {
    issues.push('⚠️  WARNING: Standalone function in internal/services/');
    issues.push('   Services should use receiver methods on service structs');
  }
}

function checkHandlersDirectory(content, issues) {
  // Handlers should use dependency injection
  const hasHandler = /type\s+\w+Handler\s+struct/.test(content);
  const hasNew = /func\s+New\w+Handler\s*\(/.test(content);

  if (hasHandler && !hasNew) {
    issues.push('⚠️  WARNING: Handler without New constructor');
    issues.push('   Use dependency injection pattern: func NewXHandler(deps) *XHandler');
  }
}

function checkDuplicateFunctions(filePath, content, issues) {
  // Extract function names from new content
  const funcPattern = /func\s+(?:\([^)]+\)\s+)?([A-Z]\w+)\s*\(/g;
  const newFunctions = [];
  let match;

  while ((match = funcPattern.exec(content)) !== null) {
    newFunctions.push(match[1]);
  }

  if (newFunctions.length === 0) {
    return; // No functions to check
  }

  // Search for duplicates in codebase
  try {
    for (const funcName of newFunctions) {
      const searchPattern = `func\\s+(\\([^)]+\\)\\s+)?${funcName}\\s*\\(`;
      const result = execSync(
        `grep -rn --include="*.go" -E "${searchPattern}" . 2>/dev/null || true`,
        { encoding: 'utf-8', cwd: process.cwd(), maxBuffer: 10 * 1024 * 1024 }
      );

      const matches = result.split('\n').filter(Boolean);

      // Filter out the current file being edited
      const normalizedPath = filePath.replace(/\\/g, '/');
      const duplicates = matches.filter(line => {
        const matchPath = line.split(':')[0].replace(/^\.\//, '');
        return matchPath !== normalizedPath;
      });

      if (duplicates.length > 0) {
        issues.push(`❌ BLOCKED: Duplicate function '${funcName}' found:`);
        duplicates.forEach(dup => {
          const [file, line] = dup.split(':');
          issues.push(`   Exists in: ${file}:${line}`);
        });
        issues.push('   Use existing function or rename to avoid duplication');
      }
    }
  } catch (error) {
    // grep not available or error - skip duplicate check
  }
}

function checkLoggingStandards(content, issues) {
  // Check for fmt.Println (should use logger)
  if (/fmt\.Println\s*\(/.test(content)) {
    issues.push('❌ BLOCKED: Using fmt.Println for logging');
    issues.push('   Use arbor logger instead: logger.Info(...), logger.Error(...)');
  }

  if (/log\.Println\s*\(/.test(content)) {
    issues.push('❌ BLOCKED: Using log.Println for logging');
    issues.push('   Use arbor logger instead: logger.Info(...), logger.Error(...)');
  }
}

function checkErrorHandling(content, issues) {
  // Check for ignored errors (_ = )
  if (/\s+_\s*=\s*\w+\(/.test(content)) {
    issues.push('⚠️  WARNING: Ignored error detected (_ = ...)');
    issues.push('   Always handle errors properly');
  }
}
