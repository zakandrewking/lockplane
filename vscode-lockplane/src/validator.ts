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

    // Run lockplane plan with --validate to get validation results
    // We'll use --from (empty schema) and --to (schema path) to validate
    const cmd = `${lockplanePath} plan --from '{"tables":[]}' --to "${schemaPath}" --validate 2>&1`;

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

        // Parse validation messages
        // Look for lines like: "✗ Validation X: FAIL"
        // and "Error: ..." or "Warning: ..."
        const lines = output.split('\n');
        let currentFile = schemaPath;
        let lineNum = 1;

        for (let i = 0; i < lines.length; i++) {
          const line = lines[i];

          // Check for validation failures
          if (line.includes('✗ Validation') && line.includes('FAIL')) {
            // Look ahead for error/warning messages
            for (let j = i + 1; j < Math.min(i + 10, lines.length); j++) {
              const nextLine = lines[j].trim();

              if (nextLine.startsWith('Error:')) {
                results.push({
                  file: currentFile,
                  line: lineNum,
                  column: 1,
                  severity: 'error',
                  message: nextLine.replace('Error:', '').trim()
                });
              } else if (nextLine.startsWith('Warning:')) {
                results.push({
                  file: currentFile,
                  line: lineNum,
                  column: 1,
                  severity: 'warning',
                  message: nextLine.replace('Warning:', '').trim()
                });
              } else if (nextLine.startsWith('-')) {
                // Reason line, add to last result if exists
                if (results.length > 0) {
                  results[results.length - 1].message += '\n' + nextLine;
                }
              } else if (nextLine.includes('Validation') || nextLine === '') {
                // Stop at next validation or empty line
                break;
              }
            }
          }

          // Check for fatal errors (schema loading failures, etc.)
          if (line.includes('Failed to load') || line.includes('Error:')) {
            results.push({
              file: currentFile,
              line: 1,
              column: 1,
              severity: 'error',
              message: line.trim()
            });
          }
        }

        // Log what we parsed
        console.log(`[Lockplane] Parsed ${results.length} validation results:`, results);

        // If we got results, return them
        if (results.length > 0) {
          resolve(results);
          return;
        }

        // If no validation errors and command succeeded, schema is valid
        if (!error || error.code === 0) {
          resolve([]);
          return;
        }

        // Check if lockplane CLI is not found
        if (error && (error.message.includes('not found') || error.message.includes('ENOENT'))) {
          reject(new Error('Lockplane CLI not found. Make sure it is installed and in your PATH.'));
          return;
        }

        // Other errors - could be validation failures, return empty for now
        // (validation messages already captured above)
        resolve(results);
      }
    );
  });
}
