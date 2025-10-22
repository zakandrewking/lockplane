import * as vscode from 'vscode';
import { ValidationResult } from './validator';

const diagnosticCollection = vscode.languages.createDiagnosticCollection('lockplane');

/**
 * Update diagnostics for a document based on validation results
 */
export function updateDiagnostics(
  document: vscode.TextDocument,
  results: ValidationResult[]
): void {
  const diagnostics: vscode.Diagnostic[] = [];

  for (const result of results) {
    // Only show diagnostics for the current file
    // (cross-file validation results will appear when editing those files)
    const resultPath = normalizeFilePath(result.file);
    const docPath = normalizeFilePath(document.fileName);

    if (!resultPath.endsWith(docPath) && !docPath.endsWith(resultPath)) {
      continue;
    }

    // Create range for the diagnostic
    // Line numbers from lockplane are 1-based, VSCode uses 0-based
    const line = Math.max(0, result.line - 1);
    const column = Math.max(0, result.column - 1);

    // Create a range that spans the likely error location
    // If we have column info, highlight ~20 characters from that position
    // Otherwise, highlight the whole line
    let range: vscode.Range;
    if (result.column > 0) {
      const endColumn = Math.min(column + 20, document.lineAt(line).text.length);
      range = new vscode.Range(line, column, line, endColumn);
    } else {
      range = document.lineAt(line).range;
    }

    // Map severity
    const severity = mapSeverity(result.severity);

    // Create diagnostic
    const diagnostic = new vscode.Diagnostic(
      range,
      result.message,
      severity
    );

    diagnostic.source = 'lockplane';
    if (result.code) {
      diagnostic.code = result.code;
    }

    diagnostics.push(diagnostic);
  }

  diagnosticCollection.set(document.uri, diagnostics);
}

/**
 * Clear diagnostics for a document
 */
export function clearDiagnostics(document: vscode.TextDocument): void {
  diagnosticCollection.delete(document.uri);
}

/**
 * Clear all diagnostics
 */
export function clearAllDiagnostics(): void {
  diagnosticCollection.clear();
}

function mapSeverity(severity: string): vscode.DiagnosticSeverity {
  switch (severity) {
    case 'error':
      return vscode.DiagnosticSeverity.Error;
    case 'warning':
      return vscode.DiagnosticSeverity.Warning;
    case 'info':
      return vscode.DiagnosticSeverity.Information;
    default:
      return vscode.DiagnosticSeverity.Error;
  }
}

function normalizeFilePath(filePath: string): string {
  // Normalize path separators and remove trailing slashes
  return filePath.replace(/\\/g, '/').replace(/\/$/, '').toLowerCase();
}
