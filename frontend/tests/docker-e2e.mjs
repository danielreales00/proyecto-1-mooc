// Mantiene los hosts y firmas de desarrollo: el navegador de este contenedor
// accede a localhost igual que el navegador de la máquina anfitriona.
import net from "node:net";
import { spawn } from "node:child_process";
const proxies = [];
const sockets = new Set();
for (const [port, host, target] of [
  [8090, "proxy", 8080],
  [9010, "minio", 9000],
  [8026, "mailpit", 8025],
]) {
  const server = net.createServer((socket) => {
    const upstream = net.connect(target, host);
    for (const connection of [socket, upstream]) {
      sockets.add(connection);
      connection.once("close", () => sockets.delete(connection));
    }
    socket.pipe(upstream);
    upstream.pipe(socket);
    socket.on("error", () => upstream.destroy());
    upstream.on("error", () => socket.destroy());
  });
  await new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(port, "0.0.0.0", resolve);
  });
  proxies.push(server);
}
try {
  await new Promise((resolve, reject) => {
    const child = spawn("npm", ["ci"], { stdio: "inherit" });
    child.on("error", reject);
    child.on("exit", (code) =>
      code === 0 ? resolve() : reject(new Error(`npm ci: ${code}`)),
    );
  });
  await import("./e2e.mjs");
} finally {
  for (const server of proxies) server.close();
  for (const connection of sockets) connection.destroy();
}
