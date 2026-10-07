const { McpServer } = require('@modelcontextprotocol/sdk/server/mcp.js');
const { StdioServerTransport } = require('@modelcontextprotocol/sdk/server/stdio.js');
const { registerLessonTools } = require('./tools');

const server = new McpServer({ name: 'hebrew-lessons', version: '1.0.0' });
registerLessonTools(server);

server.connect(new StdioServerTransport()).catch(err => {
    console.error(err);
    process.exit(1);
});
