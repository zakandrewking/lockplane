import * as cp from "child_process";
import * as path from "path";
import * as vscode from "vscode";

import { checkSchema } from "./checker";
import { clearDiagnostics, updateDiagnostics } from "./diagnostics";

let checkTimeout: NodeJS.Timeout | undefined;
let outputChannel: vscode.OutputChannel;
let statusBarItem: vscode.StatusBarItem;

export function activate(context: vscode.ExtensionContext) {
  // Create output channel for logging
  outputChannel = vscode.window.createOutputChannel("Lockplane");
  context.subscriptions.push(outputChannel);

  // Create status bar item
  statusBarItem = vscode.window.createStatusBarItem(
    vscode.StatusBarAlignment.Right,
    100
  );
  statusBarItem.text = "$(check) Lockplane";
  statusBarItem.tooltip = "Lockplane extension is active";
  context.subscriptions.push(statusBarItem);

  outputChannel.appendLine("Lockplane extension activated");
  console.log("Lockplane extension is now active");

  // Display lockplane CLI info
  displayLockplaneInfo();

  // Validate on document save
  context.subscriptions.push(
    vscode.workspace.onDidSaveTextDocument(async (document) => {
      if (!isLockplaneFile(document)) {
        return;
      }

      const config = vscode.workspace.getConfiguration("lockplane");
      if (!config.get<boolean>("checkSchemaOnSave", true)) {
        return;
      }

      await runCheckSchema(document);
    })
  );

  // Validate on document change (debounced)
  context.subscriptions.push(
    vscode.workspace.onDidChangeTextDocument((event) => {
      const document = event.document;
      if (!isLockplaneFile(document)) {
        return;
      }

      const config = vscode.workspace.getConfiguration("lockplane");
      if (!config.get<boolean>("checkSchemaOnType", true)) {
        return;
      }

      if (checkTimeout) {
        clearTimeout(checkTimeout);
      }

      const delay = config.get<number>("checkSchemaDelay", 500);
      checkTimeout = setTimeout(async () => {
        await runCheckSchema(document);
      }, delay);
    })
  );

  // Validate on document open
  context.subscriptions.push(
    vscode.workspace.onDidOpenTextDocument(async (document) => {
      if (!isLockplaneFile(document)) {
        return;
      }

      await runCheckSchema(document);
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
      await runCheckSchema(document);
    }
  });
}

function isLockplaneFile(document: vscode.TextDocument): boolean {
  const isLpSql = document.fileName.endsWith(".lp.sql");
  if (isLpSql) {
    outputChannel.appendLine(`Detected .lp.sql file: ${document.fileName}`);
    statusBarItem.show();
  }
  return isLpSql;
}

async function runCheckSchema(document: vscode.TextDocument): Promise<void> {
  const config = vscode.workspace.getConfiguration("lockplane");
  if (!config.get<boolean>("enabled", true)) {
    outputChannel.appendLine("Schema check skipped: extension is disabled");
    return;
  }

  const schemaPath = getSchemaPath(document);
  if (!schemaPath) {
    outputChannel.appendLine(
      "Schema check skipped: could not determine schema path"
    );
    return;
  }

  outputChannel.appendLine(`Checking schema at: ${schemaPath}`);
  statusBarItem.text = "$(sync~spin) Lockplane";
  statusBarItem.tooltip = "Checking schema...";

  try {
    const results = await checkSchema(schemaPath);
    outputChannel.appendLine(
      `Schema check complete: ${results.length} issues found`
    );

    updateDiagnostics(document, results);

    // Update status bar
    if (results.length === 0) {
      statusBarItem.text = "$(check) Lockplane";
      statusBarItem.tooltip = "Schema is valid";
    } else {
      const errors = results.filter((r) => r.severity === "error").length;
      const warnings = results.filter((r) => r.severity === "warning").length;
      statusBarItem.text = `$(warning) Lockplane: ${errors} errors, ${warnings} warnings`;
      statusBarItem.tooltip = `Lockplane check: ${errors} errors, ${warnings} warnings`;
    }
  } catch (error) {
    statusBarItem.text = "$(error) Lockplane";
    statusBarItem.tooltip = "Schema check failed";

    if (error instanceof Error) {
      outputChannel.appendLine(`Schema check error: ${error.message}`);

      // Only show error if lockplane CLI is not found or critical error
      if (
        error.message.includes("not found") ||
        error.message.includes("ENOENT")
      ) {
        vscode.window.showErrorMessage(
          `Lockplane CLI not found. Install from https://github.com/lockplane/lockplane`
        );
      } else {
        outputChannel.appendLine(`Full error: ${error.stack || error}`);
        console.error("Lockplane schema check error:", error);
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

  const configPath = "/schema";
  // TODO: Parse lockplane.toml for schema_path

  // For now, use configured path or directory containing the file
  const fullPath = path.join(workspaceFolder.uri.fsPath, configPath);

  // Check if the file is within the schema path
  if (document.fileName.startsWith(fullPath)) {
    return fullPath;
  }

  // Otherwise check the directory containing the file
  return path.dirname(document.fileName);
}

function displayLockplaneInfo(): void {
  const config = vscode.workspace.getConfiguration("lockplane");
  const lockplanePath = config.get<string>("cliPath", "lockplane");

  outputChannel.appendLine("---");
  outputChannel.appendLine(`Lockplane CLI path: ${lockplanePath}`);

  // Try to get the actual resolved path
  cp.exec("which " + lockplanePath, (error, stdout, stderr) => {
    if (!error && stdout) {
      const resolvedPath = stdout.trim();
      if (resolvedPath !== lockplanePath) {
        outputChannel.appendLine(`Resolved to: ${resolvedPath}`);
      }
    }

    // Get version
    cp.exec(lockplanePath + " version", (error, stdout, stderr) => {
      if (error) {
        outputChannel.appendLine(
          `Warning: Could not get lockplane version: ${error.message}`
        );
        outputChannel.appendLine(
          "Make sure lockplane is installed and in your PATH"
        );
        outputChannel.appendLine(
          "Install from: https://github.com/lockplane/lockplane"
        );
      } else {
        const version = stdout.trim();
        outputChannel.appendLine(`Version: ${version}`);
      }
      outputChannel.appendLine("---");
    });
  });
}

export function deactivate() {
  if (checkTimeout) {
    clearTimeout(checkTimeout);
  }
}
