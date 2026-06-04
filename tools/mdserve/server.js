#!/usr/bin/env node
const http = require("http");
const fs = require("fs");
const path = require("path");
const { spawn } = require("child_process");

const DEFAULT_FILE = "report.md";
const DEFAULT_PORT = 8765;
const HOST = "127.0.0.1";

function parseArgs(argv) {
  const opts = { file: DEFAULT_FILE, port: DEFAULT_PORT, open: true };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === "-f" || a === "--file") opts.file = argv[++i];
    else if (a === "-p" || a === "--port") opts.port = parseInt(argv[++i], 10);
    else if (a === "--no-open") opts.open = false;
    else if (a === "-h" || a === "--help") {
      console.log("usage: node tools/mdserve/server.js [-f file.md] [-p port] [--no-open]");
      process.exit(0);
    }
  }
  return opts;
}

const opts = parseArgs(process.argv.slice(2));
const mdPath = path.resolve(opts.file);
const indexPath = path.join(__dirname, "index.html");
const stylesPath = path.join(__dirname, "styles.css");
const markedPath = path.join(__dirname, "vendor", "marked.min.js");

if (!fs.existsSync(mdPath)) {
  console.error(`mdserve: file not found: ${mdPath}`);
  console.error("run the analyzer first, or pass -f <path-to-md>.");
  process.exit(1);
}

function send(res, status, type, body) {
  res.writeHead(status, { "Content-Type": type, "Cache-Control": "no-store" });
  res.end(body);
}

function sendFile(res, file, type, missing) {
  fs.readFile(file, (err, buf) => {
    if (err) return send(res, 404, "text/plain", missing);
    send(res, 200, type, buf);
  });
}

function mtimeMs() {
  try {
    return fs.statSync(mdPath).mtimeMs;
  } catch {
    return 0;
  }
}

const server = http.createServer((req, res) => {
  const url = req.url.split("?")[0];

  if (url === "/" || url === "/index.html") {
    return sendFile(res, indexPath, "text/html; charset=utf-8", "index.html missing");
  }
  if (url === "/styles.css") {
    return sendFile(res, stylesPath, "text/css; charset=utf-8", "styles.css missing");
  }
  if (url === "/vendor/marked.min.js") {
    return sendFile(res, markedPath, "application/javascript; charset=utf-8", "marked.min.js missing");
  }
  if (url === "/report.md") {
    return sendFile(res, mdPath, "text/markdown; charset=utf-8", "report not found");
  }
  if (url === "/mtime") {
    return send(res, 200, "application/json", JSON.stringify({ mtime: mtimeMs() }));
  }
  send(res, 404, "text/plain", "not found");
});

function openBrowser(targetUrl) {
  const platform = process.platform;
  const cmd = platform === "darwin" ? "open" : platform === "win32" ? "cmd" : "xdg-open";
  const args = platform === "win32" ? ["/c", "start", "", targetUrl] : [targetUrl];
  try {
    spawn(cmd, args, { stdio: "ignore", detached: true }).unref();
  } catch {
    /* user can open the URL manually */
  }
}

server.listen(opts.port, HOST, () => {
  const targetUrl = `http://localhost:${opts.port}/`;
  console.log(`mdserve: serving ${path.basename(mdPath)} at ${targetUrl}`);
  console.log("mdserve: live-reload on; Ctrl+C to stop");
  if (opts.open) openBrowser(targetUrl);
});

server.on("error", (err) => {
  if (err.code === "EADDRINUSE") {
    console.error(`mdserve: port ${opts.port} is busy — try -p <other-port>`);
  } else {
    console.error(`mdserve: ${err.message}`);
  }
  process.exit(1);
});
