import * as vscode from 'vscode';
import * as path from 'path';
import { validateSchema } from './validator';
import { updateDiagnostics, clearDiagnostics } from './diagnostics';

let validationTimeout: NodeJS.Timeout | undefined;
let outputChannel: vscode.OutputChannel;
let statusBarItem: vscode.StatusBarItem;

export function activate(context: vscode.ExtensionContext) {
  // Create output channel for logging
  outputChannel = vscode.window.createOutputChannel('Lockplane');
  context.subscriptions.push(outputChannel);

  // Create status bar item
  statusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Right, 100);
  statusBarItem.text = '$(check) Lockplane';
  statusBarItem.tooltip = 'Lockplane extension is active';
  context.subscriptions.push(statusBarItem);

  outputChannel.appendLine('Lockplane extension activated');
  console.log('Lockplane extension is now active');

  // Validate on document save
  context.subscriptions.push(
    vscode.workspace.onDidSaveTextDocument(async (document) => {
      if (!isLockplaneFile(document)) {
        return;
      }

      const config = vscode.workspace.getConfiguration('lockplane');
      if (!config.get<boolean>('validateOnSave', true)) {
        return;
      }

      await runValidation(document);
    })
  );

  // Validate on document change (debounced)
  context.subscriptions.push(
    vscode.workspace.onDidChangeTextDocument((event) => {
      const document = event.document;
      if (!isLockplaneFile(document)) {
        return;
      }

      const config = vscode.workspace.getConfiguration('lockplane');
      if (!config.get<boolean>('validateOnType', true)) {
        return;
      }

      if (validationTimeout) {
        clearTimeout(validationTimeout);
      }

      const delay = config.get<number>('validationDelay', 500);
      validationTimeout = setTimeout(async () => {
        await runValidation(document);
      }, delay);
    })
  );

  // Validate on document open
  context.subscriptions.push(
    vscode.workspace.onDidOpenTextDocument(async (document) => {
      if (!isLockplaneFile(document)) {
        return;
      }

      await runValidation(document);
    })
  );

  // Clear diagnostics on document close
  context.subscriptions.push(
    vscode.workspace.onDidCloseTextDocument((document) => {
      if (isLockplaneFile(document)) {
        clearDiagnostics(document);
      }
    })
  );

  // Validate all open .lp.sql files on activation
  vscode.workspace.textDocuments.forEach(async (document) => {
    if (isLockplaneFile(document)) {
      await runValidation(document);
    }
  });
}

function isLockplaneFile(document: vscode.TextDocument): boolean {
  const isLpSql = document.fileName.endsWith('.lp.sql');
  if (isLpSql) {
    outputChannel.appendLine(`Detected .lp.sql file: ${document.fileName}`);
    statusBarItem.show();
  }
  return isLpSql;
}

async function runValidation(document: vscode.TextDocument): Promise<void> {
  const config = vscode.workspace.getConfiguration('lockplane');
  if (!config.get<boolean>('enabled', true)) {
    outputChannel.appendLine('Validation skipped: extension is disabled');
    return;
  }

  const schemaPath = getSchemaPath(document);
  if (!schemaPath) {
    outputChannel.appendLine('Validation skipped: could not determine schema path');
    return;
  }

  outputChannel.appendLine(`Validating schema at: ${schemaPath}`);
  statusBarItem.text = '$(sync~spin) Lockplane';
  statusBarItem.tooltip = 'Validating schema...';

  try {
    const results = await validateSchema(schemaPath);
    outputChannel.appendLine(`Validation complete: ${results.length} issues found`);

    updateDiagnostics(document, results);

    // Update status bar
    if (results.length === 0) {
      statusBarItem.text = '$(check) Lockplane';
      statusBarItem.tooltip = 'Schema is valid';
    } else {
      const errors = results.filter(r => r.severity === 'error').length;
      const warnings = results.filter(r => r.severity === 'warning').length;
      statusBarItem.text = `$(warning) Lockplane: ${errors} errors, ${warnings} warnings`;
      statusBarItem.tooltip = `Lockplane validation: ${errors} errors, ${warnings} warnings`;
    }
  } catch (error) {
    statusBarItem.text = '$(error) Lockplane';
    statusBarItem.tooltip = 'Validation failed';

    if (error instanceof Error) {
      outputChannel.appendLine(`Validation error: ${error.message}`);

      // Only show error if lockplane CLI is not found or critical error
      if (error.message.includes('not found') || error.message.includes('ENOENT')) {
        vscode.window.showErrorMessage(
          `Lockplane CLI not found. Install from https://github.com/zakandrewking/lockplane`
        );
      } else {
        outputChannel.appendLine(`Full error: ${error.stack || error}`);
        console.error('Lockplane validation error:', error);
      }
    }
  }
}

function getSchemaPath(document: vscode.TextDocument): string | undefined {
  const workspaceFolder = vscode.workspace.getWorkspaceFolder(document.uri);
  if (!workspaceFolder) {
    // No workspace folder, validate single file
    return document.fileName;
  }

  const config = vscode.workspace.getConfiguration('lockplane');
  const configPath = config.get<string>('schemaPath', 'lockplane/schema/');

  // Try to find lockplane.toml to determine schema path
  const lockplaneToml = path.join(workspaceFolder.uri.fsPath, 'lockplane.toml');
  // TODO: Parse lockplane.toml for schema_path if it exists

  // For now, use configured path or directory containing the file
  const fullPath = path.join(workspaceFolder.uri.fsPath, configPath);

  // Check if the file is within the schema path
  if (document.fileName.startsWith(fullPath)) {
    return fullPath;
  }

  // Otherwise validate the directory containing the file
  return path.dirname(document.fileName);
}

export function deactivate() {
  if (validationTimeout) {
    clearTimeout(validationTimeout);
  }
}
