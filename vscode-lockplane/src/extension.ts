import * as vscode from 'vscode';
import * as path from 'path';
import { validateSchema } from './validator';
import { updateDiagnostics, clearDiagnostics } from './diagnostics';

let validationTimeout: NodeJS.Timeout | undefined;

export function activate(context: vscode.ExtensionContext) {
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
  return document.fileName.endsWith('.lp.sql');
}

async function runValidation(document: vscode.TextDocument): Promise<void> {
  const config = vscode.workspace.getConfiguration('lockplane');
  if (!config.get<boolean>('enabled', true)) {
    return;
  }

  const schemaPath = getSchemaPath(document);
  if (!schemaPath) {
    return;
  }

  try {
    const results = await validateSchema(schemaPath);
    updateDiagnostics(document, results);
  } catch (error) {
    if (error instanceof Error) {
      // Only show error if lockplane CLI is not found or critical error
      if (error.message.includes('not found') || error.message.includes('ENOENT')) {
        vscode.window.showErrorMessage(
          `Lockplane CLI not found. Install from https://github.com/zakandrewking/lockplane`
        );
      } else {
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
