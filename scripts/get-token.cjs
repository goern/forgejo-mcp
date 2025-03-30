#!/usr/bin/env node

/**
 * This script helps users obtain a Codeberg API token by opening the Codeberg
 * applications settings page in their default browser.
 */

const { exec } = require("child_process");
const os = require("os");

console.log("This script will help you obtain a Codeberg API token.");
console.log("");
console.log("Instructions:");
console.log(
  "1. A browser window will open to the Codeberg applications settings page",
);
console.log(
  "2. Log in to your Codeberg account if you are not already logged in",
);
console.log('3. Scroll down to the "Generate New Token" section');
console.log('4. Enter a token description (e.g., "MCP Codeberg Server")');
console.log(
  '5. Select the appropriate scopes (at minimum: "repo" for repository access)',
);
console.log('6. Click "Generate Token"');
console.log("7. Copy the generated token (it will only be shown once!)");
console.log(
  "8. Add the token to your MCP settings file as described in the README.md",
);
console.log("");

// Determine the command to open a URL based on the operating system
const openCommand = (() => {
  switch (os.platform()) {
    case "darwin":
      return "open";
    case "win32":
      return "start";
    default:
      return "xdg-open";
  }
})();

// URL to Codeberg applications settings page
const url = "https://codeberg.org/user/settings/applications";

console.log(`Opening ${url} in your default browser...`);

// Open the URL in the default browser
exec(`${openCommand} ${url}`, (error) => {
  if (error) {
    console.error(`Error opening browser: ${error.message}`);
    return;
  }

  console.log("");
  console.log("Browser opened successfully.");
  console.log("");
  console.log(
    "After generating your token, update your MCP settings file with:",
  );
  console.log("");
  console.log("{");
  console.log('  "mcpServers": {');
  console.log('    "codeberg": {');
  console.log('      "command": "node",');
  console.log('      "args": ["/path/to/mcp-codeberg/build/index.js"],');
  console.log('      "env": {');
  console.log('        "CODEBERG_API_TOKEN": "your-token-here"');
  console.log("      }");
  console.log("    }");
  console.log("  }");
  console.log("}");
});
