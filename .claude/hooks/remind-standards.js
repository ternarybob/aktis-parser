#!/usr/bin/env node
/**
 * UserPromptSubmit Hook: Inject critical reminders
 * Ensures Claude follows project standards
 */

// Read JSON input from stdin
let inputData = '';
process.stdin.on('data', chunk => inputData += chunk);
process.stdin.on('end', () => {
  try {
    const data = JSON.parse(inputData);
    const userMessage = data.user_message || '';

    // Check if user is asking about tests or building
    const isTestRelated = /test|tests|testing/i.test(userMessage);
    const isBuildRelated = /build|compile|rebuild/i.test(userMessage);

    if (isTestRelated || isBuildRelated) {
      console.log('═══════════════════════════════════════════════════════════');
      console.log('🚨 CRITICAL REMINDERS:');

      if (isTestRelated) {
        console.log('');
        console.log('  TESTS: MUST use ./tests/run-tests.ps1');
        console.log('         NEVER use: go test, cd tests && go test, etc.');
        console.log('         Location: tests/run-tests.ps1 (NOT root)');
        console.log('         Usage: ./tests/run-tests.ps1 -Type [all|api|ui]');
      }

      if (isBuildRelated) {
        console.log('');
        console.log('  BUILD: MUST use ./scripts/build.ps1');
        console.log('         NEVER use: go build directly');
        console.log('         Location: scripts/build.ps1');
        console.log('         Usage: ./scripts/build.ps1 [-Clean] [-Test] [-Release]');
      }

      console.log('');
      console.log('═══════════════════════════════════════════════════════════');
    }

    // Always allow the prompt to proceed
    process.exit(0);
  } catch (error) {
    // If parsing fails, just allow the prompt
    process.exit(0);
  }
});
