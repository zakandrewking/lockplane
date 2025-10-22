import * as vscode from 'vscode';
import * as cp from 'child_process';
import * as path from 'path';

export interface ValidationResult {
  file: string;
  line: number;
  column: number;
  severity: 'error' | 'warning' | 'info';
  message: string;
  code?: string;
}

/**
 * Validate a schema file or directory using the lockplane CLI
 */
export async function validateSchema(
  schemaPath: string
): Promise<ValidationResult[]> {
  return new Promise((resolve, reject) => {
    const config = vscode.workspace.getConfiguration('lockplane');
    const lockplanePath = config.get<string>('cliPath', 'lockplane');

    // For now, we'll use a simplified approach since we don't have
    // lockplane validate --format json yet. We'll parse the output
    // from the plan command which includes validation.

    // Get workspace folder for cwd
    const workspaceFolders = vscode.workspace.workspaceFolders;
    const cwd = workspaceFolders ? workspaceFolders[0].uri.fsPath : undefined;

    // Validate by trying to convert the schema to JSON
    // This will catch SQL syntax errors and basic validation issues
    // If the conversion succeeds, the schema is valid
    const cmd = `${lockplanePath} convert --input "${schemaPath}" 2>&1`;

    console.log(`[Lockplane] Running command: ${cmd}`);
    console.log(`[Lockplane] Working directory: ${cwd}`);

    cp.exec(
      cmd,
      { cwd, maxBuffer: 10 * 1024 * 1024 },
      (error, stdout, stderr) => {
        console.log(`[Lockplane] stdout:`, stdout);
        console.log(`[Lockplane] stderr:`, stderr);
        console.log(`[Lockplane] error:`, error);

        // Parse validation output
        const results: ValidationResult[] = [];

        // The command might fail if there are validation errors,
        // but we still want to parse the output
        const output = stdout + stderr;
        console.log(`[Lockplane] Combined output:`, output);

        // Parse validation messages from convert command
        // If there's an error, it will be in the output
        // If successful, output will be JSON (which we can ignore for validation)

        // Check if command succeeded (no error and JSON output)
        if (!error && output.trim().startsWith('{')) {
          // Schema is valid (successfully converted to JSON)
          console.log('[Lockplane] Schema is valid');
          resolve([]);
          return;
        }

        // Parse error messages
        const lines = output.split('\n');
        let currentFile = schemaPath;

        for (const line of lines) {
          const trimmed = line.trim();

          // Skip empty lines and JSON output
          if (!trimmed || trimmed.startsWith('{') || trimmed.startsWith('}')) {
            continue;
          }

          // Check for error messages
          if (trimmed.includes('Failed to') ||
              trimmed.includes('Error:') ||
              trimmed.includes('error:') ||
              trimmed.includes('parse error')) {

            // Try to extract line number from error messages like "line 5:"
            const lineMatch = trimmed.match(/line (\d+)/i);
            const lineNum = lineMatch ? parseInt(lineMatch[1]) : 1;

            results.push({
              file: currentFile,
              line: lineNum,
              column: 1,
              severity: 'error',
              message: trimmed.replace(/^\d{4}\/\d{2}\/\d{2} \d{2}:\d{2}:\d{2}\s+/, '') // Remove timestamp
            });
          }
        }

        // Check if lockplane CLI is not found (before resolving)
        if (error && (error.message.includes('not found') || error.message.includes('ENOENT'))) {
          reject(new Error('Lockplane CLI not found. Make sure it is installed and in your PATH.'));
          return;
        }

        // Log what we parsed
        console.log(`[Lockplane] Parsed ${results.length} validation results:`, results);

        // Return the results (empty array if no errors)
        resolve(results);
      }
    );
  });
}
