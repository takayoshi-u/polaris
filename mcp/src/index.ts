#!/usr/bin/env node

/**
 * Polaris Model Context Protocol (MCP) Server Entrypoint
 */

import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import { PolarisClient } from './client.js';
import { createPolarisMcpServer } from './server.js';

export { PolarisClient } from './client.js';
export { createPolarisMcpServer } from './server.js';

async function main() {
  const baseUrl = process.env.POLARIS_BASE_URL || 'http://localhost:8080/api/v1';
  const apiToken = process.env.POLARIS_API_TOKEN;
  const email = process.env.POLARIS_EMAIL;
  const password = process.env.POLARIS_PASSWORD;

  const client = new PolarisClient({
    baseUrl,
    apiToken,
    email,
    password,
  });

  const server = createPolarisMcpServer(client);
  const transport = new StdioServerTransport();

  await server.connect(transport);
  console.error(`Polaris MCP Server running on stdio (Target: ${baseUrl})`);
}

// When executed directly, start the server
main().catch((err) => {
  console.error('Fatal error starting Polaris MCP Server:', err);
  process.exit(1);
});
