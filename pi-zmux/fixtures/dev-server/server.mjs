import http from 'node:http';

const server = http.createServer((request, response) => {
  response.writeHead(200, { 'content-type': 'text/plain' });
  response.end('pi-zmux fixture ok\n');
});

server.listen(0, '127.0.0.1', () => {
  const address = server.address();
  const port = typeof address === 'object' && address ? address.port : 0;
  console.log(`ready localhost:${port}`);
});

const shutdown = () => {
  console.log('stopping fixture dev server');
  server.close(() => process.exit(0));
};

process.on('SIGINT', shutdown);
process.on('SIGTERM', shutdown);
