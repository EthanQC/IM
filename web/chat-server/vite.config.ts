import { request as httpRequest } from "node:http";
import { request as httpsRequest } from "node:https";
import type { IncomingMessage, ServerResponse } from "node:http";
import { defineConfig, loadEnv } from "vite";
import vue from "@vitejs/plugin-vue";

function readBody(req: IncomingMessage): Promise<Buffer> {
  return new Promise((resolve, reject) => {
    const chunks: Buffer[] = [];
    req.on("data", (chunk: Buffer) => chunks.push(chunk));
    req.on("end", () => resolve(Buffer.concat(chunks)));
    req.on("error", reject);
  });
}

function proxyUploadRequest(targetRaw: string, body: Buffer, contentType: string): Promise<number> {
  return new Promise((resolve, reject) => {
    const target = new URL(targetRaw);
    const useHttps = target.protocol === "https:";
    const sender = useHttps ? httpsRequest : httpRequest;
    const requestHost = target.hostname === "host.docker.internal" ? "127.0.0.1" : target.hostname;

    const upstream = sender(
      {
        protocol: target.protocol,
        hostname: requestHost,
        port: target.port || (useHttps ? "443" : "80"),
        method: "PUT",
        path: `${target.pathname}${target.search}`,
        headers: {
          // Presigned URL signature includes host; keep original.
          Host: target.host,
          "Content-Type": contentType,
          "Content-Length": String(body.length),
        },
      },
      (upstreamRes) => {
        const code = Number(upstreamRes.statusCode || 500);
        upstreamRes.resume();
        upstreamRes.on("end", () => resolve(code));
      },
    );

    upstream.on("error", reject);
    upstream.write(body);
    upstream.end();
  });
}

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");

  return {
    plugins: [
      vue(),
      {
        name: "im-upload-proxy",
        configureServer(server) {
          server.middlewares.use(async (req: IncomingMessage, res: ServerResponse, next) => {
            if (!req.url || !req.url.startsWith("/__im_upload_proxy__")) {
              next();
              return;
            }

            if (req.method !== "PUT") {
              res.statusCode = 405;
              res.end("method not allowed");
              return;
            }

            const parsed = new URL(req.url, "http://127.0.0.1");
            const target = parsed.searchParams.get("target") || "";
            if (!target.startsWith("http://") && !target.startsWith("https://")) {
              res.statusCode = 400;
              res.end("invalid upload target");
              return;
            }

            try {
              const body = await readBody(req);
              const contentType = String(req.headers["content-type"] || "application/octet-stream");
              const statusCode = await proxyUploadRequest(target, body, contentType);

              if (statusCode >= 200 && statusCode < 300) {
                res.statusCode = 204;
                res.end();
                return;
              }

              res.statusCode = statusCode || 502;
              res.end("upload proxy failed");
            } catch {
              res.statusCode = 502;
              res.end("upload proxy failed");
            }
          });
        },
      },
    ],
    server: {
      host: "0.0.0.0",
      port: 5173,
      proxy: {
        "/api": {
          target: env.VITE_API_PROXY_TARGET || "http://127.0.0.1:8080",
          changeOrigin: true,
        },
        "/ws": {
          target: env.VITE_WS_PROXY_TARGET || "ws://127.0.0.1:8084",
          ws: true,
          changeOrigin: true,
        },
      },
    },
  };
});
